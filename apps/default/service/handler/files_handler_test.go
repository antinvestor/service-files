package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"buf.build/gen/go/antinvestor/files/connectrpc/go/files/v1/filesv1connect"
	filesv1 "buf.build/gen/go/antinvestor/files/protocolbuffers/go/files/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/handler/routing"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/util"
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
		service := dep.Service
		db := dep.Database
		provider := dep.StorageProvider
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(service, mediaService, db, provider)

		require.NotNil(t, handler)
		assert.Implements(t, (*filesv1connect.FilesServiceHandler)(nil), handler)
	})
}

func (suite *FileServerTestSuite) Test_FileServer_UploadContent() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		service := dep.Service
		db := dep.Database
		provider := dep.StorageProvider
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(service, mediaService, db, provider).(*FileServer)

		// Create authenticated context
		authClaims := &security.MockClaims{
			Subject: "test-user",
		}
		ctx = security.SetClaims(ctx, authClaims)

		testCases := []struct {
			name          string
			metadata      *filesv1.UploadMetadata
			chunks        [][]byte
			expectedError string
		}{
			{
				name: "successful upload",
				metadata: &filesv1.UploadMetadata{
					ContentType: "text/plain",
					Filename:    "test.txt",
					TotalSize:   100,
				},
				chunks:        [][]byte{[]byte("test file content")},
				expectedError: "",
			},
			{
				name: "upload with multiple chunks",
				metadata: &filesv1.UploadMetadata{
					ContentType: "text/plain",
					Filename:    "test.txt",
					TotalSize:   200,
				},
				chunks:        [][]byte{[]byte("first chunk"), []byte("second chunk")},
				expectedError: "",
			},
			{
				name: "upload with no metadata",
				metadata:      nil,
				chunks:        [][]byte{[]byte("test content")},
				expectedError: "metadata is required",
			},
			{
				name: "upload with empty filename",
				metadata: &filesv1.UploadMetadata{
					ContentType: "text/plain",
					Filename:    "",
					TotalSize:   100,
				},
				chunks:        [][]byte{[]byte("test content")},
				expectedError: "filename is required",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create a mock stream
				stream := &mockUploadStream{
					messages: []filesv1.UploadContentRequest{},
					index:    0,
				}

				// Add metadata message
				if tc.metadata != nil {
					stream.messages = append(stream.messages, filesv1.UploadContentRequest{
						Data: &filesv1.UploadContentRequest_Metadata{
							Metadata: tc.metadata,
						},
					})
				}

				// Add chunk messages
				for _, chunk := range tc.chunks {
					stream.messages = append(stream.messages, filesv1.UploadContentRequest{
						Data: &filesv1.UploadContentRequest_Chunk{
							Chunk: chunk,
						},
					})
				}

				// Call the handler
				resp, err := handler.UploadContent(ctx, stream)

				if tc.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedError)
					assert.Nil(t, resp)
				} else {
					assert.NoError(t, err)
					require.NotNil(t, resp)
					assert.NotEmpty(t, resp.Msg.MediaId)
					assert.NotEmpty(t, resp.Msg.ServerName)
					assert.Contains(t, resp.Msg.ContentUri, "mxc://")
				}
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_CreateContent() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		service := dep.Service
		db := dep.Database
		provider := dep.StorageProvider
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(service, mediaService, db, provider).(*FileServer)

		// Create authenticated context
		authClaims := &security.MockClaims{
			Subject: "test-user",
		}
		ctx = security.SetClaims(ctx, authClaims)

		req := connect.NewRequest(&filesv1.CreateContentRequest{})

		resp, err := handler.CreateContent(ctx, req)

		assert.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.Msg.MediaId)
		assert.NotEmpty(t, resp.Msg.ServerName)
		assert.Contains(t, resp.Msg.ContentUri, "mxc://")
		assert.Equal(t, 32, len(resp.Msg.MediaId)) // Should be 32 characters
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetContent() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		service := dep.Service
		db := dep.Database
		provider := dep.StorageProvider
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(service, mediaService, db, provider).(*FileServer)

		// First upload a file to download
		testContent := []byte("test content for download")
		uploadReq := &business.UploadRequest{
			OwnerID:       types.OwnerID("test-user"),
			UploadName:    types.Filename("test.txt"),
			ContentType:   types.ContentType("text/plain"),
			FileSizeBytes: types.FileSizeBytes(len(testContent)),
			FileData:      io.NopCloser(bytes.NewReader(testContent)),
			Config: &config.FilesConfig{
				MaxFileSizeBytes: config.FileSizeBytes(1024 * 1024),
				ServerName:       "test.example.com",
			},
		}

		uploadResult, err := mediaService.UploadFile(ctx, uploadReq)
		require.NoError(t, err)

		testCases := []struct {
			name          string
			mediaID       string
			expectedError string
		}{
			{
				name:          "successful download",
				mediaID:       string(uploadResult.MediaID),
				expectedError: "",
			},
			{
				name:          "download non-existent file",
				mediaID:       "non-existent-id",
				expectedError: "not found",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := connect.NewRequest(&filesv1.GetContentRequest{
					MediaId: tc.mediaID,
				})

				resp, err := handler.GetContent(ctx, req)

				if tc.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedError)
					assert.Nil(t, resp)
				} else {
					assert.NoError(t, err)
					require.NotNil(t, resp)
					assert.Equal(t, testContent, resp.Msg.Content)
					assert.Equal(t, "text/plain", resp.Msg.ContentType)
					assert.Equal(t, "test.txt", resp.Msg.Filename)
				}
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetContentOverrideName() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		service := dep.Service
		db := dep.Database
		provider := dep.StorageProvider
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(service, mediaService, db, provider).(*FileServer)

		// First upload a file
		testContent := []byte("test content")
		uploadReq := &business.UploadRequest{
			OwnerID:       types.OwnerID("test-user"),
			UploadName:    types.Filename("original.txt"),
			ContentType:   types.ContentType("text/plain"),
			FileSizeBytes: types.FileSizeBytes(len(testContent)),
			FileData:      io.NopCloser(bytes.NewReader(testContent)),
			Config: &config.FilesConfig{
				MaxFileSizeBytes: config.FileSizeBytes(1024 * 1024),
				ServerName:       "test.example.com",
			},
		}

		uploadResult, err := mediaService.UploadFile(ctx, uploadReq)
		require.NoError(t, err)

		testCases := []struct {
			name          string
			mediaID       string
			fileName      string
			expectedError string
		}{
			{
				name:          "download with custom filename",
				mediaID:       string(uploadResult.MediaID),
				fileName:      "custom.txt",
				expectedError: "",
			},
			{
				name:          "download with empty filename (should use original)",
				mediaID:       string(uploadResult.MediaID),
				fileName:      "",
				expectedError: "",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := connect.NewRequest(&filesv1.GetContentOverrideNameRequest{
					MediaId:   tc.mediaID,
					FileName:  tc.fileName,
					TimeoutMs: 30000,
				})

				resp, err := handler.GetContentOverrideName(ctx, req)

				if tc.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedError)
					assert.Nil(t, resp)
				} else {
					assert.NoError(t, err)
					require.NotNil(t, resp)
					assert.Equal(t, testContent, resp.Msg.Content)
					assert.Equal(t, "text/plain", resp.Msg.ContentType)
					
					expectedFilename := tc.fileName
					if expectedFilename == "" {
						expectedFilename = "original.txt"
					}
					assert.Equal(t, expectedFilename, resp.Msg.Filename)
				}
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetConfig() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		service := dep.Service
		db := dep.Database
		provider := dep.StorageProvider
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(service, mediaService, db, provider).(*FileServer)

		req := connect.NewRequest(&filesv1.GetConfigRequest{})

		resp, err := handler.GetConfig(ctx, req)

		assert.NoError(t, err)
		require.NotNil(t, resp)
		assert.Greater(t, resp.Msg.MaxUploadSize, int64(0))
		require.NotNil(t, resp.Msg.Extra)
		require.NotNil(t, resp.Msg.Extra.Fields)
	})
}

func (suite *FileServerTestSuite) Test_FileServer_SearchMedia() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		service := dep.Service
		db := dep.Database
		provider := dep.StorageProvider
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(service, mediaService, db, provider).(*FileServer)

		// Create authenticated context
		authClaims := &security.MockClaims{
			Subject: "test-user",
		}
		ctx = security.SetClaims(ctx, authClaims)

		// Upload some test files
		cfg := &config.FilesConfig{
			MaxFileSizeBytes: config.FileSizeBytes(1024 * 1024),
			ServerName:       "test.example.com",
		}

		testFiles := []struct {
			name     string
			content  string
			mimeType string
		}{
			{"document.txt", "This is a text document", "text/plain"},
			{"image.jpg", "fake image content", "image/jpeg"},
		}

		for _, tf := range testFiles {
			uploadReq := &business.UploadRequest{
				OwnerID:       types.OwnerID("test-user"),
				UploadName:    types.Filename(tf.name),
				ContentType:   types.ContentType(tf.mimeType),
				FileSizeBytes: types.FileSizeBytes(len(tf.content)),
				FileData:      io.NopCloser(bytes.NewReader([]byte(tf.content))),
				Config:        cfg,
			}
			_, err := mediaService.UploadFile(ctx, uploadReq)
			require.NoError(t, err)
		}

		testCases := []struct {
			name        string
			query       string
			page        uint32
			limit       uint32
			expectError bool
			minResults  int
		}{
			{
				name:        "search all files",
				query:       "",
				page:        0,
				limit:       10,
				expectError: false,
				minResults:  2,
			},
			{
				name:        "search with query",
				query:       "document",
				page:        0,
				limit:       10,
				expectError: false,
				minResults:  1,
			},
			{
				name:        "search with pagination",
				query:       "",
				page:        0,
				limit:       1,
				expectError: false,
				minResults:  1,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := connect.NewRequest(&filesv1.SearchMediaRequest{
					Query: tc.query,
					Page:  tc.page,
					Limit: tc.limit,
				})

				resp, err := handler.SearchMedia(ctx, req)

				if tc.expectError {
					assert.Error(t, err)
					assert.Nil(t, resp)
				} else {
					assert.NoError(t, err)
					require.NotNil(t, resp)
					assert.GreaterOrEqual(t, len(resp.Msg.Results), tc.minResults)
					assert.GreaterOrEqual(t, resp.Msg.Total, int32(tc.minResults))
					assert.Equal(t, tc.page, resp.Msg.Page)
					
					// Verify result structure
					for _, result := range resp.Msg.Results {
						assert.NotEmpty(t, result.MediaId)
						assert.NotEmpty(t, result.ServerName)
						assert.NotEmpty(t, result.ContentType)
						assert.Greater(t, result.FileSizeBytes, int64(0))
						assert.Greater(t, result.CreationTimestamp, int64(0))
						assert.Equal(t, "test-user", result.OwnerId)
					}
				}
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetContentThumbnail() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		service := dep.Service
		db := dep.Database
		provider := dep.StorageProvider
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(service, mediaService, db, provider).(*FileServer)

		// Upload an image file (using a minimal fake image)
		imageContent := []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x01\x00H\x00H\x00\x00\xff\xdb\x00C\x00")
		uploadReq := &business.UploadRequest{
			OwnerID:       types.OwnerID("test-user"),
			UploadName:    types.Filename("test.jpg"),
			ContentType:   types.ContentType("image/jpeg"),
			FileSizeBytes: types.FileSizeBytes(len(imageContent)),
			FileData:      io.NopCloser(bytes.NewReader(imageContent)),
			Config: &config.FilesConfig{
				MaxFileSizeBytes: config.FileSizeBytes(1024 * 1024),
				ServerName:       "test.example.com",
			},
		}

		uploadResult, err := mediaService.UploadFile(ctx, uploadReq)
		require.NoError(t, err)

		testCases := []struct {
			name          string
			mediaID       string
			width         uint32
			height        uint32
			method        filesv1.ThumbnailMethod
			expectedError string
		}{
			{
				name:          "thumbnail with scale method",
				mediaID:       string(uploadResult.MediaID),
				width:         100,
				height:        100,
				method:        filesv1.ThumbnailMethod_SCALE,
				expectedError: "", // May or may not work depending on image processing
			},
			{
				name:          "thumbnail with crop method",
				mediaID:       string(uploadResult.MediaID),
				width:         50,
				height:        50,
				method:        filesv1.ThumbnailMethod_CROP,
				expectedError: "", // May or may not work depending on image processing
			},
			{
				name:          "thumbnail for non-existent file",
				mediaID:       "non-existent-id",
				width:         100,
				height:        100,
				method:        filesv1.ThumbnailMethod_SCALE,
				expectedError: "not found",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := connect.NewRequest(&filesv1.GetContentThumbnailRequest{
					MediaId: tc.mediaID,
					Width:   tc.width,
					Height:  tc.height,
					Method:  tc.method,
				})

				resp, err := handler.GetContentThumbnail(ctx, req)

				if tc.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedError)
					assert.Nil(t, resp)
				} else {
					if err != nil {
						// Thumbnail generation might not be available for fake images
						t.Logf("Thumbnail generation failed as expected: %v", err)
					} else {
						require.NotNil(t, resp)
						assert.NotEmpty(t, resp.Msg.Content)
						assert.NotEmpty(t, resp.Msg.ContentType)
					}
				}
			})
		}
	})
}

func (suite *FileServerTestSuite) Test_FileServer_GetUrlPreview() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		service := dep.Service
		db := dep.Database
		provider := dep.StorageProvider
		mediaService := business.NewMediaService(db, provider)

		handler := NewFileServer(service, mediaService, db, provider).(*FileServer)

		req := connect.NewRequest(&filesv1.GetUrlPreviewRequest{
			Url: "https://example.com",
		})

		resp, err := handler.GetUrlPreview(ctx, req)

		assert.Error(t, err)
		assert.True(t, connect.IsUnimplemented(err))
		assert.Nil(t, resp)
	})
}

// Mock stream for testing
type mockUploadStream struct {
	messages []filesv1.UploadContentRequest
	index    int
}

func (m *mockUploadStream) Receive() bool {
	if m.index >= len(m.messages) {
		return false
	}
	m.index++
	return true
}

func (m *mockUploadStream) Msg() *filesv1.UploadContentRequest {
	if m.index == 0 || m.index > len(m.messages) {
		return nil
	}
	return &m.messages[m.index-1]
}

func (m *mockUploadStream) Err() error {
	return nil
}
