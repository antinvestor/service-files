package business

import (
	"bytes"
	"context"
	"image"
	color "image/color" //nolint:misspell
	"image/jpeg"
	"io"
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MediaServiceTestSuite struct {
	tests.BaseTestSuite
}

func TestMediaServiceTestSuite(t *testing.T) {
	suite.Run(t, new(MediaServiceTestSuite))
}

func (suite *MediaServiceTestSuite) Test_NewMediaService() {
	testCases := []struct {
		name string
	}{
		{name: "creates_media_service"},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
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
				provider, err := provider.GetStorageProvider(ctx, cfg)
				require.NoError(t, err)

				service := NewMediaService(db, provider)

				require.NotNil(t, service)
				assert.Implements(t, (*MediaService)(nil), service)
			})
		}
	})
}

func (suite *MediaServiceTestSuite) Test_MediaService_UploadFile() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res := suite.CreateService(t, dep)

		// Create database instance
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: res.MediaRepository,
		}

		// Create storage provider
		cfg := &config.FilesConfig{
			MaxFileSizeBytes:           config.FileSizeBytes(1024 * 1024), // 1MB
			ServerName:                 "test.example.com",
			EnvStorageEncryptionPhrase: "0123456789abcdef0123456789abcdef",
			BasePath:                   config.Path(t.TempDir()),
		}
		require.NoError(t, cfg.Normalise())
		provider, err := provider.GetStorageProvider(ctx, cfg)
		require.NoError(t, err)

		service := NewMediaService(db, provider)

		testCases := []struct {
			name          string
			request       *UploadRequest
			expectedError string
		}{
			{
				name: "successful upload",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("@test-user:example.com"),
					UploadName:    types.Filename("test.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(100),
					FileData:      io.NopCloser(bytes.NewReader([]byte("test content"))),
					Config:        cfg,
				},
				expectedError: "",
			},
			{
				name: "upload with file too large",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("@test-user2:example.com"),
					UploadName:    types.Filename("large.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(2 * 1024 * 1024), // 2MB
					FileData:      io.NopCloser(bytes.NewReader(make([]byte, 2*1024*1024))),
					Config:        cfg,
				},
				expectedError: "HTTP Content-Length is greater than the maximum allowed upload size",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := service.UploadFile(ctx, tc.request)

				if tc.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedError)
					assert.Nil(t, result)
				} else {
					assert.NoError(t, err)
					require.NotNil(t, result)
					assert.NotEmpty(t, result.MediaID)
					assert.Equal(t, cfg.ServerName, result.ServerName)
					assert.Contains(t, result.ContentURI, "mxc://")
				}
			})
		}
	})
}

func (suite *MediaServiceTestSuite) Test_MediaService_ValidationAndSearch() {
	testCases := []struct {
		name        string
		run         func(t *testing.T, service MediaService, cfg *config.FilesConfig, ctxContext testContext) error
		expectedErr string
	}{
		{
			name: "download_invalid_media_id",
			run: func(_ *testing.T, service MediaService, cfg *config.FilesConfig, _ testContext) error {
				_, err := service.DownloadFile(contextBackground(), &DownloadRequest{
					MediaID:            "bad id",
					IsThumbnailRequest: false,
					Config:             cfg,
				})
				return err
			},
			expectedErr: "invalid parameter",
		},
		{
			name: "search_invalid_limit",
			run: func(_ *testing.T, service MediaService, _ *config.FilesConfig, ctxContext testContext) error {
				_, err := service.SearchMedia(ctxContext.ctx, &SearchRequest{
					OwnerID: "@owner:example.com",
					Page:    0,
					Limit:   0,
				})
				return err
			},
			expectedErr: "limit must be > 0 and <= 1000",
		},
		{
			name: "search_with_parent_and_pagination",
			run: func(t *testing.T, service MediaService, cfg *config.FilesConfig, ctxContext testContext) error {
				for i := 0; i < 3; i++ {
					content := "hello-" + string(rune('0'+i))
					_, err := service.UploadFile(ctxContext.ctx, &UploadRequest{
						OwnerID:       "@owner:example.com",
						MediaID:       types.MediaID("childMedia00" + string(rune('A'+i))),
						UploadName:    "file.txt",
						ContentType:   "text/plain",
						FileSizeBytes: types.FileSizeBytes(len(content)),
						FileData:      io.NopCloser(bytes.NewReader([]byte(content))),
						Config:        cfg,
					})
					require.NoError(t, err)
				}

				now := time.Now().UTC()
				res, err := service.SearchMedia(ctxContext.ctx, &SearchRequest{
					OwnerID:   "@owner:example.com",
					Query:     "file",
					Page:      0,
					Limit:     2,
					StartDate: timePtr(now.Add(-24 * time.Hour)),
					EndDate:   timePtr(now.Add(24 * time.Hour)),
				})
				require.NoError(t, err)
				assert.Len(t, res.Results, 2)
				assert.True(t, res.HasMore)
				return nil
			},
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, svc, res := suite.CreateService(t, dep)
				db := &connection.Database{
					WorkManager:     svc.WorkManager(),
					MediaRepository: res.MediaRepository,
				}
				cfg := svc.Config().(*config.FilesConfig)
				provider, err := provider.GetStorageProvider(ctx, cfg)
				require.NoError(t, err)
				service := NewMediaService(db, provider)

				err = tc.run(t, service, cfg, testContext{ctx: ctx})
				if tc.expectedErr != "" {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedErr)
					return
				}
				require.NoError(t, err)
			})
		}
	})
}

func (suite *MediaServiceTestSuite) Test_MediaService_DownloadAndThumbnail() {
	testCases := []struct {
		name    string
		runTest func(t *testing.T, service MediaService, cfg *config.FilesConfig, ctx context.Context)
	}{
		{
			name: "download_with_override_filename",
			runTest: func(t *testing.T, service MediaService, cfg *config.FilesConfig, ctx context.Context) {
				content := "download-content"
				_, err := service.UploadFile(ctx, &UploadRequest{
					OwnerID:       "@owner:example.com",
					MediaID:       "downloadMedia01",
					UploadName:    "orig.txt",
					ContentType:   "text/plain",
					FileSizeBytes: types.FileSizeBytes(len(content)),
					FileData:      bytes.NewReader([]byte(content)),
					Config:        cfg,
				})
				require.NoError(t, err)

				res, err := service.DownloadFile(ctx, &DownloadRequest{
					MediaID:          "downloadMedia01",
					DownloadFilename: "override.txt",
					Config:           cfg,
				})
				require.NoError(t, err)
				defer res.FileData.Close()

				body, err := io.ReadAll(res.FileData)
				require.NoError(t, err)
				assert.Equal(t, "override.txt", res.Filename)
				assert.Equal(t, "text/plain", res.ContentType)
				assert.Equal(t, content, string(body))
			},
		},
		{
			name: "thumbnail_request_validation_paths",
			runTest: func(t *testing.T, service MediaService, cfg *config.FilesConfig, ctx context.Context) {
				_, err := service.DownloadFile(ctx, &DownloadRequest{
					MediaID:            "downloadMedia01",
					IsThumbnailRequest: true,
					ThumbnailSize: &types.ThumbnailSize{
						Width:        -1,
						Height:       -1,
						ResizeMethod: types.Scale,
					},
					Config: cfg,
				})
				require.Error(t, err)
				assert.Contains(t, err.Error(), "width and height must be > 0")
			},
		},
		{
			name: "dynamic_thumbnail_generation",
			runTest: func(t *testing.T, service MediaService, cfg *config.FilesConfig, ctx context.Context) {
				cfg.DynamicThumbnails = true
				cfg.ThumbnailSizes = []config.ThumbnailSize{
					{Width: 32, Height: 32, ResizeMethod: "scale"},
				}

				imgBuf := new(bytes.Buffer)
				img := image.NewRGBA(image.Rect(0, 0, 128, 128))
				for y := 0; y < 128; y++ {
					for x := 0; x < 128; x++ {
						img.Set(x, y, color.RGBA{R: 200, G: 20, B: 20, A: 255}) //nolint:misspell
					}
				}
				require.NoError(t, jpeg.Encode(imgBuf, img, nil))

				_, err := service.UploadFile(ctx, &UploadRequest{
					OwnerID:       "@owner:example.com",
					MediaID:       "imageMedia01",
					UploadName:    "img.jpg",
					ContentType:   "image/jpeg",
					FileSizeBytes: types.FileSizeBytes(imgBuf.Len()),
					FileData:      bytes.NewReader(imgBuf.Bytes()),
					Config:        cfg,
					IsPublic:      true,
				})
				require.NoError(t, err)

				res, err := service.DownloadFile(ctx, &DownloadRequest{
					MediaID:            "imageMedia01",
					IsThumbnailRequest: true,
					ThumbnailSize: &types.ThumbnailSize{
						Width:        32,
						Height:       32,
						ResizeMethod: types.Scale,
					},
					Config: cfg,
				})
				require.NoError(t, err)
				defer res.FileData.Close()

				b, err := io.ReadAll(res.FileData)
				require.NoError(t, err)
				assert.NotEmpty(t, b)
			},
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, svc, res := suite.CreateService(t, dep)
				db := &connection.Database{
					WorkManager:     svc.WorkManager(),
					MediaRepository: res.MediaRepository,
				}
				cfg := svc.Config().(*config.FilesConfig)
				provider, err := provider.GetStorageProvider(ctx, cfg)
				require.NoError(t, err)
				service := NewMediaService(db, provider)

				tc.runTest(t, service, cfg, ctx)
			})
		}
	})
}

func (suite *MediaServiceTestSuite) Test_MediaService_HelperFunctions() {
	testCases := []struct {
		name string
		run  func(t *testing.T, service MediaService)
	}{
		{
			name: "generate_media_id",
			run: func(t *testing.T, service MediaService) {
				id := service.(*mediaService).GenerateMediaID(t.Context())
				require.NotEmpty(t, id)
			},
		},
		{
			name: "thumbnail_size_equal",
			run: func(t *testing.T, _ MediaService) {
				assert.True(t, thumbnailSizeEqual(
					types.ThumbnailSize{Width: 10, Height: 10, ResizeMethod: "scale"},
					types.ThumbnailSize{Width: 10, Height: 10, ResizeMethod: "scale"},
				))
				assert.False(t, thumbnailSizeEqual(
					types.ThumbnailSize{Width: 10, Height: 11, ResizeMethod: "scale"},
					types.ThumbnailSize{Width: 10, Height: 10, ResizeMethod: "scale"},
				))
			},
		},
		{
			name: "download_filename_fallback_to_media_id",
			run: func(t *testing.T, service MediaService) {
				got := service.(*mediaService).getDownloadFilename(&types.MediaMetadata{
					MediaID:    "media-id-fallback",
					UploadName: "",
				}, "")
				assert.Equal(t, "media-id-fallback", got)
			},
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, svc, res := suite.CreateService(t, dep)
				db := &connection.Database{
					WorkManager:     svc.WorkManager(),
					MediaRepository: res.MediaRepository,
				}
				cfg := svc.Config().(*config.FilesConfig)
				provider, err := provider.GetStorageProvider(ctx, cfg)
				require.NoError(t, err)
				service := NewMediaService(db, provider)
				tc.run(t, service)
			})
		}
	})
}

func (suite *MediaServiceTestSuite) Test_MediaService_ValidateDownloadRequestBranches() {
	testCases := []struct {
		name      string
		req       *DownloadRequest
		expectErr string
	}{
		{
			name: "unsupported_resize_method",
			req: &DownloadRequest{
				MediaID:            "media123",
				IsThumbnailRequest: true,
				ThumbnailSize: &types.ThumbnailSize{
					Width:        32,
					Height:       32,
					ResizeMethod: "fit",
				},
				Config: &config.FilesConfig{MaxThumbnailDimension: 2048},
			},
			expectErr: "unsupported resize method",
		},
		{
			name: "uses_default_max_dimension",
			req: &DownloadRequest{
				MediaID:            "media123",
				IsThumbnailRequest: true,
				ThumbnailSize: &types.ThumbnailSize{
					Width:        4096,
					Height:       10,
					ResizeMethod: types.Scale,
				},
				Config: &config.FilesConfig{MaxThumbnailDimension: 0},
			},
			expectErr: "width and height must be <= 2048",
		},
		{
			name: "valid_thumbnail_request",
			req: &DownloadRequest{
				MediaID:            "media123",
				IsThumbnailRequest: true,
				ThumbnailSize: &types.ThumbnailSize{
					Width:        128,
					Height:       128,
					ResizeMethod: types.Scale,
				},
				Config: &config.FilesConfig{MaxThumbnailDimension: 2048},
			},
		},
	}

	service := &mediaService{}
	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			err := service.validateDownloadRequest(tc.req)
			if tc.expectErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func (suite *MediaServiceTestSuite) Test_MediaService_ValidateUploadRequest() {
	testCases := []struct {
		name      string
		request   *UploadRequest
		expectErr string
	}{
		{
			name: "rejects_size_over_limit",
			request: &UploadRequest{
				OwnerID:       "@owner:example.com",
				UploadName:    "file.txt",
				FileSizeBytes: 20,
				Config:        &config.FilesConfig{MaxFileSizeBytes: 10},
			},
			expectErr: "HTTP Content-Length is greater than the maximum allowed upload size",
		},
		{
			name: "rejects_tilde_prefix_filename",
			request: &UploadRequest{
				OwnerID:       "@owner:example.com",
				UploadName:    "~file.txt",
				FileSizeBytes: 10,
				Config:        &config.FilesConfig{MaxFileSizeBytes: 100},
			},
			expectErr: "File name must not begin with '~'",
		},
		{
			name: "rejects_path_separators",
			request: &UploadRequest{
				OwnerID:       "@owner:example.com",
				UploadName:    "a/b.txt",
				FileSizeBytes: 10,
				Config:        &config.FilesConfig{MaxFileSizeBytes: 100},
			},
			expectErr: "File name must not contain path separators",
		},
		{
			name: "rejects_invalid_media_id",
			request: &UploadRequest{
				OwnerID:       "@owner:example.com",
				MediaID:       "bad id",
				UploadName:    "file.txt",
				FileSizeBytes: 10,
				Config:        &config.FilesConfig{MaxFileSizeBytes: 100},
			},
			expectErr: "mediaId must be a non-empty string",
		},
		{
			name: "accepts_valid_request",
			request: &UploadRequest{
				OwnerID:       "@owner:example.com",
				MediaID:       "valid_media_id",
				UploadName:    "file.txt",
				FileSizeBytes: 10,
				Config:        &config.FilesConfig{MaxFileSizeBytes: 100},
			},
		},
	}

	service := &mediaService{}
	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			err := service.validateUploadRequest(tc.request)
			if tc.expectErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func (suite *MediaServiceTestSuite) Test_MediaService_UploadDuplicateAndConflict() {
	testCases := []struct {
		name      string
		run       func(t *testing.T, service MediaService, cfg *config.FilesConfig, ctx context.Context)
		expectErr string
	}{
		{
			name: "duplicate_hash_reuses_existing_media",
			run: func(t *testing.T, service MediaService, cfg *config.FilesConfig, ctx context.Context) {
				first, err := service.UploadFile(ctx, &UploadRequest{
					OwnerID:       "@owner:example.com",
					UploadName:    "first.txt",
					ContentType:   "text/plain",
					FileSizeBytes: 5,
					FileData:      bytes.NewReader([]byte("hello")),
					Config:        cfg,
				})
				require.NoError(t, err)

				second, err := service.UploadFile(ctx, &UploadRequest{
					OwnerID:       "@owner:example.com",
					UploadName:    "second.txt",
					ContentType:   "text/plain",
					FileSizeBytes: 5,
					FileData:      bytes.NewReader([]byte("hello")),
					Config:        cfg,
				})
				require.NoError(t, err)
				assert.Equal(t, first.MediaID, second.MediaID)
			},
		},
		{
			name: "media_id_conflict_returns_error",
			run: func(t *testing.T, service MediaService, cfg *config.FilesConfig, ctx context.Context) {
				mediaID := types.MediaID("conflict-" + util.RandomAlphaNumericString(12))
				firstContent := "first-" + util.RandomAlphaNumericString(16)
				secondContent := "second-" + util.RandomAlphaNumericString(16)
				_, err := service.UploadFile(ctx, &UploadRequest{
					OwnerID:       "@owner:example.com",
					MediaID:       mediaID,
					UploadName:    "first.txt",
					ContentType:   "text/plain",
					FileSizeBytes: types.FileSizeBytes(len(firstContent)),
					FileData:      bytes.NewReader([]byte(firstContent)),
					Config:        cfg,
				})
				require.NoError(t, err)

				_, err = service.UploadFile(ctx, &UploadRequest{
					OwnerID:       "@owner:example.com",
					MediaID:       mediaID,
					UploadName:    "second.txt",
					ContentType:   "text/plain",
					FileSizeBytes: types.FileSizeBytes(len(secondContent)),
					FileData:      bytes.NewReader([]byte(secondContent)),
					Config:        cfg,
				})
				require.Error(t, err)
				assert.Contains(t, err.Error(), "media already exists")
			},
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, svc, res := suite.CreateService(t, dep)
				db := &connection.Database{
					WorkManager:     svc.WorkManager(),
					MediaRepository: res.MediaRepository,
				}
				cfg := svc.Config().(*config.FilesConfig)
				storageProvider, err := provider.GetStorageProvider(ctx, cfg)
				require.NoError(t, err)
				service := NewMediaService(db, storageProvider)
				tc.run(t, service, cfg, ctx)
			})
		}
	})
}

type testContext struct {
	ctx context.Context
}

func timePtr(v time.Time) *time.Time {
	return &v
}

func contextBackground() context.Context {
	return context.Background()
}
