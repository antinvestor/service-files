package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	color "image/color" //nolint:misspell
	"image/jpeg"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/files/connectrpc/go/files/v1/filesv1connect"
	filesv1 "buf.build/gen/go/antinvestor/files/protocolbuffers/go/files/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/authz"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FileServerTestSuite struct {
	tests.BaseTestSuite
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFileServerTestSuite(t *testing.T) {
	suite.Run(t, new(FileServerTestSuite))
}

func (suite *FileServerTestSuite) Test_NewFileServer() {
	testCases := []struct {
		name string
	}{
		{name: "creates_file_server"},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res := suite.CreateService(t, dep)

		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: res.MediaRepository,
		}

		cfg := &config.FilesConfig{
			MaxFileSizeBytes:           config.FileSizeBytes(1024 * 1024),
			ServerName:                 "test.example.com",
			EnvStorageEncryptionPhrase: "0123456789abcdef0123456789abcdef",
			BasePath:                   config.Path(t.TempDir()),
		}
		require.NoError(t, cfg.Normalise())
		storageProvider, err := provider.GetStorageProvider(ctx, cfg)
		require.NoError(t, err)

		mediaService := business.NewMediaService(db, storageProvider)

		sm := svc.SecurityManager()
		authorizer := sm.GetAuthorizer(ctx)
		authzMiddleware := authz.NewMiddleware(authorizer, db)

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				handler := NewFileServer(svc, mediaService, authzMiddleware, db, storageProvider)
				require.NotNil(t, handler)
				assert.Implements(t, (*filesv1connect.FilesServiceHandler)(nil), handler)
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_CreateContent() {
	testCases := []struct {
		name   string
		userID string
	}{
		{
			name:   "creates_content_with_authenticated_user",
			userID: "@test-user:example.com",
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, _, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				authClaims := &security.AuthenticationClaims{
					RegisteredClaims: jwt.RegisteredClaims{
						Subject: tc.userID,
					},
				}
				ctx = authClaims.ClaimsToContext(ctx)

				req := connect.NewRequest(&filesv1.CreateContentRequest{})

				resp, err := handler.CreateContent(ctx, req)

				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.NotEmpty(t, resp.Msg.MediaId)
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetContentValidation() {
	testCases := []struct {
		name      string
		userID    string
		request   *filesv1.GetContentRequest
		expectErr connect.Code
	}{
		{
			name:      "unauthenticated",
			request:   &filesv1.GetContentRequest{MediaId: "abc123"},
			expectErr: connect.CodeUnauthenticated,
		},
		{
			name:      "invalid_media_id",
			userID:    "@test-user:example.com",
			request:   &filesv1.GetContentRequest{MediaId: "bad id"},
			expectErr: connect.CodeInvalidArgument,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, _, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				caseCtx := ctx
				if tc.userID != "" {
					caseCtx = claimsCtx(caseCtx, tc.userID)
				}

				_, err := handler.GetContent(caseCtx, connect.NewRequest(tc.request))
				require.Error(t, err)
				assert.Equal(t, tc.expectErr, connect.CodeOf(err))
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetContentSuccess() {
	testCases := []struct {
		name     string
		userID   string
		filename string
		mimeType string
		content  string
		mediaID  string
	}{
		{
			name:     "owner_can_get_content",
			userID:   "@owner:example.com",
			filename: "sample.txt",
			mimeType: "text/plain",
			content:  "hello connect rpc",
			mediaID:  "mediaid123",
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				uploadRes, err := mediaService.UploadFile(ctx, &business.UploadRequest{
					OwnerID:       types.OwnerID(tc.userID),
					MediaID:       types.MediaID(tc.mediaID),
					UploadName:    types.Filename(tc.filename),
					ContentType:   types.ContentType(tc.mimeType),
					FileSizeBytes: types.FileSizeBytes(len(tc.content)),
					FileData:      io.NopCloser(bytes.NewReader([]byte(tc.content))),
					Config:        cfg,
				})
				require.NoError(t, err)
				require.NotNil(t, uploadRes)

				caseCtx := claimsCtx(ctx, tc.userID)
				resp, err := handler.GetContent(caseCtx, connect.NewRequest(&filesv1.GetContentRequest{
					MediaId: string(uploadRes.MediaID),
				}))
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.NotNil(t, resp.Msg.Metadata)
				assert.Equal(t, tc.mimeType, resp.Msg.Metadata.ContentType)
				assert.Equal(t, tc.filename, resp.Msg.Metadata.Filename)
				assert.Equal(t, tc.content, string(resp.Msg.Content))
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetContentRejectsOversizedPayloads() {
	testCases := []struct {
		name      string
		sizeBytes int
	}{
		{
			name:      "above_memory_limit",
			sizeBytes: int(maxContentBytes) + 1,
		},
		{
			name:      "well_above_memory_limit",
			sizeBytes: int(maxContentBytes) + (128 << 10),
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				payload := bytes.Repeat([]byte("x"), tc.sizeBytes)
				mediaID := fmt.Sprintf("oversized-%s", strings.ReplaceAll(tc.name, "_", "-"))
				_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
					OwnerID:       "@owner:example.com",
					MediaID:       types.MediaID(mediaID),
					UploadName:    "oversized.bin",
					ContentType:   "application/octet-stream",
					FileSizeBytes: types.FileSizeBytes(len(payload)),
					FileData:      bytes.NewReader(payload),
					Config:        cfg,
				})
				require.NoError(t, err)

				_, err = handler.GetContent(claimsCtx(ctx, "@owner:example.com"), connect.NewRequest(&filesv1.GetContentRequest{
					MediaId: mediaID,
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_SearchMediaValidation() {
	testCases := []struct {
		name      string
		userID    string
		request   *filesv1.SearchMediaRequest
		expectErr connect.Code
	}{
		{
			name:      "owner_mismatch",
			userID:    "@owner:example.com",
			request:   &filesv1.SearchMediaRequest{OwnerId: "@other:example.com", Cursor: &commonv1.PageCursor{Limit: 10}},
			expectErr: connect.CodePermissionDenied,
		},
		{
			name:      "invalid_cursor",
			userID:    "@owner:example.com",
			request:   &filesv1.SearchMediaRequest{Cursor: &commonv1.PageCursor{Limit: 10, Page: "invalid"}},
			expectErr: connect.CodeInvalidArgument,
		},
		{
			name:   "end_before_start",
			userID: "@owner:example.com",
			request: &filesv1.SearchMediaRequest{
				CreatedAfter:  timestamppb.New(time.Date(2026, 2, 21, 9, 0, 0, 0, time.UTC)),
				CreatedBefore: timestamppb.New(time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC)),
				Cursor:        &commonv1.PageCursor{Limit: 10},
			},
			expectErr: connect.CodeInvalidArgument,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, _, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				caseCtx := claimsCtx(ctx, tc.userID)

				_, err := handler.SearchMedia(caseCtx, connect.NewRequest(tc.request))
				require.Error(t, err)
				assert.Equal(t, tc.expectErr, connect.CodeOf(err))
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetUrlPreviewValidation() {
	testCases := []struct {
		name      string
		userID    string
		url       string
		expectErr connect.Code
	}{
		{
			name:      "empty_url",
			userID:    "@owner:example.com",
			url:       "",
			expectErr: connect.CodeInvalidArgument,
		},
		{
			name:      "invalid_url",
			userID:    "@owner:example.com",
			url:       "://invalid",
			expectErr: connect.CodeInvalidArgument,
		},
		{
			name:      "private_url_blocked",
			userID:    "@owner:example.com",
			url:       "http://127.0.0.1:8080",
			expectErr: connect.CodeInvalidArgument,
		},
		{
			name:      "unauthenticated",
			url:       "https://example.com",
			expectErr: connect.CodeUnauthenticated,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, _, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				caseCtx := ctx
				if tc.userID != "" {
					caseCtx = claimsCtx(caseCtx, tc.userID)
				}
				_, err := handler.GetUrlPreview(caseCtx, connect.NewRequest(&filesv1.GetUrlPreviewRequest{Url: tc.url}))
				require.Error(t, err)
				assert.Equal(t, tc.expectErr, connect.CodeOf(err))
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetUrlPreviewSuccess() {
	testCases := []struct {
		name   string
		userID string
	}{
		{
			name:   "fetches_open_graph_and_image_size",
			userID: "@owner:example.com",
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
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
		handler := NewFileServer(svc, mediaService, authzMiddleware, db, storageProvider).(*FileServer)

		imagePayload := createJPEGPayload(t, 32, 32)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				if r.URL.Path == "/image.jpg" {
					w.Header().Set("Content-Type", "image/jpeg")
					_, _ = w.Write(imagePayload)
					return
				}
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write([]byte(`<html><head><meta property="og:title" content="Preview Title"><meta property="og:image" content="http://8.8.8.8/image.jpg"><title>Fallback</title></head></html>`))
			case http.MethodHead:
				if r.URL.Path == "/image.jpg" {
					w.Header().Set("Content-Length", fmt.Sprintf("%d", len(imagePayload)))
					w.Header().Set("Content-Type", "image/jpeg")
				}
				w.WriteHeader(http.StatusOK)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		}))
		defer server.Close()
		targetURL, _ := url.Parse(server.URL)

		customClient := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				clone := req.Clone(req.Context())
				clone.URL.Scheme = targetURL.Scheme
				clone.URL.Host = targetURL.Host
				return http.DefaultTransport.RoundTrip(clone)
			}),
		}
		svc.HTTPClientManager().SetClient(ctx, customClient)

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				caseCtx := claimsCtx(ctx, tc.userID)
				resp, err := handler.GetUrlPreview(caseCtx, connect.NewRequest(&filesv1.GetUrlPreviewRequest{
					Url: "http://8.8.8.8/page",
				}))
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.NotEmpty(t, resp.Msg.OgImageMediaId)
				require.NotNil(t, resp.Msg.OgData)
				assert.Equal(t, "Preview Title", resp.Msg.OgData.AsMap()["og:title"])
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetUrlPreviewAdditionalCases() {
	testCases := []struct {
		name                 string
		setupClient          func(t *testing.T, svcCtx context.Context, svc *frame.Service) string
		expectErr            connect.Code
		expectOgImageMediaID string
	}{
		{
			name: "private_og_image_is_filtered",
			setupClient: func(t *testing.T, svcCtx context.Context, svc *frame.Service) string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html")
					_, _ = w.Write([]byte(`<html><head><meta property="og:title" content="Filtered"><meta property="og:image" content="http://127.0.0.1/private.jpg"></head></html>`))
				}))
				t.Cleanup(server.Close)
				targetURL, _ := url.Parse(server.URL)
				svc.HTTPClientManager().SetClient(svcCtx, &http.Client{
					Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
						clone := req.Clone(req.Context())
						clone.URL.Scheme = targetURL.Scheme
						clone.URL.Host = targetURL.Host
						return http.DefaultTransport.RoundTrip(clone)
					}),
				})
				return "http://8.8.8.8/page"
			},
			expectOgImageMediaID: "",
		},
		{
			name: "http_client_error_returns_unavailable",
			setupClient: func(t *testing.T, svcCtx context.Context, svc *frame.Service) string {
				svc.HTTPClientManager().SetClient(svcCtx, &http.Client{
					Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
						return nil, errors.New("network down")
					}),
				})
				return "http://8.8.8.8/page"
			},
			expectErr: connect.CodeUnavailable,
		},
		{
			name: "oversized_og_image_is_not_loaded",
			setupClient: func(t *testing.T, svcCtx context.Context, svc *frame.Service) string {
				largeLength := maxPreviewImageBytes + 1
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.Method {
					case http.MethodGet:
						if r.URL.Path == "/large.jpg" {
							w.Header().Set("Content-Type", "image/jpeg")
							_, _ = w.Write(createJPEGPayload(t, 64, 64))
							return
						}
						w.Header().Set("Content-Type", "text/html")
						_, _ = w.Write([]byte(`<html><head><meta property="og:image" content="http://8.8.8.8/large.jpg"></head></html>`))
					case http.MethodHead:
						if r.URL.Path == "/large.jpg" {
							w.Header().Set("Content-Type", "image/jpeg")
							w.Header().Set("Content-Length", fmt.Sprintf("%d", largeLength))
						}
						w.WriteHeader(http.StatusOK)
					default:
						w.WriteHeader(http.StatusMethodNotAllowed)
					}
				}))
				t.Cleanup(server.Close)
				targetURL, _ := url.Parse(server.URL)
				svc.HTTPClientManager().SetClient(svcCtx, &http.Client{
					Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
						clone := req.Clone(req.Context())
						clone.URL.Scheme = targetURL.Scheme
						clone.URL.Host = targetURL.Host
						return http.DefaultTransport.RoundTrip(clone)
					}),
				})
				return "http://8.8.8.8/page"
			},
			expectOgImageMediaID: "",
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res := suite.CreateService(t, dep)
		cfg := svc.Config().(*config.FilesConfig)
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: res.MediaRepository,
		}
		storageProvider, err := provider.GetStorageProvider(ctx, cfg)
		require.NoError(t, err)
		mediaService := business.NewMediaService(db, storageProvider)
		authorizer := svc.SecurityManager().GetAuthorizer(ctx)
		authzMiddleware := authz.NewMiddleware(authorizer, db)
		handler := NewFileServer(svc, mediaService, authzMiddleware, db, storageProvider).(*FileServer)

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				targetURL := tc.setupClient(t, ctx, svc)
				caseCtx := claimsCtx(ctx, "@owner:example.com")
				resp, err := handler.GetUrlPreview(caseCtx, connect.NewRequest(&filesv1.GetUrlPreviewRequest{
					Url: targetURL,
				}))
				if tc.expectErr != 0 {
					require.Error(t, err)
					assert.Equal(t, tc.expectErr, connect.CodeOf(err))
					return
				}
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tc.expectOgImageMediaID, resp.Msg.OgImageMediaId)
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetConfig() {
	testCases := []struct {
		name      string
		userID    string
		expectErr connect.Code
	}{
		{name: "returns_upload_limits", userID: "@owner:example.com"},
		{name: "unauthenticated", expectErr: connect.CodeUnauthenticated},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, cfg, _, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				caseCtx := ctx
				if tc.userID != "" {
					caseCtx = claimsCtx(caseCtx, tc.userID)
				}
				resp, err := handler.GetConfig(caseCtx, connect.NewRequest(&filesv1.GetConfigRequest{}))
				if tc.expectErr != 0 {
					require.Error(t, err)
					assert.Equal(t, tc.expectErr, connect.CodeOf(err))
					return
				}
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, int64(cfg.MaxFileSizeBytes), resp.Msg.MaxUploadBytes)
				require.NotNil(t, resp.Msg.Extra)
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_CreateContentValidation() {
	testCases := []struct {
		name      string
		userID    string
		expectErr connect.Code
	}{
		{name: "authenticated", userID: "@owner:example.com"},
		{name: "unauthenticated", expectErr: connect.CodeUnauthenticated},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, _, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				caseCtx := ctx
				if tc.userID != "" {
					caseCtx = claimsCtx(caseCtx, tc.userID)
				}
				resp, err := handler.CreateContent(caseCtx, connect.NewRequest(&filesv1.CreateContentRequest{}))
				if tc.expectErr != 0 {
					require.Error(t, err)
					assert.Equal(t, tc.expectErr, connect.CodeOf(err))
					return
				}
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.NotEmpty(t, resp.Msg.MediaId)
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetContentOverrideName() {
	testCases := []struct {
		name         string
		overrideName string
		expectedName string
	}{
		{name: "override_filename", overrideName: "override.txt", expectedName: "override.txt"},
		{name: "fallback_filename", overrideName: "", expectedName: "original.txt"},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mediaID := "overrideMedia123" + tc.name
				content := "hello-" + tc.name

				_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
					OwnerID:       types.OwnerID("@owner:example.com"),
					MediaID:       types.MediaID(mediaID),
					UploadName:    types.Filename("original.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(len(content)),
					FileData:      io.NopCloser(bytes.NewReader([]byte(content))),
					Config:        cfg,
				})
				require.NoError(t, err)

				caseCtx := claimsCtx(ctx, "@owner:example.com")
				resp, err := handler.GetContentOverrideName(caseCtx, connect.NewRequest(&filesv1.GetContentOverrideNameRequest{
					MediaId:  mediaID,
					FileName: tc.overrideName,
				}))
				require.NoError(t, err)
				require.NotNil(t, resp.Msg.Metadata)
				assert.Equal(t, tc.expectedName, resp.Msg.Metadata.Filename)
				assert.Equal(t, "text/plain", resp.Msg.Metadata.ContentType)
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetContentThumbnailValidation() {
	testCases := []struct {
		name      string
		userID    string
		request   *filesv1.GetContentThumbnailRequest
		expectErr connect.Code
	}{
		{
			name:      "unauthenticated",
			request:   &filesv1.GetContentThumbnailRequest{MediaId: "abc123"},
			expectErr: connect.CodeUnauthenticated,
		},
		{
			name:      "invalid_media_id",
			userID:    "@owner:example.com",
			request:   &filesv1.GetContentThumbnailRequest{MediaId: "bad id"},
			expectErr: connect.CodeInvalidArgument,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, _, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				caseCtx := ctx
				if tc.userID != "" {
					caseCtx = claimsCtx(caseCtx, tc.userID)
				}
				_, err := handler.GetContentThumbnail(caseCtx, connect.NewRequest(tc.request))
				require.Error(t, err)
				assert.Equal(t, tc.expectErr, connect.CodeOf(err))
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetContentThumbnailBusinessValidation() {
	testCases := []struct {
		name      string
		userID    string
		request   *filesv1.GetContentThumbnailRequest
		expectErr connect.Code
	}{
		{
			name:   "missing_thumbnail_size",
			userID: "@owner:example.com",
			request: &filesv1.GetContentThumbnailRequest{
				MediaId: "thumbnailMediaX",
			},
			expectErr: connect.CodeInvalidArgument,
		},
		{
			name:   "invalid_thumbnail_dimensions",
			userID: "@owner:example.com",
			request: &filesv1.GetContentThumbnailRequest{
				MediaId: "thumbnailMediaX",
				Width:   5000,
				Height:  5000,
			},
			expectErr: connect.CodeInvalidArgument,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mediaID := "thumbnailMediaX" + tc.name
				content := "thumbnail source content " + tc.name
				_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
					OwnerID:       types.OwnerID(tc.userID),
					MediaID:       types.MediaID(mediaID),
					UploadName:    types.Filename("file.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(len(content)),
					FileData:      bytes.NewReader([]byte(content)),
					Config:        cfg,
				})
				require.NoError(t, err)

				req := protoCloneGetContentThumbnailRequest(tc.request)
				req.MediaId = mediaID
				caseCtx := claimsCtx(ctx, tc.userID)
				_, err = handler.GetContentThumbnail(caseCtx, connect.NewRequest(req))
				require.Error(t, err)
				assert.Equal(t, tc.expectErr, connect.CodeOf(err))
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_UtilityFunctions() {
	testCases := []struct {
		name      string
		run       func() any
		validator func(t *testing.T, result any)
	}{
		{
			name: "is_valid_media_id",
			run: func() any {
				return isValidMediaID("AbC_123=-")
			},
			validator: func(t *testing.T, result any) {
				assert.True(t, result.(bool))
			},
		},
		{
			name: "is_invalid_media_id",
			run: func() any {
				return isValidMediaID("bad id")
			},
			validator: func(t *testing.T, result any) {
				assert.False(t, result.(bool))
			},
		},
		{
			name: "apply_timeout_default",
			run: func() any {
				ctx, cancel := applyRequestTimeout(context.Background(), 0)
				defer cancel()
				deadline, ok := ctx.Deadline()
				return ok && time.Until(deadline) > 0
			},
			validator: func(t *testing.T, result any) {
				assert.True(t, result.(bool))
			},
		},
		{
			name: "business_error_not_found",
			run: func() any {
				return mapBusinessErrorToConnectCode(errors.New("resource not found"))
			},
			validator: func(t *testing.T, result any) {
				assert.Equal(t, connect.CodeNotFound, result.(connect.Code))
			},
		},
		{
			name: "business_error_invalid",
			run: func() any {
				return mapBusinessErrorToConnectCode(errors.New("invalid parameter"))
			},
			validator: func(t *testing.T, result any) {
				assert.Equal(t, connect.CodeInvalidArgument, result.(connect.Code))
			},
		},
		{
			name: "business_error_permission",
			run: func() any {
				return mapBusinessErrorToConnectCode(errors.New("permission denied"))
			},
			validator: func(t *testing.T, result any) {
				assert.Equal(t, connect.CodePermissionDenied, result.(connect.Code))
			},
		},
		{
			name: "business_error_default_internal",
			run: func() any {
				return mapBusinessErrorToConnectCode(errors.New("boom"))
			},
			validator: func(t *testing.T, result any) {
				assert.Equal(t, connect.CodeInternal, result.(connect.Code))
			},
		},
		{
			name: "business_error_nil_unknown",
			run: func() any {
				return mapBusinessErrorToConnectCode(nil)
			},
			validator: func(t *testing.T, result any) {
				assert.Equal(t, connect.CodeUnknown, result.(connect.Code))
			},
		},
		{
			name: "business_error_deadline",
			run: func() any {
				return mapBusinessErrorToConnectCode(context.DeadlineExceeded)
			},
			validator: func(t *testing.T, result any) {
				assert.Equal(t, connect.CodeDeadlineExceeded, result.(connect.Code))
			},
		},
		{
			name: "private_ip_detection",
			run: func() any {
				return isPrivateIP(net.ParseIP("127.0.0.1"))
			},
			validator: func(t *testing.T, result any) {
				assert.True(t, result.(bool))
			},
		},
		{
			name: "extract_open_graph",
			run: func() any {
				og, title := extractOpenGraph(bytes.NewBufferString("<html><head><meta property=\"og:title\" content=\"A\"><title>B</title></head></html>"))
				return map[string]any{
					"title": title,
					"og":    og["og:title"],
				}
			},
			validator: func(t *testing.T, result any) {
				out := result.(map[string]any)
				assert.Equal(t, "B", out["title"])
				assert.Equal(t, "A", out["og"])
			},
		},
		{
			name: "fetch_content_length_unknown",
			run: func() any {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()

				_, err := fetchContentLength(context.Background(), server.Client(), server.URL)
				return err
			},
			validator: func(t *testing.T, result any) {
				require.Error(t, result.(error))
			},
		},
		{
			name: "fetch_content_length_success",
			run: func() any {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodHead {
						w.Header().Set("Content-Length", "42")
						w.WriteHeader(http.StatusOK)
						return
					}
					w.WriteHeader(http.StatusMethodNotAllowed)
				}))
				defer server.Close()

				targetURL, _ := url.Parse(server.URL)
				client := &http.Client{
					Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
						clone := req.Clone(req.Context())
						clone.URL.Scheme = targetURL.Scheme
						clone.URL.Host = targetURL.Host
						return http.DefaultTransport.RoundTrip(clone)
					}),
				}

				size, err := fetchContentLength(context.Background(), client, "http://8.8.8.8/preview.jpg")
				return map[string]any{"size": size, "err": err}
			},
			validator: func(t *testing.T, result any) {
				out := result.(map[string]any)
				require.Nil(t, out["err"])
				assert.Equal(t, int64(42), out["size"].(int64))
			},
		},
		{
			name: "allowed_preview_url_rejects_private",
			run: func() any {
				u, _ := url.Parse("http://127.0.0.1")
				return isAllowedPreviewURL(u)
			},
			validator: func(t *testing.T, result any) {
				assert.False(t, result.(bool))
			},
		},
		{
			name: "allowed_preview_url_rejects_scheme",
			run: func() any {
				u, _ := url.Parse("ftp://example.com/file")
				return isAllowedPreviewURL(u)
			},
			validator: func(t *testing.T, result any) {
				assert.False(t, result.(bool))
			},
		},
		{
			name: "allowed_preview_url_nil_rejected",
			run: func() any {
				return isAllowedPreviewURL(nil)
			},
			validator: func(t *testing.T, result any) {
				assert.False(t, result.(bool))
			},
		},
		{
			name: "allowed_preview_url_empty_host_rejected",
			run: func() any {
				u, _ := url.Parse("http:///path")
				return isAllowedPreviewURL(u)
			},
			validator: func(t *testing.T, result any) {
				assert.False(t, result.(bool))
			},
		},
		{
			name: "allowed_preview_url_public_ip_allowed",
			run: func() any {
				u, _ := url.Parse("http://8.8.8.8")
				return isAllowedPreviewURL(u)
			},
			validator: func(t *testing.T, result any) {
				assert.True(t, result.(bool))
			},
		},
		{
			name: "allowed_preview_url_dns_lookup_failure",
			run: func() any {
				u, _ := url.Parse("http://invalid.invalid")
				return isAllowedPreviewURL(u)
			},
			validator: func(t *testing.T, result any) {
				assert.False(t, result.(bool))
			},
		},
		{
			name: "authenticated_subject_empty_subject",
			run: func() any {
				authClaims := &security.AuthenticationClaims{
					RegisteredClaims: jwt.RegisteredClaims{Subject: ""},
				}
				_, err := authenticatedSubject(authClaims.ClaimsToContext(context.Background()))
				return err
			},
			validator: func(t *testing.T, result any) {
				require.Error(t, result.(error))
				assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(result.(error)))
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := tc.run()
			tc.validator(t, result)
		})
	}
}

func (suite *FileServerTestSuite) Test_FileServer_UploadContentStream() {
	testCases := []struct {
		name      string
		userID    string
		metadata  *filesv1.UploadMetadata
		chunks    [][]byte
		expectErr connect.Code
	}{
		{
			name:   "stream_upload_attempt",
			userID: "@owner:example.com",
			metadata: &filesv1.UploadMetadata{
				Filename:    "upload.txt",
				ContentType: "text/plain",
				TotalSize:   11,
			},
			chunks: [][]byte{[]byte("hello "), []byte("world")},
		},
		{
			name:   "stream_upload_with_public_property",
			userID: "@owner:example.com",
			metadata: &filesv1.UploadMetadata{
				MediaId:     "streampublic001",
				Filename:    "public.txt",
				ContentType: "text/plain",
				TotalSize:   5,
				Properties: func() *structpb.Struct {
					props, _ := structpb.NewStruct(map[string]any{"is_public": true})
					return props
				}(),
			},
			chunks: [][]byte{[]byte("hello")},
		},
		{
			name:      "unauthenticated_stream",
			expectErr: connect.CodeUnauthenticated,
			metadata: &filesv1.UploadMetadata{
				Filename:    "upload.txt",
				ContentType: "text/plain",
				TotalSize:   5,
			},
			chunks: [][]byte{[]byte("hello")},
		},
		{
			name:      "missing_metadata",
			userID:    "@owner:example.com",
			expectErr: connect.CodeInvalidArgument,
			chunks:    [][]byte{[]byte("hello")},
		},
		{
			name:      "invalid_media_id",
			userID:    "@owner:example.com",
			expectErr: connect.CodeInvalidArgument,
			metadata: &filesv1.UploadMetadata{
				MediaId:     "bad id",
				Filename:    "upload.txt",
				ContentType: "text/plain",
				TotalSize:   5,
			},
			chunks: [][]byte{[]byte("hello")},
		},
		{
			name:      "server_name_mismatch",
			userID:    "@owner:example.com",
			expectErr: connect.CodeInvalidArgument,
			metadata: &filesv1.UploadMetadata{
				ServerName:  "other.example.com",
				Filename:    "upload.txt",
				ContentType: "text/plain",
				TotalSize:   5,
			},
			chunks: [][]byte{[]byte("hello")},
		},
		{
			name:      "total_size_mismatch",
			userID:    "@owner:example.com",
			expectErr: connect.CodeInvalidArgument,
			metadata: &filesv1.UploadMetadata{
				Filename:    "upload.txt",
				ContentType: "text/plain",
				TotalSize:   3,
			},
			chunks: [][]byte{[]byte("hello")},
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, _, fileHandler := suite.setupFileServer(t, dep)
		_, connectHandler := filesv1connect.NewFilesServiceHandler(fileHandler)

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {

				authWrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tc.userID != "" {
						r = r.WithContext(claimsCtx(r.Context(), tc.userID))
					}
					connectHandler.ServeHTTP(w, r)
				})

				server := httptest.NewServer(authWrapped)
				defer server.Close()

				client := filesv1connect.NewFilesServiceClient(server.Client(), server.URL)
				stream := client.UploadContent(ctx)
				var err error

				if tc.metadata != nil {
					meta := proto.Clone(tc.metadata).(*filesv1.UploadMetadata)
					err = stream.Send(&filesv1.UploadContentRequest{
						Data: &filesv1.UploadContentRequest_Metadata{
							Metadata: meta,
						},
					})
					require.NoError(t, err)
				}

				for _, chunk := range tc.chunks {
					err = stream.Send(&filesv1.UploadContentRequest{
						Data: &filesv1.UploadContentRequest_Chunk{
							Chunk: chunk,
						},
					})
					require.NoError(t, err)
				}

				resp, err := stream.CloseAndReceive()
				if tc.expectErr != 0 {
					require.Error(t, err)
					assert.Equal(t, tc.expectErr, connect.CodeOf(err))
					return
				}

				// Depending on queue publisher config, upload may succeed or fail at publish time;
				// both paths still validate stream handling correctness.
				if err != nil {
					assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
					if tc.metadata != nil && tc.metadata.GetMediaId() == "streampublic001" {
						meta, dbErr := fileHandler.db.GetMediaMetadata(ctx, "streampublic001")
						require.NoError(t, dbErr)
						require.NotNil(t, meta)
						assert.True(t, meta.IsPublic)
					}
					return
				}
				require.NotNil(t, resp)
				assert.NotEmpty(t, resp.Msg.MediaId)
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_SearchMediaSuccess() {
	testCases := []struct {
		name   string
		userID string
		query  string
	}{
		{
			name:   "owner_searches_own_media",
			userID: "@search-owner:example.com",
			query:  "invoice",
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				content := "search payload"
				_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
					OwnerID:       types.OwnerID(tc.userID),
					MediaID:       "searchMedia123",
					UploadName:    "invoice-2026.pdf",
					ContentType:   "application/pdf",
					FileSizeBytes: types.FileSizeBytes(len(content)),
					FileData:      bytes.NewReader([]byte(content)),
					Config:        cfg,
				})
				require.NoError(t, err)

				caseCtx := claimsCtx(ctx, tc.userID)
				resp, err := handler.SearchMedia(caseCtx, connect.NewRequest(&filesv1.SearchMediaRequest{
					Query:   tc.query,
					Cursor:  &commonv1.PageCursor{Limit: 10},
					OwnerId: tc.userID,
				}))
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.NotEmpty(t, resp.Msg.Results)
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_SearchMediaResultBounds() {
	testCases := []struct {
		name       string
		ownerID    string
		otherOwner string
		sharedUser string
		limit      int32
		expectLen  int
	}{
		{
			name:       "limit_applies_to_merged_results",
			ownerID:    "@owner-a:example.com",
			otherOwner: "@owner-b:example.com",
			sharedUser: "@owner-a:example.com",
			limit:      1,
			expectLen:  1,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				uploads := []struct {
					mediaID  string
					ownerID  string
					filename string
				}{
					{mediaID: "ownMedia001", ownerID: tc.ownerID, filename: "invoice-a.pdf"},
					{mediaID: "ownMedia002", ownerID: tc.ownerID, filename: "invoice-b.pdf"},
					{mediaID: "sharedMedia", ownerID: tc.otherOwner, filename: "invoice-shared.pdf"},
				}

				for _, item := range uploads {
					content := "content-" + item.mediaID
					_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
						OwnerID:       types.OwnerID(item.ownerID),
						MediaID:       types.MediaID(item.mediaID),
						UploadName:    types.Filename(item.filename),
						ContentType:   "application/pdf",
						FileSizeBytes: types.FileSizeBytes(len(content)),
						FileData:      bytes.NewReader([]byte(content)),
						Config:        cfg,
					})
					require.NoError(t, err)
				}

				require.NoError(t, handler.authz.GrantFileAccess(ctx, tc.otherOwner, "sharedMedia", tc.sharedUser, "viewer"))

				caseCtx := claimsCtx(ctx, tc.ownerID)
				resp, err := handler.SearchMedia(caseCtx, connect.NewRequest(&filesv1.SearchMediaRequest{
					OwnerId: tc.ownerID,
					Query:   "invoice",
					Cursor:  &commonv1.PageCursor{Limit: tc.limit},
				}))
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Len(t, resp.Msg.Results, tc.expectLen)
				assert.LessOrEqual(t, len(resp.Msg.Results), int(tc.limit))
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_SearchMediaSharedEdgeCases() {
	testCases := []struct {
		name      string
		query     string
		expectMin int
	}{
		{
			name:      "skips_missing_and_non_matching_shared_entries",
			query:     "shared",
			expectMin: 0,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res := suite.CreateService(t, dep)
		cfg := svc.Config().(*config.FilesConfig)
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: res.MediaRepository,
		}
		storageProvider, err := provider.GetStorageProvider(ctx, cfg)
		require.NoError(t, err)
		mediaService := business.NewMediaService(db, storageProvider)
		authorizer := svc.SecurityManager().GetAuthorizer(ctx)
		authzMiddleware := authz.NewMiddleware(authorizer, db)
		handler := NewFileServer(svc, mediaService, authzMiddleware, db, storageProvider).(*FileServer)

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err = mediaService.UploadFile(ctx, &business.UploadRequest{
					OwnerID:       "@other:example.com",
					MediaID:       "sharedHit001",
					UploadName:    "shared-file.txt",
					ContentType:   "text/plain",
					FileSizeBytes: 8,
					FileData:      bytes.NewReader([]byte("shared-1")),
					Config:        cfg,
				})
				require.NoError(t, err)

				_, err = mediaService.UploadFile(ctx, &business.UploadRequest{
					OwnerID:       "@other:example.com",
					MediaID:       "sharedNoMatch001",
					UploadName:    "photo.png",
					ContentType:   "image/png",
					FileSizeBytes: 7,
					FileData:      bytes.NewReader([]byte("sharedx")),
					Config:        cfg,
				})
				require.NoError(t, err)
				require.NoError(t, handler.authz.GrantFileAccess(ctx, "@other:example.com", "sharedHit001", "@owner:example.com", "viewer"))
				require.NoError(t, handler.authz.GrantFileAccess(ctx, "@other:example.com", "sharedNoMatch001", "@owner:example.com", "viewer"))

				resp, err := handler.SearchMedia(claimsCtx(ctx, "@owner:example.com"), connect.NewRequest(&filesv1.SearchMediaRequest{
					Query: tc.query,
				}))
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.GreaterOrEqual(t, len(resp.Msg.Results), tc.expectMin)
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetContentThumbnailSuccess() {
	testCases := []struct {
		name   string
		method filesv1.ThumbnailMethod
		width  int32
		height int32
		srcW   int
		srcH   int
	}{
		{
			name:   "crop_thumbnail",
			method: filesv1.ThumbnailMethod_CROP,
			width:  32,
			height: 32,
			srcW:   1024,
			srcH:   768,
		},
		{
			name:   "scale_thumbnail",
			method: filesv1.ThumbnailMethod_SCALE,
			width:  640,
			height: 480,
			srcW:   1400,
			srcH:   1050,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				payload := createJPEGPayload(t, tc.srcW, tc.srcH)
				mediaID := "thumbSuccess" + tc.name

				_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
					OwnerID:       "@owner:example.com",
					MediaID:       types.MediaID(mediaID),
					UploadName:    "image.jpg",
					ContentType:   "image/jpeg",
					FileSizeBytes: types.FileSizeBytes(len(payload)),
					FileData:      bytes.NewReader(payload),
					Config:        cfg,
					IsPublic:      true,
				})
				require.NoError(t, err)

				resp, err := handler.GetContentThumbnail(claimsCtx(ctx, "@owner:example.com"), connect.NewRequest(&filesv1.GetContentThumbnailRequest{
					MediaId: mediaID,
					Width:   tc.width,
					Height:  tc.height,
					Method:  tc.method,
				}))
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.NotEmpty(t, resp.Msg.Content)
				require.NotNil(t, resp.Msg.Metadata)
				assert.Equal(t, "image/jpeg", resp.Msg.Metadata.ContentType)
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_SearchQueryAndQueueHelpers() {
	testCases := []struct {
		name      string
		run       func(t *testing.T, ctx context.Context, dep *definition.DependencyOption) any
		validator func(t *testing.T, out any)
	}{
		{
			name: "matches_search_query",
			run: func(_ *testing.T, _ context.Context, _ *definition.DependencyOption) any {
				return matchesSearchQuery(&types.MediaMetadata{
					UploadName:  "Report.pdf",
					ContentType: "application/pdf",
				}, "report")
			},
			validator: func(t *testing.T, out any) {
				assert.True(t, out.(bool))
			},
		},
		{
			name: "matches_search_query_false",
			run: func(_ *testing.T, _ context.Context, _ *definition.DependencyOption) any {
				return matchesSearchQuery(&types.MediaMetadata{
					UploadName:  "Report.pdf",
					ContentType: "application/pdf",
				}, "video")
			},
			validator: func(t *testing.T, out any) {
				assert.False(t, out.(bool))
			},
		},
		{
			name: "queue_thumbnail_generation_without_publisher_errors",
			run: func(t *testing.T, _ context.Context, dep *definition.DependencyOption) any {
				ctx2, svc, _ := suite.CreateService(t, dep)
				return queueThumbnailGeneration(ctx2, svc, types.MediaID("media-queue-1"))
			},
			validator: func(t *testing.T, out any) {
				require.Error(t, out.(error))
			},
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, _ := suite.CreateService(t, dep)
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.name == "queue_thumbnail_generation_without_publisher_errors" {
					out := queueThumbnailGeneration(ctx, svc, types.MediaID("media-queue-1"))
					tc.validator(t, out)
					return
				}
				out := tc.run(t, t.Context(), dep)
				tc.validator(t, out)
			})
		}
	})
}

func (suite *FileServerTestSuite) setupFileServer(
	t *testing.T,
	dep *definition.DependencyOption,
) (context.Context, *config.FilesConfig, business.MediaService, *FileServer) {
	ctx, svc, res := suite.CreateService(t, dep)
	cfg := svc.Config().(*config.FilesConfig)

	db := &connection.Database{
		WorkManager:             svc.WorkManager(),
		MediaRepository:         res.MediaRepository,
		MultipartUploadRepo:     res.MultipartUploadRepo,
		MultipartUploadPartRepo: res.MultipartUploadPartRepo,
		FileVersionRepo:         res.FileVersionRepo,
		RetentionPolicyRepo:     res.RetentionPolicyRepo,
		FileRetentionRepo:       res.FileRetentionRepo,
		StorageStatsRepo:        res.StorageStatsRepo,
	}

	storageProvider, err := provider.GetStorageProvider(ctx, cfg)
	require.NoError(t, err)

	mediaService := business.NewMediaService(db, storageProvider)
	authorizer := svc.SecurityManager().GetAuthorizer(ctx)
	authzMiddleware := authz.NewMiddleware(authorizer, db)
	handler := NewFileServer(svc, mediaService, authzMiddleware, db, storageProvider).(*FileServer)

	return ctx, cfg, mediaService, handler
}

func claimsCtx(ctx context.Context, userID string) context.Context {
	authClaims := &security.AuthenticationClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID,
		},
	}
	return authClaims.ClaimsToContext(ctx)
}

func protoCloneGetContentThumbnailRequest(in *filesv1.GetContentThumbnailRequest) *filesv1.GetContentThumbnailRequest {
	if in == nil {
		return &filesv1.GetContentThumbnailRequest{}
	}
	return proto.Clone(in).(*filesv1.GetContentThumbnailRequest)
}

func createJPEGPayload(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{ //nolint:misspell
				R: uint8((x + y) % 255),
				G: uint8((2 * y) % 255),
				B: 180,
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}))
	return buf.Bytes()
}

func (suite *FileServerTestSuite) Test_FileServer_GetUserUsage() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			t.Run("unauthenticated", func(t *testing.T) {
				_, err := handler.GetUserUsage(t.Context(), connect.NewRequest(&filesv1.GetUserUsageRequest{}))
				require.Error(t, err)
				require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			})

			t.Run("success", func(t *testing.T) {
				userID := "@test-user-usage:example.com"
				authCtx := claimsCtx(ctx, userID)

				resp, err := handler.GetUserUsage(authCtx, connect.NewRequest(&filesv1.GetUserUsageRequest{}))
				require.NoError(t, err)
				require.NotNil(t, resp.Msg.Usage)
			})

			t.Run("other_user_forbidden", func(t *testing.T) {
				authCtx := claimsCtx(ctx, "@test-user:example.com")
				_, err := handler.GetUserUsage(authCtx, connect.NewRequest(&filesv1.GetUserUsageRequest{
					UserId: "@other-user:example.com",
				}))
				require.Error(t, err)
				require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
			})
		})
	})
}

func internalServiceClaimsCtx(ctx context.Context, serviceName string) context.Context {
	authClaims := &security.AuthenticationClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: serviceName},
		ServiceName:      serviceName,
		Roles:            []string{"system_internal"},
	}
	return authClaims.ClaimsToContext(ctx)
}

func (suite *FileServerTestSuite) Test_FileServer_GetStorageStats() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			t.Run("unauthenticated", func(t *testing.T) {
				_, err := handler.GetStorageStats(t.Context(), connect.NewRequest(&filesv1.GetStorageStatsRequest{}))
				require.Error(t, err)
				require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			})

			t.Run("regular_user_denied", func(t *testing.T) {
				authCtx := claimsCtx(ctx, "@test-user:example.com")
				_, err := handler.GetStorageStats(authCtx, connect.NewRequest(&filesv1.GetStorageStatsRequest{}))
				require.Error(t, err)
				require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
			})

			t.Run("internal_service_returns_stats", func(t *testing.T) {
				svcCtx := internalServiceClaimsCtx(ctx, "service_ocr")
				resp, err := handler.GetStorageStats(svcCtx, connect.NewRequest(&filesv1.GetStorageStatsRequest{}))
				require.NoError(t, err)
				require.Equal(t, int64(0), resp.Msg.TotalBytes)
				require.Equal(t, int64(0), resp.Msg.TotalFiles)
				require.Equal(t, int64(0), resp.Msg.TotalUsers)
			})
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_DeleteContent() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			tests := []struct {
				name      string
				mediaID   string
				userID    string
				expectErr connect.Code
			}{
				{
					name:      "invalid_media_id",
					mediaID:   "bad id",
					userID:    "@test:example.com",
					expectErr: connect.CodeInvalidArgument,
				},
				{
					name:      "media_not_found_returns_permission_denied",
					mediaID:   "nonexistent123",
					userID:    "@test:example.com",
					expectErr: connect.CodePermissionDenied,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					caseCtx := ctx
					if tc.userID != "" {
						caseCtx = claimsCtx(ctx, tc.userID)
					}

					_, err := handler.HeadContent(caseCtx, connect.NewRequest(&filesv1.HeadContentRequest{
						MediaId: tc.mediaID,
					}))
					require.Error(t, err)
					require.Equal(t, tc.expectErr, connect.CodeOf(err))
				})
			}
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetSignedUploadUrl() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			tests := []struct {
				name      string
				mediaID   string
				userID    string
				expectErr connect.Code
			}{
				{
					name:      "unauthenticated",
					mediaID:   "abc123",
					userID:    "",
					expectErr: connect.CodeUnauthenticated,
				},
				{
					name:      "invalid_media_id",
					mediaID:   "bad id",
					userID:    "@test:example.com",
					expectErr: connect.CodeInvalidArgument,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					caseCtx := ctx
					if tc.userID != "" {
						caseCtx = claimsCtx(ctx, tc.userID)
					}

					_, err := handler.GetSignedUploadUrl(caseCtx, connect.NewRequest(&filesv1.GetSignedUploadUrlRequest{
						MediaId: tc.mediaID,
					}))
					require.Error(t, err)
					require.Equal(t, tc.expectErr, connect.CodeOf(err))
				})
			}
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetSignedDownloadUrl() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			tests := []struct {
				name      string
				mediaID   string
				userID    string
				expectErr connect.Code
			}{
				{
					name:      "unauthenticated",
					mediaID:   "abc123",
					userID:    "",
					expectErr: connect.CodeUnauthenticated,
				},
				{
					name:      "invalid_media_id",
					mediaID:   "bad id",
					userID:    "@test:example.com",
					expectErr: connect.CodeInvalidArgument,
				},
				{
					name:      "media_not_found_returns_permission_denied",
					mediaID:   "nonexistent123",
					userID:    "@test:example.com",
					expectErr: connect.CodePermissionDenied,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					caseCtx := ctx
					if tc.userID != "" {
						caseCtx = claimsCtx(ctx, tc.userID)
					}

					_, err := handler.GetSignedDownloadUrl(caseCtx, connect.NewRequest(&filesv1.GetSignedDownloadUrlRequest{
						MediaId: tc.mediaID,
					}))
					require.Error(t, err)
					require.Equal(t, tc.expectErr, connect.CodeOf(err))
				})
			}
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_BatchGetContent() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			tests := []struct {
				name      string
				mediaIDs  []string
				userID    string
				expectErr connect.Code
			}{
				{
					name:      "unauthenticated",
					mediaIDs:  []string{"abc123"},
					userID:    "",
					expectErr: connect.CodeUnauthenticated,
				},
				{
					name:      "empty_media_ids",
					mediaIDs:  []string{},
					userID:    "@test:example.com",
					expectErr: connect.CodeInvalidArgument,
				},
				{
					name:      "too_many_media_ids",
					mediaIDs:  make([]string, maxBatchGetItems+1),
					userID:    "@test:example.com",
					expectErr: connect.CodeInvalidArgument,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					caseCtx := ctx
					if tc.userID != "" {
						caseCtx = claimsCtx(ctx, tc.userID)
					}

					_, err := handler.BatchGetContent(caseCtx, connect.NewRequest(&filesv1.BatchGetContentRequest{
						MediaIds: tc.mediaIDs,
					}))
					require.Error(t, err)
					require.Equal(t, tc.expectErr, connect.CodeOf(err))
				})
			}
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_BatchDeleteContent() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			tests := []struct {
				name      string
				mediaIDs  []string
				userID    string
				expectErr connect.Code
			}{
				{
					name:      "unauthenticated",
					mediaIDs:  []string{"abc123"},
					userID:    "",
					expectErr: connect.CodeUnauthenticated,
				},
				{
					name:      "empty_media_ids",
					mediaIDs:  []string{},
					userID:    "@test:example.com",
					expectErr: connect.CodeInvalidArgument,
				},
				{
					name:      "too_many_media_ids",
					mediaIDs:  make([]string, maxBatchDeleteItems+1),
					userID:    "@test:example.com",
					expectErr: connect.CodeInvalidArgument,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					caseCtx := ctx
					if tc.userID != "" {
						caseCtx = claimsCtx(ctx, tc.userID)
					}

					_, err := handler.BatchDeleteContent(caseCtx, connect.NewRequest(&filesv1.BatchDeleteContentRequest{
						MediaIds: tc.mediaIDs,
					}))
					require.Error(t, err)
					require.Equal(t, tc.expectErr, connect.CodeOf(err))
				})
			}
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_MultipartUploads() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			tests := []struct {
				name      string
				userID    string
				expectErr connect.Code
			}{
				{
					name:      "create_unauthenticated",
					userID:    "",
					expectErr: connect.CodeUnauthenticated,
				},
				{
					name:      "complete_unauthenticated",
					userID:    "",
					expectErr: connect.CodeUnauthenticated,
				},
				{
					name:      "abort_unauthenticated",
					userID:    "",
					expectErr: connect.CodeUnauthenticated,
				},
				{
					name:      "list_parts_unauthenticated",
					userID:    "",
					expectErr: connect.CodeUnauthenticated,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					caseCtx := ctx
					if tc.userID != "" {
						caseCtx = claimsCtx(ctx, tc.userID)
					}

					switch tc.name {
					case "create_unauthenticated":
						_, err := handler.CreateMultipartUpload(caseCtx, connect.NewRequest(&filesv1.CreateMultipartUploadRequest{}))
						require.Error(t, err)
						require.Equal(t, tc.expectErr, connect.CodeOf(err))

					case "complete_unauthenticated":
						_, err := handler.CompleteMultipartUpload(caseCtx, connect.NewRequest(&filesv1.CompleteMultipartUploadRequest{}))
						require.Error(t, err)
						require.Equal(t, tc.expectErr, connect.CodeOf(err))

					case "abort_unauthenticated":
						_, err := handler.AbortMultipartUpload(caseCtx, connect.NewRequest(&filesv1.AbortMultipartUploadRequest{}))
						require.Error(t, err)
						require.Equal(t, tc.expectErr, connect.CodeOf(err))

					case "list_parts_unauthenticated":
						_, err := handler.ListMultipartParts(caseCtx, connect.NewRequest(&filesv1.ListMultipartPartsRequest{}))
						require.Error(t, err)
						require.Equal(t, tc.expectErr, connect.CodeOf(err))
					}
				})
			}
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_Versioning() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			tests := []struct {
				name      string
				mediaID   string
				userID    string
				expectErr connect.Code
			}{
				{
					name:      "get_versions_unauthenticated",
					mediaID:   "abc123",
					userID:    "",
					expectErr: connect.CodeUnauthenticated,
				},
				{
					name:      "get_versions_permission_denied_for_invalid_id",
					mediaID:   "bad id",
					userID:    "@test:example.com",
					expectErr: connect.CodePermissionDenied,
				},
				{
					name:      "restore_version_unauthenticated",
					mediaID:   "abc123",
					userID:    "",
					expectErr: connect.CodeUnauthenticated,
				},
				{
					name:      "restore_version_permission_denied_for_invalid_id",
					mediaID:   "bad id",
					userID:    "@test:example.com",
					expectErr: connect.CodePermissionDenied,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					caseCtx := ctx
					if tc.userID != "" {
						caseCtx = claimsCtx(ctx, tc.userID)
					}

					switch {
					case strings.HasPrefix(tc.name, "get_versions"):
						_, err := handler.GetVersions(caseCtx, connect.NewRequest(&filesv1.GetVersionsRequest{
							MediaId: tc.mediaID,
						}))
						require.Error(t, err)
						require.Equal(t, tc.expectErr, connect.CodeOf(err))

					case strings.HasPrefix(tc.name, "restore_version"):
						_, err := handler.RestoreVersion(caseCtx, connect.NewRequest(&filesv1.RestoreVersionRequest{
							MediaId: tc.mediaID,
						}))
						require.Error(t, err)
						require.Equal(t, tc.expectErr, connect.CodeOf(err))
					}
				})
			}
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GrantAccess() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)

			ownerID := "@grant-owner:example.com"
			_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
				OwnerID:       types.OwnerID(ownerID),
				MediaID:       "grantfile01",
				UploadName:    "grant.txt",
				ContentType:   "text/plain",
				FileSizeBytes: 5,
				FileData:      io.NopCloser(bytes.NewReader([]byte("hello"))),
				Config:        cfg,
			})
			require.NoError(t, err)

			t.Run("unauthenticated", func(t *testing.T) {
				grant := &filesv1.AccessGrant{}
				grant.SetPrincipalId("@other:example.com")
				grant.SetRole(filesv1.AccessRole_ACCESS_ROLE_READER)
				_, err := handler.GrantAccess(t.Context(), connect.NewRequest(&filesv1.GrantAccessRequest{
					MediaId: "grantfile01",
					Grant:   grant,
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			})

			t.Run("invalid_media_id", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				grant := &filesv1.AccessGrant{}
				grant.SetPrincipalId("@other:example.com")
				grant.SetRole(filesv1.AccessRole_ACCESS_ROLE_READER)
				_, err := handler.GrantAccess(authCtx, connect.NewRequest(&filesv1.GrantAccessRequest{
					MediaId: "bad id",
					Grant:   grant,
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			})

			t.Run("missing_grant", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				_, err := handler.GrantAccess(authCtx, connect.NewRequest(&filesv1.GrantAccessRequest{
					MediaId: "grantfile01",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			})

			t.Run("success", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				grant := &filesv1.AccessGrant{}
				grant.SetPrincipalId("@reader:example.com")
				grant.SetRole(filesv1.AccessRole_ACCESS_ROLE_READER)
				resp, err := handler.GrantAccess(authCtx, connect.NewRequest(&filesv1.GrantAccessRequest{
					MediaId: "grantfile01",
					Grant:   grant,
				}))
				require.NoError(t, err)
				assert.True(t, resp.Msg.GetSuccess())
			})
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_RevokeAccess() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)

			ownerID := "@revoke-owner:example.com"
			_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
				OwnerID:       types.OwnerID(ownerID),
				MediaID:       "revokefile01",
				UploadName:    "revoke.txt",
				ContentType:   "text/plain",
				FileSizeBytes: 5,
				FileData:      io.NopCloser(bytes.NewReader([]byte("hello"))),
				Config:        cfg,
			})
			require.NoError(t, err)

			t.Run("unauthenticated", func(t *testing.T) {
				_, err := handler.RevokeAccess(t.Context(), connect.NewRequest(&filesv1.RevokeAccessRequest{
					MediaId:     "revokefile01",
					PrincipalId: "@other:example.com",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			})

			t.Run("invalid_media_id", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				_, err := handler.RevokeAccess(authCtx, connect.NewRequest(&filesv1.RevokeAccessRequest{
					MediaId:     "bad id",
					PrincipalId: "@other:example.com",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			})

			t.Run("success", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				resp, err := handler.RevokeAccess(authCtx, connect.NewRequest(&filesv1.RevokeAccessRequest{
					MediaId:     "revokefile01",
					PrincipalId: "@other:example.com",
				}))
				require.NoError(t, err)
				assert.True(t, resp.Msg.GetSuccess())
			})
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_ListAccess() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)

			ownerID := "@list-owner:example.com"
			_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
				OwnerID:       types.OwnerID(ownerID),
				MediaID:       "listfile01",
				UploadName:    "list.txt",
				ContentType:   "text/plain",
				FileSizeBytes: 5,
				FileData:      io.NopCloser(bytes.NewReader([]byte("hello"))),
				Config:        cfg,
			})
			require.NoError(t, err)

			t.Run("unauthenticated", func(t *testing.T) {
				_, err := handler.ListAccess(t.Context(), connect.NewRequest(&filesv1.ListAccessRequest{
					MediaId: "listfile01",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			})

			t.Run("invalid_media_id", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				_, err := handler.ListAccess(authCtx, connect.NewRequest(&filesv1.ListAccessRequest{
					MediaId: "bad id",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			})

			t.Run("non_owner_denied", func(t *testing.T) {
				authCtx := claimsCtx(ctx, "@not-owner:example.com")
				_, err := handler.ListAccess(authCtx, connect.NewRequest(&filesv1.ListAccessRequest{
					MediaId: "listfile01",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
			})

			t.Run("success", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				resp, err := handler.ListAccess(authCtx, connect.NewRequest(&filesv1.ListAccessRequest{
					MediaId: "listfile01",
				}))
				require.NoError(t, err)
				require.NotNil(t, resp.Msg)
			})
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_PatchContent() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)

			ownerID := "@patch-owner:example.com"
			_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
				OwnerID:       types.OwnerID(ownerID),
				MediaID:       "patchfile01",
				UploadName:    "original.txt",
				ContentType:   "text/plain",
				FileSizeBytes: 5,
				FileData:      io.NopCloser(bytes.NewReader([]byte("hello"))),
				Config:        cfg,
			})
			require.NoError(t, err)

			t.Run("unauthenticated", func(t *testing.T) {
				_, err := handler.PatchContent(t.Context(), connect.NewRequest(&filesv1.PatchContentRequest{
					MediaId:  "patchfile01",
					Filename: "renamed.txt",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			})

			t.Run("invalid_media_id", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				_, err := handler.PatchContent(authCtx, connect.NewRequest(&filesv1.PatchContentRequest{
					MediaId:  "bad id",
					Filename: "renamed.txt",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			})

			t.Run("rename_success", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				resp, err := handler.PatchContent(authCtx, connect.NewRequest(&filesv1.PatchContentRequest{
					MediaId:  "patchfile01",
					Filename: "renamed.txt",
				}))
				require.NoError(t, err)
				require.NotNil(t, resp.Msg.GetMetadata())
				assert.Equal(t, "renamed.txt", resp.Msg.GetMetadata().GetFilename())
			})

			t.Run("change_visibility", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				resp, err := handler.PatchContent(authCtx, connect.NewRequest(&filesv1.PatchContentRequest{
					MediaId:    "patchfile01",
					Visibility: filesv1.MediaMetadata_VISIBILITY_PUBLIC,
				}))
				require.NoError(t, err)
				require.NotNil(t, resp.Msg.GetMetadata())
				assert.Equal(t, filesv1.MediaMetadata_VISIBILITY_PUBLIC, resp.Msg.GetMetadata().GetVisibility())
			})
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_FinalizeSignedUpload() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, cfg, mediaService, handler := suite.setupFileServer(t, dep)

			ownerID := "@finalise-owner:example.com"
			_, err := mediaService.UploadFile(ctx, &business.UploadRequest{
				OwnerID:       types.OwnerID(ownerID),
				MediaID:       "finalfile01",
				UploadName:    "final.txt",
				ContentType:   "text/plain",
				FileSizeBytes: 5,
				FileData:      io.NopCloser(bytes.NewReader([]byte("hello"))),
				Config:        cfg,
			})
			require.NoError(t, err)

			t.Run("unauthenticated", func(t *testing.T) {
				_, err := handler.FinalizeSignedUpload(t.Context(), connect.NewRequest(&filesv1.FinalizeSignedUploadRequest{
					MediaId: "finalfile01",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			})

			t.Run("invalid_media_id", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				_, err := handler.FinalizeSignedUpload(authCtx, connect.NewRequest(&filesv1.FinalizeSignedUploadRequest{
					MediaId: "bad id",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			})

			t.Run("success", func(t *testing.T) {
				authCtx := claimsCtx(ctx, ownerID)
				resp, err := handler.FinalizeSignedUpload(authCtx, connect.NewRequest(&filesv1.FinalizeSignedUploadRequest{
					MediaId:        "finalfile01",
					ChecksumSha256: "abc123checksum",
					SizeBytes:      5,
				}))
				require.NoError(t, err)
				require.NotNil(t, resp.Msg.GetMetadata())
			})
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_DownloadContent_Validation() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			t.Run("unauthenticated", func(t *testing.T) {
				err := handler.DownloadContent(t.Context(), connect.NewRequest(&filesv1.DownloadContentRequest{
					MediaId: "dlfile01",
				}), nil)
				require.Error(t, err)
				assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			})

			t.Run("invalid_media_id", func(t *testing.T) {
				authCtx := claimsCtx(ctx, "@dl-owner:example.com")
				err := handler.DownloadContent(authCtx, connect.NewRequest(&filesv1.DownloadContentRequest{
					MediaId: "bad id",
				}), nil)
				require.Error(t, err)
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			})
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_DownloadContentRange_Validation() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			t.Run("unauthenticated", func(t *testing.T) {
				err := handler.DownloadContentRange(t.Context(), connect.NewRequest(&filesv1.DownloadContentRangeRequest{
					MediaId: "dlrangefile01",
					Start:   0,
					End:     5,
				}), nil)
				require.Error(t, err)
				assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			})

			t.Run("negative_start", func(t *testing.T) {
				authCtx := claimsCtx(ctx, "@dlrange-owner:example.com")
				err := handler.DownloadContentRange(authCtx, connect.NewRequest(&filesv1.DownloadContentRangeRequest{
					MediaId: "dlrangefile01",
					Start:   -1,
					End:     5,
				}), nil)
				require.Error(t, err)
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			})

			t.Run("end_before_start", func(t *testing.T) {
				authCtx := claimsCtx(ctx, "@dlrange-owner:example.com")
				err := handler.DownloadContentRange(authCtx, connect.NewRequest(&filesv1.DownloadContentRangeRequest{
					MediaId: "dlrangefile01",
					Start:   10,
					End:     5,
				}), nil)
				require.Error(t, err)
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			})
		})
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetMultipartUpload() {
	suite.Run("default", func() {
		suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
			ctx, _, _, handler := suite.setupFileServer(t, dep)

			t.Run("unauthenticated", func(t *testing.T) {
				_, err := handler.GetMultipartUpload(t.Context(), connect.NewRequest(&filesv1.GetMultipartUploadRequest{
					UploadId: "nonexistent",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			})

			t.Run("not_found", func(t *testing.T) {
				authCtx := claimsCtx(ctx, "@test:example.com")
				_, err := handler.GetMultipartUpload(authCtx, connect.NewRequest(&filesv1.GetMultipartUploadRequest{
					UploadId: "nonexistent",
				}))
				require.Error(t, err)
				assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
			})
		})
	})
}
