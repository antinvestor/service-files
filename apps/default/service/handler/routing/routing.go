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
	"regexp"
	"strings"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/authz"
	"github.com/antinvestor/service-files/apps/default/service/business"
	storage2 "github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/util"
)

const (
	PublicMediaPathPrefix = "/v1/media/"
)

type ctxValueString string

// Route represents a single route with pattern and handler
type Route struct {
	Pattern string
	Handler http.Handler
	Methods []string
	Regex   *regexp.Regexp
	Prefix  bool
}

// Router is a custom router that replaces mux functionality
type Router struct {
	routes *[]Route
	prefix string
}

// NewRouter creates a new custom router
func NewRouter() *Router {
	routes := make([]Route, 0)
	return &Router{
		routes: &routes,
	}
}

// PathPrefix creates a subrouter with the given prefix
func (r *Router) PathPrefix(prefix string) *Router {
	return &Router{
		routes: r.routes,
		prefix: joinRoutePath(r.prefix, prefix),
	}
}

// Handle adds a route with the given pattern and handler
func (r *Router) Handle(pattern string, handler http.Handler) *RouteBuilder {
	fullPattern := joinRoutePath(r.prefix, pattern)
	route := Route{
		Pattern: fullPattern,
		Handler: handler,
		Methods: []string{},
	}

	// Convert pattern to regex for path variable extraction
	patternForRegex := fullPattern
	if strings.HasSuffix(fullPattern, "*") {
		route.Prefix = true
		patternForRegex = strings.TrimSuffix(fullPattern, "*")
	}

	regexPattern := regexp.QuoteMeta(patternForRegex)
	regexPattern = strings.ReplaceAll(regexPattern, "\\{serverName\\}", "([^/]+)")
	regexPattern = strings.ReplaceAll(regexPattern, "\\{mediaId\\}", "([^/]+)")
	regexPattern = strings.ReplaceAll(regexPattern, "\\{downloadName\\}", "([^/]+)")
	if route.Prefix {
		regexPattern = "^" + regexPattern + ".*$"
	} else {
		regexPattern = "^" + regexPattern + "$"
	}

	route.Regex = regexp.MustCompile(regexPattern)
	*r.routes = append(*r.routes, route)

	return &RouteBuilder{router: r, routeIndex: len(*r.routes) - 1}
}

func joinRoutePath(base, segment string) string {
	if base == "" {
		if segment == "" {
			return "/"
		}
		if strings.HasPrefix(segment, "/") {
			return segment
		}
		return "/" + segment
	}
	if segment == "" {
		return base
	}
	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(segment, "/")
}

// RouteBuilder allows method chaining for route configuration
type RouteBuilder struct {
	router     *Router
	routeIndex int
}

// Methods sets the HTTP methods for the route
func (rb *RouteBuilder) Methods(methods ...string) *RouteBuilder {
	(*rb.router.routes)[rb.routeIndex].Methods = methods
	return rb
}

// ServeHTTP implements the http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, route := range *r.routes {
		if r.matchesRoute(route, req) {
			// Extract path variables and add them to request context
			vars := r.extractPathVars(route, req.URL.Path)
			ctx := req.Context()
			for key, value := range vars {
				ctx = context.WithValue(ctx, ctxValueString(key), value)
			}
			req = req.WithContext(ctx)

			route.Handler.ServeHTTP(w, req)
			return
		}
	}

	// No matching route found
	http.NotFound(w, req)
}

// matchesRoute checks if the request matches the route
func (r *Router) matchesRoute(route Route, req *http.Request) bool {
	// Check HTTP method
	if len(route.Methods) > 0 {
		methodMatch := false
		for _, method := range route.Methods {
			if req.Method == method {
				methodMatch = true
				break
			}
		}
		if !methodMatch {
			return false
		}
	}

	// Check path pattern
	return route.Regex.MatchString(req.URL.Path)
}

// extractPathVars extracts variables from the request path using the route regex
func (r *Router) extractPathVars(route Route, path string) map[string]string {
	matches := route.Regex.FindStringSubmatch(path)
	if len(matches) == 0 {
		return nil
	}

	// Extract group names from the pattern
	varNames := []string{"serverName", "mediaId", "downloadName"}
	vars := make(map[string]string)

	for i, name := range varNames {
		if i+1 < len(matches) {
			vars[name] = matches[i+1]
		}
	}

	return vars
}

// SetupMediaRoutes sets up all the media API routes
// Note: This function has high cyclomatic complexity but is necessary for route setup
// nolint: gocyclo
func SetupMediaRoutes(
	service *frame.Service,
	db storage2.Database,
	provider storage2.Provider,
	mediaService business.MediaService,
	authzMiddleware authz.Middleware,
) *Router {
	cfg := service.Config().(*config.FilesConfig)
	mediaRouter := NewRouter()

	// Add search endpoint at /media/search
	searchHandler := CreateHandler(
		func(req *http.Request) util.JSONResponse {
			return Search(req, service, db, mediaService)
		})
	mediaRouter.Handle("/v1/media/search", searchHandler).Methods(http.MethodGet, http.MethodOptions)

	v1mux := mediaRouter.PathPrefix(PublicMediaPathPrefix)

	uploadHandler := CreateHandler(
		func(req *http.Request) util.JSONResponse {
			return Upload(req, service, db, provider, mediaService, authzMiddleware)
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
	v1mux.Handle("/config", configHandler).Methods(http.MethodGet, http.MethodOptions)

	// Download endpoints
	downloadHandlerAuthed := makeDownloadAPI("download_client", cfg, db, provider, mediaService, authzMiddleware)
	v1mux.Handle("/download/{serverName}/{mediaId}", downloadHandlerAuthed).Methods(http.MethodGet, http.MethodOptions)
	v1mux.Handle("/download/{serverName}/{mediaId}/{downloadName}", downloadHandlerAuthed).Methods(http.MethodGet, http.MethodOptions)

	v1mux.Handle("/thumbnail/{serverName}/{mediaId}", makeDownloadAPI("thumbnail_authed_client", cfg, db, provider, mediaService, authzMiddleware)).Methods(http.MethodGet, http.MethodOptions)

	return mediaRouter
}

func makeDownloadAPI(
	name string,
	cfg *config.FilesConfig,
	db storage2.Database,
	provider storage2.Provider,
	mediaService business.MediaService,
	authzMiddleware authz.Middleware,
) http.HandlerFunc {
	httpHandler := func(w http.ResponseWriter, req *http.Request) {
		req = util.RequestWithLogging(req)

		// Set internal headers returned regardless of the outcome of the request
		util.SetCORSHeaders(w)
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		// Content-Type will be overridden in case of returning file data, else we respond with JSON-formatted errors
		w.Header().Set("Content-Type", "application/json")

		vars, _ := URLDecodeMapValues(GetPathVars(req))
		_ = vars["serverName"]

		// CacheOptions media for at least one day.
		w.Header().Set("Cache-Control", "public,max-age=86400,s-maxage=86400")

		Download(w, req, types.MediaID(vars["mediaId"]),
			cfg, db, provider, mediaService, authzMiddleware,
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

// CreateHandler creates an HTTP handler from a JSON response function
func CreateHandler(f func(*http.Request) util.JSONResponse) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		response := f(req)

		// Set headers
		if response.Headers != nil {
			for key, value := range response.Headers {
				if values, ok := value.([]string); ok {
					for _, v := range values {
						w.Header().Add(key, v)
					}
				} else if str, ok := value.(string); ok {
					w.Header().Add(key, str)
				}
			}
		}

		// Set content type if not already set
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}

		// Write status code and body
		w.WriteHeader(response.Code)
		if response.JSON != nil {
			encoder := json.NewEncoder(w)
			encoder.SetEscapeHTML(false)
			if err := encoder.Encode(response.JSON); err != nil {
				util.Log(req.Context()).WithError(err).Error("Failed to write JSON response")
			}
		}
	})
}

// configResponse represents the configuration response
type configResponse struct {
	UploadSize *config.FileSizeBytes `json:"m.upload.size,omitempty"`
}

// Helper functions for path variable extraction
func GetPathVars(req *http.Request) map[string]string {
	// This is a simplified implementation - in a real scenario you'd want to
	// extract variables from the URL pattern matching
	vars := make(map[string]string)

	// Extract from URL path components
	pathParts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")

	// Look for media API patterns: /v1/media/{download|thumbnail}/{serverName}/{mediaId}[/{downloadName}]
	if len(pathParts) >= 2 && pathParts[0] == "v1" && pathParts[1] == "media" {
		if len(pathParts) >= 5 && (pathParts[2] == "download" || pathParts[2] == "thumbnail") {
			vars["serverName"] = pathParts[3]
			vars["mediaId"] = pathParts[4]
			if len(pathParts) >= 6 {
				vars["downloadName"] = pathParts[5]
			}
		}
	}

	return vars
}

func URLDecodeMapValues(values map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	for key, value := range values {
		decoded, err := url.QueryUnescape(value)
		if err != nil {
			return nil, err
		}
		result[key] = decoded
	}
	return result, nil
}
