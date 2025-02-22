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
	"encoding/json"
	"github.com/antinvestor/gomatrixserverlib/spec"
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/business/storage_provider"
	"github.com/antinvestor/service-files/service/storage"
	"github.com/antinvestor/service-files/service/types"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pitabwire/util"
)

const (
	PublicClientPathPrefix = "/_matrix/client/"
	PublicMediaPathPrefix  = "/_matrix/media/"
	PublicStaticPath       = "/_matrix/static/"
)

// configResponse is the response to GET /_matrix/media/r0/config
// https://matrix.org/docs/spec/client_server/latest#get-matrix-media-r0-config
type configResponse struct {
	UploadSize *config.FileSizeBytes `json:"m.upload.size,omitempty"`
}

// SetupMatrixRoutes registers the media API HTTP handlers
//
// Due to Setup being used to call many other functions, a gocyclo nolint is
// applied:
// nolint: gocyclo
func SetupMatrixRoutes(
	cfg *config.FilesConfig,
	db storage.Database,
	provider storage_provider.Provider,
) *mux.Router {

	matrixPathsRouter := mux.NewRouter().SkipClean(true)
	ClientRouters := matrixPathsRouter.PathPrefix(PublicClientPathPrefix).Subrouter().UseEncodedPath()

	ClientRouters.NotFoundHandler = NotFoundCORSHandler
	ClientRouters.MethodNotAllowedHandler = NotAllowedHandler

	v1mux := ClientRouters.PathPrefix("/v1/media/").Subrouter()

	activeThumbnailGeneration := &types.ActiveThumbnailGeneration{
		PathToResult: map[string]*types.ThumbnailGenerationResult{},
	}

	uploadHandler := CreateHandler(
		func(req *http.Request) util.JSONResponse {
			return Upload(req, cfg, db, provider, activeThumbnailGeneration)
		})

	configHandler := CreateHandler(
		func(req *http.Request) util.JSONResponse {

			respondSize := &cfg.MaxFileSizeBytes
			if cfg.MaxFileSizeBytes == 0 {
				respondSize = nil
			}
			return util.JSONResponse{
				Code: http.StatusOK,
				JSON: configResponse{UploadSize: respondSize},
			}
		})

	v1mux.Handle("/upload", uploadHandler).Methods(http.MethodPost, http.MethodOptions)

	//TODO: Implement the endpoints for Create new mxc:// URIs and upload content to mxc:// URIs

	downloadHandlerAuthed := makeDownloadAPI("download_client", cfg, db, provider, activeThumbnailGeneration)
	v1mux.Handle("/config", configHandler).Methods(http.MethodGet, http.MethodOptions)
	v1mux.Handle("/download/{serverName}/{mediaId}", downloadHandlerAuthed).Methods(http.MethodGet, http.MethodOptions)
	v1mux.Handle("/download/{serverName}/{mediaId}/{downloadName}", downloadHandlerAuthed).Methods(http.MethodGet, http.MethodOptions)

	v1mux.Handle("/thumbnail/{serverName}/{mediaId}", makeDownloadAPI("thumbnail_authed_client", cfg, db, provider, activeThumbnailGeneration)).Methods(http.MethodGet, http.MethodOptions)

	return matrixPathsRouter
}

func makeDownloadAPI(
	name string,
	cfg *config.FilesConfig,
	db storage.Database,
	provider storage_provider.Provider,
	activeThumbnailGeneration *types.ActiveThumbnailGeneration,
) http.HandlerFunc {

	httpHandler := func(w http.ResponseWriter, req *http.Request) {
		req = util.RequestWithLogging(req)

		// Set internal headers returned regardless of the outcome of the request
		util.SetCORSHeaders(w)
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		// Content-Type will be overridden in case of returning file data, else we respond with JSON-formatted errors
		w.Header().Set("Content-Type", "application/json")

		vars, _ := URLDecodeMapValues(mux.Vars(req))
		_ = spec.ServerName(vars["serverName"])

		// CacheOptions media for at least one day.
		w.Header().Set("Cache-Control", "public,max-age=86400,s-maxage=86400")

		Download(w, req, types.MediaID(vars["mediaId"]),
			cfg, db, provider, activeThumbnailGeneration,
			strings.HasPrefix(name, "thumbnail"), vars["downloadName"],
		)
	}

	return httpHandler
}

// WrapHandlerInCORS adds CORS headers to all responses, including all error
// responses.
// Handles OPTIONS requests directly.
func WrapHandlerInCORS(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			// Its easiest just to always return a 200 OK for everything. Whether
			// this is technically correct or not is a question, but in the end this
			// is what a lot of other people do (including synapse) and the clients
			// are perfectly happy with it.
			w.WriteHeader(http.StatusOK)
		} else {
			h.ServeHTTP(w, r)
		}
	}
}

var NotAllowedHandler = WrapHandlerInCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	unrecognizedErr, _ := json.Marshal(spec.Unrecognized("Unrecognized request")) // nolint:misspell
	_, _ = w.Write(unrecognizedErr)                                               // nolint:misspell
}))

var NotFoundCORSHandler = WrapHandlerInCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	unrecognizedErr, _ := json.Marshal(spec.Unrecognized("Unrecognized request")) // nolint:misspell
	_, _ = w.Write(unrecognizedErr)                                               // nolint:misspell
}))

// URLDecodeMapValues is a function that iterates through each of the items in a
// map, URL decodes the value, and returns a new map with the decoded values
// under the same key names
func URLDecodeMapValues(vmap map[string]string) (map[string]string, error) {
	decoded := make(map[string]string, len(vmap))
	for key, value := range vmap {
		decodedVal, err := url.PathUnescape(value)
		if err != nil {
			return make(map[string]string), err
		}
		decoded[key] = decodedVal
	}

	return decoded, nil
}

func CreateHandler(f func(*http.Request) util.JSONResponse) http.Handler {
	return util.MakeJSONAPI(util.NewJSONRequestHandler(f))
}
