// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routing

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strings"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/authz"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/util"
)

// Upload implements POST /upload
// This endpoint involves uploading potentially significant amounts of data.
// This implementation supports a configurable maximum file size limit in bytes. If a user tries to upload more than this, they will receive an error that their upload is too large.
// Uploaded files are processed piece-wise to avoid DoS attacks which would starve the server of memory.
// TODO: We should time out requests if they have not received any data within a configured timeout period.
func Upload(req *http.Request, service *frame.Service, db storage.Database, provider storage.Provider, mediaService business.MediaService, authzMiddleware authz.Middleware) util.JSONResponse {
	ctx := req.Context()
	authClaims := security.ClaimsFromContext(ctx)

	cfg := service.Config().(*config.FilesConfig)

	if authClaims == nil {
		return util.JSONResponse{
			Code: http.StatusUnauthorized,
			JSON: map[string]interface{}{
				"errcode": "M_UNKNOWN",
				"error":   "Unauthorised",
			},
		}
	}

	sub, err := authClaims.GetSubject()
	if err != nil {
		return util.JSONResponse{
			Code: http.StatusUnauthorized,
			JSON: map[string]interface{}{
				"errcode": "M_UNKNOWN",
				"error":   "Unauthorised",
			},
		}
	}

	if err = authzMiddleware.CanUploadFile(ctx, sub); err != nil {
		return util.JSONResponse{
			Code: http.StatusForbidden,
			JSON: map[string]interface{}{
				"errcode": "M_FORBIDDEN",
				"error":   "Forbidden",
			},
		}
	}

	ownerID := types.OwnerID(sub)

	// Parse upload request
	uploadReq, resErr := parseAndValidateRequest(req, cfg, ownerID)
	if resErr != nil {
		return *resErr
	}
	defer func() { _ = uploadReq.Close() }()

	// Create business request
	businessReq := &business.UploadRequest{
		OwnerID:       ownerID,
		UploadName:    uploadReq.MediaMetadata.UploadName,
		ContentType:   uploadReq.MediaMetadata.ContentType,
		FileSizeBytes: uploadReq.MediaMetadata.FileSizeBytes,
		FileData:      uploadReq.FileData,
		Config:        cfg,
		IsPublic:      false,
	}

	// Execute business logic
	result, err := mediaService.UploadFile(ctx, businessReq)
	if err != nil {
		return util.JSONResponse{
			Code: http.StatusBadRequest,
			JSON: map[string]interface{}{
				"errcode": "M_UNKNOWN",
				"error":   err.Error(),
			},
		}
	}

	// Queue thumbnail generation
	err = queueThumbnailGeneration(ctx, service, result.MediaID)
	if err != nil {
		return util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: map[string]interface{}{
				"errcode": "M_UNKNOWN",
				"error":   "Failed to generate thumbnails",
			},
		}
	}

	return util.JSONResponse{
		Code: http.StatusOK,
		JSON: uploadResponse{
			ContentURI: result.ContentURI,
		},
	}
}

// uploadRequest metadata included in or derivable from an upload request
// https://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-media-r0-upload
// NOTE: The members come from HTTP request metadata such as headers, query parameters or can be derived from such
type uploadRequest struct {
	MediaMetadata *types.MediaMetadata
	FileData      io.Reader
	closeFn       func() error
}

// uploadResponse defines the format of the JSON response
// https://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-media-r0-upload
type uploadResponse struct {
	ContentURI string `json:"content_uri"`
}

// parseAndValidateRequest parses the incoming upload request to validate and extract
// all the metadata about the media being uploaded.
// Returns either an uploadRequest or an error formatted as a util.JSONResponse
func parseAndValidateRequest(req *http.Request, cfg *config.FilesConfig, ownerID types.OwnerID) (*uploadRequest, *util.JSONResponse) {
	filename := strings.TrimSpace(req.URL.Query().Get("filename"))

	r := &uploadRequest{
		MediaMetadata: &types.MediaMetadata{
			FileSizeBytes: types.FileSizeBytes(req.ContentLength),
			ContentType:   types.ContentType(req.Header.Get("Content-Type")),
			UploadName:    types.Filename(filename),
			OwnerID:       ownerID,
		},
		FileData: req.Body,
		closeFn:  req.Body.Close,
	}

	contentTypeHeader := req.Header.Get("Content-Type")
	contentType, _, err := mime.ParseMediaType(contentTypeHeader)
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentTypeHeader)), "multipart/form-data") {
		if err != nil {
			return nil, &util.JSONResponse{
				Code: http.StatusBadRequest,
				JSON: map[string]interface{}{
					"errcode": "M_UNKNOWN",
					"error":   "Invalid multipart payload",
				},
			}
		}
	}
	if err == nil && strings.EqualFold(contentType, "multipart/form-data") {
		multipartReq, parseErr := parseMultipartUpload(req, ownerID, types.Filename(filename))
		if parseErr != nil {
			return nil, parseErr
		}
		r = multipartReq
	}

	if resErr := r.Validate(cfg.MaxFileSizeBytes); resErr != nil {
		_ = r.Close()
		return nil, resErr
	}

	return r, nil
}

func parseMultipartUpload(req *http.Request, ownerID types.OwnerID, fallbackFilename types.Filename) (*uploadRequest, *util.JSONResponse) {
	reader, err := req.MultipartReader()
	if err != nil {
		return nil, &util.JSONResponse{
			Code: http.StatusBadRequest,
			JSON: map[string]interface{}{
				"errcode": "M_UNKNOWN",
				"error":   "Invalid multipart payload",
			},
		}
	}

	request := &uploadRequest{
		MediaMetadata: &types.MediaMetadata{
			FileSizeBytes: -1,
			OwnerID:       ownerID,
			UploadName:    fallbackFilename,
		},
		closeFn: req.Body.Close,
	}

	var filePart *multipart.Part
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, &util.JSONResponse{
				Code: http.StatusBadRequest,
				JSON: map[string]interface{}{
					"errcode": "M_UNKNOWN",
					"error":   "Invalid multipart payload",
				},
			}
		}

		formName := strings.TrimSpace(part.FormName())
		if formName == "filename" && request.MediaMetadata.UploadName == "" {
			limited := io.LimitReader(part, 4096)
			content, _ := io.ReadAll(limited)
			name := strings.TrimSpace(string(content))
			if name != "" {
				request.MediaMetadata.UploadName = types.Filename(name)
			}
			_ = part.Close()
			continue
		}

		if part.FileName() == "" {
			_ = part.Close()
			continue
		}

		if filePart != nil {
			_ = part.Close()
			_ = filePart.Close()
			return nil, &util.JSONResponse{
				Code: http.StatusBadRequest,
				JSON: map[string]interface{}{
					"errcode": "M_UNKNOWN",
					"error":   "Multipart payload must contain exactly one file part",
				},
			}
		}

		filePart = part
		request.FileData = filePart

		if request.MediaMetadata.UploadName == "" {
			request.MediaMetadata.UploadName = types.Filename(part.FileName())
		}

		if contentType := part.Header.Get("Content-Type"); contentType != "" {
			request.MediaMetadata.ContentType = types.ContentType(contentType)
		}
	}

	if filePart == nil {
		return nil, &util.JSONResponse{
			Code: http.StatusBadRequest,
			JSON: map[string]interface{}{
				"errcode": "M_UNKNOWN",
				"error":   "Multipart payload does not include a file",
			},
		}
	}

	return request, nil
}

// Validate validates the uploadRequest fields
func (r *uploadRequest) Validate(maxFileSizeBytes config.FileSizeBytes) *util.JSONResponse {
	if maxFileSizeBytes > 0 && r.MediaMetadata.FileSizeBytes >= 0 && r.MediaMetadata.FileSizeBytes > types.FileSizeBytes(maxFileSizeBytes) {
		return requestEntityTooLargeJSONResponse()
	}
	if path.Base(string(r.MediaMetadata.UploadName)) != string(r.MediaMetadata.UploadName) {
		return &util.JSONResponse{
			Code: http.StatusBadRequest,
			JSON: map[string]interface{}{
				"errcode": "M_UNKNOWN",
				"error":   "Filename must not contain path separators",
			},
		}
	}
	return nil
}

func (r *uploadRequest) Close() error {
	if r == nil || r.closeFn == nil {
		return nil
	}
	return r.closeFn()
}

func requestEntityTooLargeJSONResponse() *util.JSONResponse {
	return &util.JSONResponse{
		Code: http.StatusRequestEntityTooLarge,
		JSON: map[string]interface{}{
			"errcode": "M_UNKNOWN",
			"error":   "HTTP Content-Length is greater than the maximum allowed upload size",
		},
	}
}

func queueThumbnailGeneration(ctx context.Context, service *frame.Service, mediaID types.MediaID) error {
	cfg := service.Config().(*config.FilesConfig)
	thumbnailGenerationQueue := cfg.QueueThumbnailsGenerateName
	return service.QueueManager().Publish(ctx, thumbnailGenerationQueue, map[string]string{
		"media_id": string(mediaID),
	})
}
