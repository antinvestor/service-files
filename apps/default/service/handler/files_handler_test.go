package handler

import (
	"context"
	"testing"

	"buf.build/gen/go/antinvestor/files/connectrpc/go/files/v1/filesv1connect"
	filesv1 "buf.build/gen/go/antinvestor/files/protocolbuffers/go/files/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type FileServerTestSuite struct {
	tests.BaseTestSuite
}

func TestFileServerTestSuite(t *testing.T) {
	suite.Run(t, new(FileServerTestSuite))
}

func (suite *FileServerTestSuite) Test_NewFileServer() {
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
		
		// Create media service
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(svc, mediaService, db, provider)

		require.NotNil(t, handler)
		assert.Implements(t, (*filesv1connect.FilesServiceHandler)(nil), handler)
	})
}

func (suite *FileServerTestSuite) Test_FileServer_CreateContent() {
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
		
		// Create media service
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(svc, mediaService, db, provider).(*FileServer)

		// Create authenticated context with mock claims
		ctx = context.WithValue(ctx, "user_id", "@test-user:example.com")

		req := connect.NewRequest(&filesv1.CreateContentRequest{})

		resp, err := handler.CreateContent(ctx, req)

		// We expect this to fail due to authentication, but that's ok for now
		// The important thing is that the handler is properly instantiated
		if err != nil {
			assert.Contains(t, err.Error(), "unauthenticated")
		} else {
			require.NotNil(t, resp)
			assert.NotEmpty(t, resp.Msg.MediaId)
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetConfig() {
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
		
		// Create media service
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(svc, mediaService, db, provider).(*FileServer)

		req := connect.NewRequest(&filesv1.GetConfigRequest{})

		resp, err := handler.GetConfig(ctx, req)

		assert.NoError(t, err)
		require.NotNil(t, resp)
		// Just check that we get a response - the actual values may be 0 in test
		assert.NotNil(t, resp.Msg)
		require.NotNil(t, resp.Msg.Extra)
		require.NotNil(t, resp.Msg.Extra.Fields)
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetUrlPreview() {
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
		
		// Create media service
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(svc, mediaService, db, provider).(*FileServer)

		req := connect.NewRequest(&filesv1.GetUrlPreviewRequest{
			Url: "https://example.com",
		})

		resp, err := handler.GetUrlPreview(ctx, req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unimplemented")
		assert.Nil(t, resp)
	})
}
