package routing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/authz"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RoutingTestSuite struct {
	tests.BaseTestSuite
}

func TestRoutingTestSuite(t *testing.T) {
	suite.Run(t, new(RoutingTestSuite))
}

func (suite *RoutingTestSuite) Test_IsValidMediaID() {
	testCases := []struct {
		name     string
		mediaID  string
		expected bool
	}{
		{
			name:     "valid media ID",
			mediaID:  "AbCdEf1234567890",
			expected: true,
		},
		{
			name:     "valid media ID with more characters",
			mediaID:  "validMediaID1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			expected: true,
		},
		{
			name:     "invalid media ID with special chars",
			mediaID:  "invalid@id",
			expected: false,
		},
		{
			name:     "invalid media ID with spaces",
			mediaID:  "invalid id",
			expected: false,
		},
		{
			name:     "empty media ID",
			mediaID:  "",
			expected: false,
		},
		{
			name:     "short but valid media ID",
			mediaID:  "abc",
			expected: true, // Actually valid according to the validation function
		},
	}

	for _, tc := range testCases {
		t := suite.T()
		t.Run(tc.name, func(t *testing.T) {
			result := isValidMediaID(tc.mediaID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *RoutingTestSuite) TestGetPathVars() {
	testCases := []struct {
		name         string
		path         string
		wantServer   string
		wantMediaID  string
		wantDownload string
	}{
		{
			name:         "download_without_name",
			path:         "/v1/media/download/serverA/mediaA",
			wantServer:   "serverA",
			wantMediaID:  "mediaA",
			wantDownload: "",
		},
		{
			name:         "download_with_name",
			path:         "/v1/media/download/serverA/mediaA/file.txt",
			wantServer:   "serverA",
			wantMediaID:  "mediaA",
			wantDownload: "file.txt",
		},
		{
			name:         "thumbnail_path",
			path:         "/v1/media/thumbnail/serverB/mediaB",
			wantServer:   "serverB",
			wantMediaID:  "mediaB",
			wantDownload: "",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			vars := GetPathVars(req)
			assert.Equal(t, tc.wantServer, vars["serverName"])
			assert.Equal(t, tc.wantMediaID, vars["mediaId"])
			assert.Equal(t, tc.wantDownload, vars["downloadName"])
		})
	}
}

func (suite *RoutingTestSuite) TestURLDecodeMapValues() {
	testCases := []struct {
		name      string
		values    map[string]string
		wantError bool
	}{
		{
			name: "decodes_values",
			values: map[string]string{
				"serverName": "example.com",
				"mediaId":    "abc%2Fdef",
			},
			wantError: false,
		},
		{
			name: "invalid_escape",
			values: map[string]string{
				"mediaId": "%zz",
			},
			wantError: true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := URLDecodeMapValues(tc.values)
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, result["mediaId"])
		})
	}
}

func (suite *RoutingTestSuite) TestWrapHandlerInCORS() {
	testCases := []struct {
		name           string
		method         string
		requestMethod  string
		expectedStatus int
		expectBody     bool
	}{
		{
			name:           "options_preflight",
			method:         http.MethodOptions,
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			expectBody:     false,
		},
		{
			name:           "passes_through",
			method:         http.MethodGet,
			expectedStatus: http.StatusAccepted,
			expectBody:     true,
		},
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	})
	wrapped := WrapHandlerInCORS(h)

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/x", nil)
			if tc.requestMethod != "" {
				req.Header.Set("Access-Control-Request-Method", tc.requestMethod)
			}
			rec := httptest.NewRecorder()
			wrapped(rec, req)

			assert.Equal(t, tc.expectedStatus, rec.Code)
			assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
			if tc.expectBody {
				assert.Contains(t, rec.Body.String(), "ok")
			}
		})
	}
}

func (suite *RoutingTestSuite) TestCreateHandler() {
	testCases := []struct {
		name         string
		response     util.JSONResponse
		expectedCode int
	}{
		{
			name: "writes_json_and_headers",
			response: util.JSONResponse{
				Code: http.StatusCreated,
				Headers: map[string]interface{}{
					"X-Test": "yes",
				},
				JSON: map[string]any{"k": "v"},
			},
			expectedCode: http.StatusCreated,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			h := CreateHandler(func(_ *http.Request) util.JSONResponse { return tc.response })
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedCode, rec.Code)
			assert.Equal(t, "yes", rec.Header().Get("X-Test"))
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.Contains(t, rec.Body.String(), "\"k\":\"v\"")
		})
	}
}

func (suite *RoutingTestSuite) TestRouter_PathPrefixAndCatchAll() {
	testCases := []struct {
		name       string
		method     string
		path       string
		wantCode   int
		wantBody   string
		setupRoute func(router *Router)
	}{
		{
			name:     "subrouter_route_is_reachable",
			method:   http.MethodGet,
			path:     "/v1/media/config",
			wantCode: http.StatusOK,
			wantBody: "ok",
			setupRoute: func(router *Router) {
				sub := router.PathPrefix("/v1")
				sub.Handle("/media/config", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("ok"))
				})).Methods(http.MethodGet)
			},
		},
		{
			name:     "catch_all_route_matches",
			method:   http.MethodGet,
			path:     "/any/path",
			wantCode: http.StatusAccepted,
			wantBody: "catch",
			setupRoute: func(router *Router) {
				router.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusAccepted)
					_, _ = w.Write([]byte("catch"))
				})).Methods(http.MethodGet)
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			router := NewRouter()
			tc.setupRoute(router)

			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantCode, rec.Code)
			assert.Equal(t, tc.wantBody, rec.Body.String())
		})
	}
}

func (suite *RoutingTestSuite) TestSetupMediaRoutes() {
	testCases := []struct {
		name       string
		method     string
		path       string
		claimsSub  string
		body       string
		wantStatus int
	}{
		{
			name:       "config_route_available",
			method:     http.MethodGet,
			path:       "/v1/media/config",
			claimsSub:  "@owner:example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "upload_requires_auth",
			method:     http.MethodPost,
			path:       "/v1/media/upload?filename=x.txt",
			body:       "content",
			wantStatus: http.StatusUnauthorized,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, svc, res := suite.CreateService(t, dep)
				cfg := svc.Config().(*config.FilesConfig)

				db := &connection.Database{
					WorkManager:     svc.WorkManager(),
					MediaRepository: res.MediaRepository,
				}

				storageProvider, err := provider.GetStorageProvider(ctx, cfg)
				require.NoError(t, err)

				mediaService := business.NewMediaService(db, storageProvider)
				authzMiddleware := authz.NewMiddleware(svc.SecurityManager().GetAuthorizer(ctx), db)
				router := SetupMediaRoutes(svc, db, storageProvider, mediaService, authzMiddleware)

				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				if tc.body != "" {
					req.Header.Set("Content-Type", "text/plain")
					req.ContentLength = int64(len(tc.body))
				}
				if tc.claimsSub != "" {
					claims := &security.AuthenticationClaims{
						RegisteredClaims: jwt.RegisteredClaims{Subject: tc.claimsSub},
					}
					req = req.WithContext(claims.ClaimsToContext(req.Context()))
				}

				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)

				assert.Equal(t, tc.wantStatus, rec.Code)
			})
		}
	})
}

func (suite *RoutingTestSuite) TestMakeDownloadAPI() {
	testCases := []struct {
		name           string
		mediaID        string
		path           string
		claimsSubject  string
		isThumbnail    bool
		expectCode     int
		expectedHeader string
	}{
		{
			name:           "download_streams_content",
			mediaID:        "mediaDownloadAuth",
			path:           "/v1/media/download/server/mediaDownloadAuth/file.txt",
			claimsSubject:  "@owner:example.com",
			expectCode:     http.StatusOK,
			expectedHeader: "text/plain",
		},
		{
			mediaID:    "mediaDownloadNoAuth",
			name:       "download_requires_auth",
			path:       "/v1/media/download/server/mediaDownloadNoAuth/file.txt",
			expectCode: http.StatusUnauthorized,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, svc, res := suite.CreateService(t, dep)
				cfg := svc.Config().(*config.FilesConfig)

				db := &connection.Database{
					WorkManager:     svc.WorkManager(),
					MediaRepository: res.MediaRepository,
				}
				storageProvider, err := provider.GetStorageProvider(ctx, cfg)
				require.NoError(t, err)
				mediaService := business.NewMediaService(db, storageProvider)
				authzMiddleware := authz.NewMiddleware(svc.SecurityManager().GetAuthorizer(ctx), db)

				content := "sample-" + tc.mediaID
				_, err = mediaService.UploadFile(ctx, &business.UploadRequest{
					OwnerID:       types.OwnerID("@owner:example.com"),
					MediaID:       types.MediaID(tc.mediaID),
					UploadName:    "file.txt",
					ContentType:   "text/plain",
					FileSizeBytes: types.FileSizeBytes(len(content)),
					FileData:      strings.NewReader(content),
					Config:        cfg,
				})
				require.NoError(t, err)

				handler := makeDownloadAPI("download_client", cfg, db, storageProvider, mediaService, authzMiddleware)
				req := httptest.NewRequest(http.MethodGet, tc.path, nil)
				if tc.claimsSubject != "" {
					claims := &security.AuthenticationClaims{
						RegisteredClaims: jwt.RegisteredClaims{Subject: tc.claimsSubject},
					}
					req = req.WithContext(claims.ClaimsToContext(context.Background()))
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				assert.Equal(t, tc.expectCode, rec.Code)
				if tc.expectedHeader != "" {
					assert.Equal(t, tc.expectedHeader, rec.Header().Get("Content-Type"))
				}
			})
		}
	})
}
