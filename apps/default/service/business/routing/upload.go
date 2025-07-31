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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/antinvestor/gomatrixserverlib"
	"github.com/antinvestor/gomatrixserverlib/spec"
	"github.com/antinvestor/service-files/apps/default/config"
	storage2 "github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/util"
)

// uploadRequest metadata included in or derivable from an upload request
// https://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-media-r0-upload
// NOTE: The members come from HTTP request metadata such as headers, query parameters or can be derived from such
type uploadRequest struct {
	MediaMetadata *types.MediaMetadata
	Logger        *util.LogEntry
}

// uploadResponse defines the format of the JSON response
// https://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-media-r0-upload
type uploadResponse struct {
	ContentURI string `json:"content_uri"`
}

// Upload implements POST /upload
// This endpoint involves uploading potentially significant amounts of data.
// This implementation supports a configurable maximum file size limit in bytes. If a user tries to upload more than this, they will receive an error that their upload is too large.
// Uploaded files are processed piece-wise to avoid DoS attacks which would starve the server of memory.
// TODO: We should time out requests if they have not received any data within a configured timeout period.
func Upload(req *http.Request, service *frame.Service, db storage2.Database, provider storage2.Provider) util.JSONResponse {

	ctx := req.Context()
	authClaims := frame.ClaimsFromContext(ctx)

	cfg := service.Config().(*config.FilesConfig)

	if authClaims == nil {
		return util.JSONResponse{
			Code: http.StatusUnauthorized,
			JSON: spec.Unknown("Unauthorised"),
		}
	}

	sub, err := authClaims.GetSubject()

	if err != nil {
		return util.JSONResponse{
			Code: http.StatusUnauthorized,
			JSON: spec.Unknown("Unauthorised"),
		}
	}

	ownerID := types.OwnerID(sub)
	r, resErr := parseAndValidateRequest(req, cfg, ownerID)
	if resErr != nil {
		return *resErr
	}

	if resErr = r.doUpload(req.Context(), ownerID, req.Body, cfg, db, provider); resErr != nil {
		return *resErr
	}

	err = queueThumbnailGeneration(ctx, service, r.MediaMetadata.MediaID)
	if err != nil {
		return util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: spec.Unknown("Failed to generate thumbnails"),
		}
	}

	return util.JSONResponse{
		Code: http.StatusOK,
		JSON: uploadResponse{
			ContentURI: fmt.Sprintf("mxc://%s/%s", r.MediaMetadata.ServerName, r.MediaMetadata.MediaID),
		},
	}
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
		Logger: util.Log(req.Context()),
	}

	if resErr := r.Validate(cfg.MaxFileSizeBytes); resErr != nil {
		return nil, resErr
	}

	return r, nil
}

func (r *uploadRequest) generateMediaID(ctx context.Context) types.MediaID {

	model := frame.BaseModel{}
	model.GenID(ctx)

	return types.MediaID(model.GetID())
}

func (r *uploadRequest) doUpload(
	ctx context.Context,
	ownerID types.OwnerID,
	reqReader io.Reader,
	cfg *config.FilesConfig,
	db storage2.Database,
	provider storage2.Provider,
) *util.JSONResponse {

	r.Logger.With(
		"UploadName", r.MediaMetadata.UploadName,
		"FileSizeBytes", r.MediaMetadata.FileSizeBytes,
		"ContentType", r.MediaMetadata.ContentType,
	).Info("Uploading file")

	// The file data is hashed and the hash is used as the MediaID. The hash is useful as a
	// method of deduplicating files to save storage, as well as a way to conduct
	// integrity checks on the file data in the repository.
	// Data is truncated to maxFileSizeBytes. Content-Length was reported as 0 < Content-Length <= maxFileSizeBytes so this is OK.
	//
	// TODO: This has a bad API shape where you either need to call:
	//   utils.RemoveDir(tmpDir, r.Logger)
	// or call:
	//   r.storeFileAndMetadata(ctx, tmpDir, ...)
	// before you return from doUpload else we will leak a temp file. We could make this nicer with a `WithTransaction` style of
	// nested function to guarantee either storage or cleanup.
	if cfg.MaxFileSizeBytes > 0 {
		if cfg.MaxFileSizeBytes+1 <= 0 {
			r.Logger.With(
				"MaxFileSizeBytes", cfg.MaxFileSizeBytes,
				"Default File SizeBytes", config.DefaultMaxFileSizeBytes,
			).Warn("Configured MaxFileSizeBytes overflows int64")
			cfg.MaxFileSizeBytes = config.DefaultMaxFileSizeBytes
		}
		reqReader = io.LimitReader(reqReader, int64(cfg.MaxFileSizeBytes)+1)
	}

	hash, bytesWritten, tmpDir, err := utils.WriteTempFile(ctx, reqReader, cfg.AbsBasePath)
	if err != nil {
		r.Logger.WithError(err).With(
			"MaxFileSizeBytes", cfg.MaxFileSizeBytes,
		).Warn("Error while transferring file")
		return &util.JSONResponse{
			Code: http.StatusBadRequest,
			JSON: spec.Unknown("Failed to upload"),
		}
	}

	// Check if temp file size exceeds max file size configuration
	if cfg.MaxFileSizeBytes > 0 && bytesWritten > types.FileSizeBytes(cfg.MaxFileSizeBytes) {
		utils.RemoveDir(tmpDir, r.Logger) // delete temp file
		return requestEntityTooLargeJSONResponse(cfg.MaxFileSizeBytes)
	}

	// Look up the media by the file hash. If we already have the file but under a
	// different media ID then we won't upload the file again - instead we'll just
	// add a new metadata entry that refers to the same file.
	existingMetadata, err := db.GetMediaMetadataByHash(ctx, ownerID, hash)
	if err != nil {
		utils.RemoveDir(tmpDir, r.Logger)
		r.Logger.WithError(err).Error("Error querying the database by hash.")
		return &util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: spec.InternalServerError{},
		}
	}
	if existingMetadata != nil {
		// The file already exists, delete the uploaded temporary file.
		defer utils.RemoveDir(tmpDir, r.Logger)

		// Then amend the upload metadata.
		r.MediaMetadata = existingMetadata
	} else {

		// The file doesn't exist. Update the request metadata.
		r.MediaMetadata.FileSizeBytes = bytesWritten
		r.MediaMetadata.Base64Hash = hash
		r.MediaMetadata.MediaID = r.generateMediaID(ctx)
	}

	r.Logger = r.Logger.WithField("media_id", r.MediaMetadata.MediaID)
	r.Logger.With(
		"Base64Hash", r.MediaMetadata.Base64Hash,
		"UploadName", r.MediaMetadata.UploadName,
		"FileSizeBytes", r.MediaMetadata.FileSizeBytes,
		"ContentType", r.MediaMetadata.ContentType,
	).Info("File uploaded")

	err = r.storeFileAndMetadata(ctx, tmpDir, cfg.AbsBasePath, db, provider)
	if err != nil {
		r.Logger.WithError(err).Error("Failed to upload file.")
		return &util.JSONResponse{
			Code: http.StatusBadRequest,
			JSON: spec.Unknown(err.Error()),
		}
	}

	return nil

}

func requestEntityTooLargeJSONResponse(maxFileSizeBytes config.FileSizeBytes) *util.JSONResponse {
	return &util.JSONResponse{
		Code: http.StatusRequestEntityTooLarge,
		JSON: spec.Unknown(fmt.Sprintf("HTTP Content-Length is greater than the maximum allowed upload size (%v).", maxFileSizeBytes)),
	}
}

// Validate validates the uploadRequest fields
func (r *uploadRequest) Validate(maxFileSizeBytes config.FileSizeBytes) *util.JSONResponse {
	if maxFileSizeBytes > 0 && r.MediaMetadata.FileSizeBytes > types.FileSizeBytes(maxFileSizeBytes) {
		return requestEntityTooLargeJSONResponse(maxFileSizeBytes)
	}
	if strings.HasPrefix(string(r.MediaMetadata.UploadName), "~") {
		return &util.JSONResponse{
			Code: http.StatusBadRequest,
			JSON: spec.Unknown("File name must not begin with '~'."),
		}
	}
	// TODO: Validate filename - what are the valid characters?
	if r.MediaMetadata.OwnerID != "" {
		// TODO: We should put user ID parsing code into gomatrixserverlib and use that instead
		//       (see https://github.com/antinvestor/gomatrixserverlib/blob/3394e7c7003312043208aa73727d2256eea3d1f6/eventcontent.go#L347 )
		//       It should be a struct (with pointers into a single string to avoid copying) and
		//       we should update all refs to use OwnerID types rather than strings.
		// https://github.com/matrix-org/synapse/blob/v0.19.2/synapse/types.py#L92
		if _, _, err := gomatrixserverlib.SplitID('@', string(r.MediaMetadata.OwnerID)); err != nil {
			return &util.JSONResponse{
				Code: http.StatusBadRequest,
				JSON: spec.BadJSON("user id must be in the form @localpart:domain"),
			}
		}
	}
	return nil
}

// storeFileAndMetadata moves the temporary file to its final path based on metadata and stores the metadata in the database
// See getPathFromMediaMetadata in fileutils for details of the final path.
// The order of operations is important as it avoids metadata entering the database before the file
// is ready, and if we fail to move the file, it never gets added to the database.
// Returns a util.JSONResponse error and cleans up directories in case of error.
func (r *uploadRequest) storeFileAndMetadata(
	ctx context.Context,
	tmpDir types.Path,
	absBasePath config.Path,
	db storage2.Database,
	provider storage2.Provider,
) error {
	finalPath, duplicate, err := storage2.UploadFileWithHashCheck(ctx, provider, tmpDir, r.MediaMetadata, absBasePath, r.Logger)
	if err != nil {
		return err
	}
	if duplicate {
		r.Logger.WithField("dst", finalPath).Info("File was stored previously - discarding duplicate")
	}

	if err = db.StoreMediaMetadata(ctx, r.MediaMetadata); err != nil {
		r.Logger.WithError(err).Warn("Failed to store metadata")
		// If the file is a duplicate (has the same hash as an existing file) then
		// there is valid metadata in the database for that file. As such we only
		// remove the file if it is not a duplicate.
		if !duplicate {
			utils.RemoveDir(types.Path(path.Dir(string(finalPath))), r.Logger)
		}
		return err
	}

	return nil
}

func queueThumbnailGeneration(ctx context.Context, service *frame.Service, mediaID types.MediaID) error {
	cfg := service.Config().(*config.FilesConfig)
	thumbnailGenerationQueue := cfg.QueueThumbnailsGenerateName
	return service.Publish(ctx, thumbnailGenerationQueue, map[string]string{
		"media_id": string(mediaID),
	})
}
