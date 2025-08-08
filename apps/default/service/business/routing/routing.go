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
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/antinvestor/gomatrixserverlib/spec"
	"github.com/antinvestor/service-files/apps/default/config"
	storage2 "github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/util"
)

const (
	PublicClientPathPrefix = "/_matrix/client/"
	PublicMediaPathPrefix  = "/_matrix/media/"
	PublicStaticPath       = "/_matrix/static/"
	PublicAPISpecPath      = "/swagger.json"
)

type ctxValueString string

// Route represents a single route with pattern and handler
type Route struct {
	Pattern string
	Handler http.Handler
	Methods []string
	Regex   *regexp.Regexp
}

// Router is a custom router that replaces mux functionality
type Router struct {
	routes []Route
	prefix string
}

// NewRouter creates a new custom router
func NewRouter() *Router {
	return &Router{
		routes: make([]Route, 0),
	}
}

// PathPrefix creates a subrouter with the given prefix
func (r *Router) PathPrefix(prefix string) *Router {
	return &Router{
		routes: make([]Route, 0),
		prefix: r.prefix + prefix,
	}
}

// Handle adds a route with the given pattern and handler
func (r *Router) Handle(pattern string, handler http.Handler) *RouteBuilder {
	fullPattern := r.prefix + pattern
	route := Route{
		Pattern: fullPattern,
		Handler: handler,
		Methods: []string{},
	}

	// Convert pattern to regex for path variable extraction
	regexPattern := regexp.QuoteMeta(fullPattern)
	regexPattern = strings.ReplaceAll(regexPattern, "\\{serverName\\}", "([^/]+)")
	regexPattern = strings.ReplaceAll(regexPattern, "\\{mediaId\\}", "([^/]+)")
	regexPattern = strings.ReplaceAll(regexPattern, "\\{downloadName\\}", "([^/]+)")
	regexPattern = "^" + regexPattern + "$"

	route.Regex = regexp.MustCompile(regexPattern)
	r.routes = append(r.routes, route)

	return &RouteBuilder{router: r, routeIndex: len(r.routes) - 1}
}

// RouteBuilder allows method chaining for route configuration
type RouteBuilder struct {
	router     *Router
	routeIndex int
}

// Methods sets the allowed HTTP methods for the route
func (rb *RouteBuilder) Methods(methods ...string) *RouteBuilder {
	rb.router.routes[rb.routeIndex].Methods = methods
	return rb
}

// ServeHTTP implements http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	method := req.Method

	for _, route := range r.routes {
		if route.Regex.MatchString(path) {
			// Check if method is allowed
			if len(route.Methods) > 0 {
				methodAllowed := false
				for _, allowedMethod := range route.Methods {
					if method == allowedMethod {
						methodAllowed = true
						break
					}
				}
				if !methodAllowed {
					NotAllowedHandler.ServeHTTP(w, req)
					return
				}
			}

			// Extract path variables and add to request context
			matches := route.Regex.FindStringSubmatch(path)
			if len(matches) > 1 {
				vars := make(map[string]string)

				// Map matches to variable names based on the pattern
				if strings.Contains(route.Pattern, "{serverName}") && strings.Contains(route.Pattern, "{mediaId}") {
					if strings.Contains(route.Pattern, "{downloadName}") {
						// Pattern: /download/{serverName}/{mediaId}/{downloadName}
						if len(matches) >= 4 {
							vars["serverName"] = matches[1]
							vars["mediaId"] = matches[2]
							vars["downloadName"] = matches[3]
						}
					} else {
						// Pattern: /download/{serverName}/{mediaId} or /thumbnail/{serverName}/{mediaId}
						if len(matches) >= 3 {
							vars["serverName"] = matches[1]
							vars["mediaId"] = matches[2]
						}
					}
				}

				ctx := context.WithValue(req.Context(), ctxValueString("pathVars"), vars)
				req = req.WithContext(ctx)
			}

			route.Handler.ServeHTTP(w, req)
			return
		}
	}

	// No route found
	NotFoundCORSHandler.ServeHTTP(w, req)
}

// GetPathVars extracts path variables from request context (replaces mux.Vars)
func GetPathVars(req *http.Request) map[string]string {
	if vars := req.Context().Value(ctxValueString("pathVars")); vars != nil {
		if pathVars, ok := vars.(map[string]string); ok {
			return pathVars
		}
	}
	return make(map[string]string)
}

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
	service *frame.Service,
	db storage2.Database,
	provider storage2.Provider,
) *Router {

	cfg := service.Config().(*config.FilesConfig)

	matrixPathsRouter := NewRouter()

	// Add OpenAPI spec route at root
	matrixPathsRouter.Handle(PublicAPISpecPath, http.HandlerFunc(ServeOpenAPISpec)).Methods(http.MethodGet, http.MethodOptions)

	// Add search endpoint at /media/search
	searchHandler := CreateHandler(
		func(req *http.Request) util.JSONResponse {
			return Search(req, service, db)
		})
	matrixPathsRouter.Handle("/media/search", searchHandler).Methods(http.MethodGet, http.MethodOptions)

	ClientRouters := matrixPathsRouter.PathPrefix(PublicClientPathPrefix)

	v1mux := ClientRouters.PathPrefix("/v1/media/")

	uploadHandler := CreateHandler(
		func(req *http.Request) util.JSONResponse {
			return Upload(req, service, db, provider)
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

	// TODO: Implement the endpoints for Create new mxc:// URIs and upload content to mxc:// URIs

	downloadHandlerAuthed := makeDownloadAPI("download_client", cfg, db, provider)
	v1mux.Handle("/config", configHandler).Methods(http.MethodGet, http.MethodOptions)
	v1mux.Handle("/download/{serverName}/{mediaId}", downloadHandlerAuthed).Methods(http.MethodGet, http.MethodOptions)
	v1mux.Handle("/download/{serverName}/{mediaId}/{downloadName}", downloadHandlerAuthed).Methods(http.MethodGet, http.MethodOptions)

	v1mux.Handle("/thumbnail/{serverName}/{mediaId}", makeDownloadAPI("thumbnail_authed_client", cfg, db, provider)).Methods(http.MethodGet, http.MethodOptions)

	return matrixPathsRouter
}

// ServeOpenAPISpec serves the OpenAPI specification file
func ServeOpenAPISpec(w http.ResponseWriter, req *http.Request) {
	// Set CORS headers
	util.SetCORSHeaders(w)

	// Handle OPTIONS request
	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		errorResponse, _ := json.Marshal(spec.InternalServerError{})
		_, _ = w.Write(errorResponse)
		return
	}

	// Construct path to openapi.yaml
	openAPIPath := filepath.Join(wd, "api", "openapi.yaml")

	// Read the OpenAPI spec file
	content, err := os.ReadFile(openAPIPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		errorResponse, _ := json.Marshal(spec.NotFound("OpenAPI specification not found"))
		_, _ = w.Write(errorResponse)
		return
	}

	// Set content type and write the YAML content
	w.Header().Set("Content-Type", "application/x-yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func makeDownloadAPI(
	name string,
	cfg *config.FilesConfig,
	db storage2.Database,
	provider storage2.Provider,
) http.HandlerFunc {

	httpHandler := func(w http.ResponseWriter, req *http.Request) {
		req = util.RequestWithLogging(req)

		// Set internal headers returned regardless of the outcome of the request
		util.SetCORSHeaders(w)
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		// Content-Type will be overridden in case of returning file data, else we respond with JSON-formatted errors
		w.Header().Set("Content-Type", "application/json")

		vars, _ := URLDecodeMapValues(GetPathVars(req))
		_ = spec.ServerName(vars["serverName"])

		// CacheOptions media for at least one day.
		w.Header().Set("Cache-Control", "public,max-age=86400,s-maxage=86400")

		Download(w, req, types.MediaID(vars["mediaId"]),
			cfg, db, provider,
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
