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
	"net/http"
	"strconv"
	"strings"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/util"
)

// Download implements GET /download and GET /thumbnail
// Files from remote servers (i.e. origin != cfg.ServerName) are cached locally.
// If they are present in the cache, they are served directly.
// If they are not present in the cache, they are obtained from the remote server and
// simultaneously served back to the client and written into the cache.
func Download(
	w http.ResponseWriter,
	req *http.Request,
	mediaID types.MediaID,
	cfg *config.FilesConfig,
	db storage.Database,
	provider storage.Provider,
	mediaService business.MediaService,
	isThumbnailRequest bool,
	customFilename string,
) {
	// Parse thumbnail parameters if this is a thumbnail request
	var thumbnailSize *types.ThumbnailSize
	if isThumbnailRequest {
		width, err := strconv.Atoi(req.FormValue("width"))
		if err != nil {
			width = -1
		}
		height, err := strconv.Atoi(req.FormValue("height"))
		if err != nil {
			height = -1
		}
		thumbnailSize = &types.ThumbnailSize{
			Width:        width,
			Height:       height,
			ResizeMethod: strings.ToLower(req.FormValue("method")),
		}
	}

	// Create business request
	businessReq := &business.DownloadRequest{
		MediaID:            mediaID,
		IsThumbnailRequest: isThumbnailRequest,
		ThumbnailSize:      thumbnailSize,
		DownloadFilename:   customFilename,
		Config:             cfg,
	}

	// Execute business logic
	result, err := mediaService.DownloadFile(req.Context(), businessReq)
	if err != nil {
		handleDownloadError(req.Context(), w, err)
		return
	}
	defer util.CloseAndLogOnError(req.Context(), result.FileData)

	// Set response headers
	addDownloadHeaders(w, result, customFilename, isThumbnailRequest)

	// Stream the file content
	_, err = io.Copy(w, result.FileData)
	if err != nil {
		util.Log(req.Context()).WithError(err).Error("Failed to stream file content")
	}
}

// addDownloadHeaders adds appropriate headers to the download response
func addDownloadHeaders(w http.ResponseWriter, result *business.DownloadResult, customFilename string, isThumbnailRequest bool) {
	// Set content type
	if result.ContentType != "" {
		w.Header().Set("Content-Type", result.ContentType)
	}

	// Set content length
	if result.ContentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(result.ContentLength, 10))
	}

	// Set content disposition for downloads
	if !isThumbnailRequest && customFilename != "" {
		w.Header().Set("Content-Disposition", "inline; filename=\""+customFilename+"\"")
	}

	// Set cache control headers
	if result.IsCached {
		w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
	}
}

// handleDownloadError handles errors during download and sets appropriate HTTP responses
func handleDownloadError(ctx context.Context, w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)

	// Write error response
	w.Header().Set("Content-Type", "application/json")
	if _, writeErr := w.Write([]byte(`{"error":"` + err.Error() + `"}`)); writeErr != nil {
		util.Log(ctx).WithError(writeErr).Warn("Failed to write error response")
	}
}

// isValidMediaID checks if the media ID is valid
func isValidMediaID(mediaID string) bool {
	if mediaID == "" {
		return false
	}
	// Check if all characters are valid
	for _, r := range mediaID {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '=' || r == '-') {
			return false
		}
	}
	return true
}
