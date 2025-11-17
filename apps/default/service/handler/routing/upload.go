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
	"net/http"
	"net/url"
	"path"

	"github.com/antinvestor/service-files/apps/default/config"
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
func Upload(req *http.Request, service *frame.Service, db storage.Database, provider storage.Provider, mediaService business.MediaService) util.JSONResponse {
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

	ownerID := types.OwnerID(sub)

	// Parse upload request
	uploadReq, resErr := parseAndValidateRequest(req, cfg, ownerID)
	if resErr != nil {
		return *resErr
	}

	// Create business request
	businessReq := &business.UploadRequest{
		OwnerID:       ownerID,
		UploadName:    uploadReq.MediaMetadata.UploadName,
		ContentType:   uploadReq.MediaMetadata.ContentType,
		FileSizeBytes: uploadReq.MediaMetadata.FileSizeBytes,
		FileData:      req.Body,
		Config:        cfg,
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
	r := &uploadRequest{
		MediaMetadata: &types.MediaMetadata{
			FileSizeBytes: types.FileSizeBytes(req.ContentLength),
			ContentType:   types.ContentType(req.Header.Get("Content-Type")),
			UploadName:    types.Filename(url.PathEscape(req.FormValue("filename"))),
			OwnerID:       ownerID,
		},
	}

	if resErr := r.Validate(cfg.MaxFileSizeBytes); resErr != nil {
		return nil, resErr
	}

	return r, nil
}

// Validate validates the uploadRequest fields
func (r *uploadRequest) Validate(maxFileSizeBytes config.FileSizeBytes) *util.JSONResponse {
	if maxFileSizeBytes > 0 && r.MediaMetadata.FileSizeBytes > types.FileSizeBytes(maxFileSizeBytes) {
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
