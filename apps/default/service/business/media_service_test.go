package business

import (
	"bytes"
	"io"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/frametests/definition"
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
		ctx, svc, res := suite.CreateService(t, dep)

		// Create database instance
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: res.MediaRepository,
		}

		// Create storage provider
		cfg := &config.FilesConfig{
			MaxFileSizeBytes: config.FileSizeBytes(1024 * 1024),
			ServerName:       "test.example.com",
		}
		provider, err := provider.GetStorageProvider(ctx, cfg)
		require.NoError(t, err)

		service := NewMediaService(db, provider)

		require.NotNil(t, service)
		assert.Implements(t, (*MediaService)(nil), service)
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
			MaxFileSizeBytes: config.FileSizeBytes(1024 * 1024), // 1MB
			ServerName:       "test.example.com",
		}
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
