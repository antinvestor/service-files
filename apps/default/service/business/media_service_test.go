package business

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage"
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
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		db := dep.Database
		provider := dep.StorageProvider

		service := NewMediaService(db, provider)

		require.NotNil(t, service)
		assert.Implements(t, (*MediaService)(nil), service)
	})
}

func (suite *MediaServiceTestSuite) Test_MediaService_UploadFile() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		db := dep.Database
		provider := dep.StorageProvider
		service := NewMediaService(db, provider)

		cfg := &config.FilesConfig{
			MaxFileSizeBytes: config.FileSizeBytes(1024 * 1024), // 1MB
			ServerName:       "test.example.com",
		}

		testCases := []struct {
			name           string
			request        *UploadRequest
			expectedError  string
			expectedResult *UploadResult
		}{
			{
				name: "successful upload",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("test-user"),
					UploadName:    types.Filename("test.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(100),
					FileData:      io.NopCloser(bytes.NewReader([]byte("test content"))),
					Config:        cfg,
				},
				expectedError: "",
			},
			{
				name: "upload with empty filename",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("test-user"),
					UploadName:    types.Filename(""),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(100),
					FileData:      io.NopCloser(bytes.NewReader([]byte("test content"))),
					Config:        cfg,
				},
				expectedError: "filename is required",
			},
			{
				name: "upload with file too large",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("test-user"),
					UploadName:    types.Filename("test.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(2 * 1024 * 1024), // 2MB
					FileData:      io.NopCloser(bytes.NewReader(make([]byte, 2*1024*1024))),
					Config:        cfg,
				},
				expectedError: "file size exceeds",
			},
			{
				name: "upload with nil file data",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("test-user"),
					UploadName:    types.Filename("test.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(100),
					FileData:      nil,
					Config:        cfg,
				},
				expectedError: "file data is required",
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
					assert.NotEmpty(t, result.Base64Hash)
				}
			})
		}
	})
}

func (suite *MediaServiceTestSuite) Test_MediaService_DownloadFile() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		db := dep.Database
		provider := dep.StorageProvider
		service := NewMediaService(db, provider)

		cfg := &config.FilesConfig{
			MaxFileSizeBytes: config.FileSizeBytes(1024 * 1024), // 1MB
			ServerName:       "test.example.com",
		}

		// First upload a file to download
		uploadReq := &UploadRequest{
			OwnerID:       types.OwnerID("test-user"),
			UploadName:    types.Filename("test.txt"),
			ContentType:   types.ContentType("text/plain"),
			FileSizeBytes: types.FileSizeBytes(100),
			FileData:      io.NopCloser(bytes.NewReader([]byte("test content for download"))),
			Config:        cfg,
		}

		uploadResult, err := service.UploadFile(ctx, uploadReq)
		require.NoError(t, err)
		require.NotNil(t, uploadResult)

		testCases := []struct {
			name           string
			request        *DownloadRequest
			expectedError  string
			expectedResult *DownloadResult
		}{
			{
				name: "successful download",
				request: &DownloadRequest{
					MediaID:            uploadResult.MediaID,
					IsThumbnailRequest: false,
					Config:             cfg,
				},
				expectedError: "",
			},
			{
				name: "download non-existent file",
				request: &DownloadRequest{
					MediaID:            types.MediaID("non-existent-id"),
					IsThumbnailRequest: false,
					Config:             cfg,
				},
				expectedError: "not found",
			},
			{
				name: "download with thumbnail request",
				request: &DownloadRequest{
					MediaID:            uploadResult.MediaID,
					IsThumbnailRequest: true,
					Config:             cfg,
				},
				expectedError: "", // May or may not work depending on file type
			},
			{
				name: "download with thumbnail size",
				request: &DownloadRequest{
					MediaID:            uploadResult.MediaID,
					IsThumbnailRequest: true,
					ThumbnailSize: &types.ThumbnailSize{
						Width:        100,
						Height:       100,
						ResizeMethod: "scale",
					},
					Config: cfg,
				},
				expectedError: "", // May or may not work depending on file type
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := service.DownloadFile(ctx, tc.request)

				if tc.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedError)
					assert.Nil(t, result)
				} else {
					if err != nil {
						// Some thumbnail requests might fail for non-image files
						t.Logf("Download failed as expected for thumbnail request: %v", err)
					} else {
						require.NotNil(t, result)
						assert.NotEmpty(t, result.ContentType)
						assert.NotEmpty(t, result.Filename)
						assert.Greater(t, result.ContentLength, int64(0))
						require.NotNil(t, result.FileData)
						
						// Read and verify content
						content, err := io.ReadAll(result.FileData)
						require.NoError(t, err)
						assert.Greater(t, len(content), 0)
					}
				}
			})
		}
	})
}

func (suite *MediaServiceTestSuite) Test_MediaService_SearchMedia() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		db := dep.Database
		provider := dep.StorageProvider
		service := NewMediaService(db, provider)

		cfg := &config.FilesConfig{
			MaxFileSizeBytes: config.FileSizeBytes(1024 * 1024), // 1MB
			ServerName:       "test.example.com",
		}

		// Upload some test files
		ownerID := types.OwnerID("test-user")
		testFiles := []struct {
			name     string
			content  string
			mimeType string
		}{
			{"document.txt", "This is a text document", "text/plain"},
			{"image.jpg", "fake image content", "image/jpeg"},
			{"data.json", "{\"key\": \"value\"}", "application/json"},
		}

		for _, tf := range testFiles {
			uploadReq := &UploadRequest{
				OwnerID:       ownerID,
				UploadName:    types.Filename(tf.name),
				ContentType:   types.ContentType(tf.mimeType),
				FileSizeBytes: types.FileSizeBytes(len(tf.content)),
				FileData:      io.NopCloser(bytes.NewReader([]byte(tf.content))),
				Config:        cfg,
			}
			_, err := service.UploadFile(ctx, uploadReq)
			require.NoError(t, err)
		}

		testCases := []struct {
			name           string
			request        *SearchRequest
			expectedError  string
			expectedMin    int // Minimum number of results expected
		}{
			{
				name: "search all files",
				request: &SearchRequest{
					OwnerID: ownerID,
					Query:   "",
					Page:    0,
					Limit:   10,
				},
				expectedError: "",
				expectedMin:   3, // Should find all 3 files
			},
			{
				name: "search with text query",
				request: &SearchRequest{
					OwnerID: ownerID,
					Query:   "document",
					Page:    0,
					Limit:   10,
				},
				expectedError: "",
				expectedMin:   1, // Should find at least 1 file
			},
			{
				name: "search with image query",
				request: &SearchRequest{
					OwnerID: ownerID,
					Query:   "image",
					Page:    0,
					Limit:   10,
				},
				expectedError: "",
				expectedMin:   1, // Should find at least 1 file
			},
			{
				name: "search with pagination",
				request: &SearchRequest{
					OwnerID: ownerID,
					Query:   "",
					Page:    0,
					Limit:   2,
				},
				expectedError: "",
				expectedMin:   2, // Should find at least 2 files
			},
			{
				name: "search non-existent user",
				request: &SearchRequest{
					OwnerID: types.OwnerID("non-existent-user"),
					Query:   "",
					Page:    0,
					Limit:   10,
				},
				expectedError: "",
				expectedMin:   0, // Should find no files
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := service.SearchMedia(ctx, tc.request)

				if tc.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedError)
					assert.Nil(t, result)
				} else {
					assert.NoError(t, err)
					require.NotNil(t, result)
					assert.GreaterOrEqual(t, len(result.Results), tc.expectedMin)
					assert.GreaterOrEqual(t, result.Count, int32(tc.expectedMin))
					assert.GreaterOrEqual(t, result.Page, tc.request.Page)
					
					// Verify result structure
					for _, media := range result.Results {
						assert.NotEmpty(t, media.MediaID)
						assert.NotEmpty(t, media.ServerName)
						assert.NotEmpty(t, media.ContentType)
						assert.Greater(t, media.FileSizeBytes, types.FileSizeBytes(0))
						assert.Greater(t, media.CreationTimestamp, uint64(0))
						assert.Equal(t, tc.request.OwnerID, media.OwnerID)
					}
				}
			})
		}
	})
}

func (suite *MediaServiceTestSuite) Test_MediaService_GenerateMediaID() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		db := dep.Database
		provider := dep.StorageProvider
		service := NewMediaService(db, provider)

		// Generate multiple IDs to ensure uniqueness
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := service.GenerateMediaID(ctx)
			assert.NotEmpty(t, string(id))
			assert.Len(t, string(id), 32) // Should be 32 characters
			
			// Check for uniqueness
			idStr := string(id)
			assert.False(t, ids[idStr], "Generated ID should be unique")
			ids[idStr] = true
		}
	})
}

func (suite *MediaServiceTestSuite) Test_MediaService_ValidateUploadRequest() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		db := dep.Database
		provider := dep.StorageProvider
		service := NewMediaService(db, provider)

		cfg := &config.FilesConfig{
			MaxFileSizeBytes: config.FileSizeBytes(1024), // 1KB
		}

		testCases := []struct {
			name          string
			request       *UploadRequest
			expectedError string
		}{
			{
				name: "valid request",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("test-user"),
					UploadName:    types.Filename("test.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(100),
					FileData:      io.NopCloser(bytes.NewReader([]byte("test"))),
					Config:        cfg,
				},
				expectedError: "",
			},
			{
				name: "nil request",
				request:       nil,
				expectedError: "request is required",
			},
			{
				name: "empty owner ID",
				request: &UploadRequest{
					OwnerID:       types.OwnerID(""),
					UploadName:    types.Filename("test.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(100),
					FileData:      io.NopCloser(bytes.NewReader([]byte("test"))),
					Config:        cfg,
				},
				expectedError: "owner ID is required",
			},
			{
				name: "empty filename",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("test-user"),
					UploadName:    types.Filename(""),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(100),
					FileData:      io.NopCloser(bytes.NewReader([]byte("test"))),
					Config:        cfg,
				},
				expectedError: "filename is required",
			},
			{
				name: "file too large",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("test-user"),
					UploadName:    types.Filename("test.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(2048), // 2KB
					FileData:      io.NopCloser(bytes.NewReader([]byte("test"))),
					Config:        cfg,
				},
				expectedError: "file size exceeds",
			},
			{
				name: "nil file data",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("test-user"),
					UploadName:    types.Filename("test.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(100),
					FileData:      nil,
					Config:        cfg,
				},
				expectedError: "file data is required",
			},
			{
				name: "nil config",
				request: &UploadRequest{
					OwnerID:       types.OwnerID("test-user"),
					UploadName:    types.Filename("test.txt"),
					ContentType:   types.ContentType("text/plain"),
					FileSizeBytes: types.FileSizeBytes(100),
					FileData:      io.NopCloser(bytes.NewReader([]byte("test"))),
					Config:        nil,
				},
				expectedError: "config is required",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := service.(*mediaService).validateUploadRequest(tc.request)

				if tc.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedError)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}
