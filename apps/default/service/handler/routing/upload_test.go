package routing

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UploadTestSuite struct {
	tests.BaseTestSuite
}

func TestUploadTestSuite(t *testing.T) {
	suite.Run(t, new(UploadTestSuite))
}

func (suite *UploadTestSuite) Test_Upload() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		
		// Create test dependencies
		db := dep.Database
		provider := dep.StorageProvider
		service := dep.Service
		mediaService := business.NewMediaService(db, provider)

		testCases := []struct {
			name           string
			contentType    string
			filename       string
			content        []byte
			expectedStatus int
		}{
			{
				name:           "upload text file",
				contentType:    "text/plain",
				filename:       "test.txt",
				content:        []byte("this is test content"),
				expectedStatus: http.StatusOK,
			},
			{
				name:           "upload image file",
				contentType:    "image/jpeg",
				filename:       "test.jpg",
				content:        []byte("\xff\xd8\xff\xe0\x00\x10JFIF"), // JPEG header
				expectedStatus: http.StatusOK,
			},
			{
				name:           "upload without content type",
				contentType:    "",
				filename:       "test.bin",
				content:        []byte("binary content"),
				expectedStatus: http.StatusOK,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create multipart form
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, err := writer.CreateFormFile("file", tc.filename)
				require.NoError(t, err)
				_, err = part.Write(tc.content)
				require.NoError(t, err)
				err = writer.Close()
				require.NoError(t, err)

				// Create request
				req := httptest.NewRequest("POST", "/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				if tc.contentType != "" {
					req.Header.Set("Content-Type", tc.contentType)
				}
				
				// Set up authenticated context
				ctx = util.SetAuthClaims(ctx, "test-user")
				req = req.WithContext(ctx)

				// Call upload function
				resp := Upload(req, service, db, provider, mediaService)

				// Verify response
				require.NotNil(t, resp)
				assert.Equal(t, tc.expectedStatus, resp.Code)

				if resp.Code == http.StatusOK {
					respData := resp.JSON.(map[string]interface{})
					assert.NotEmpty(t, respData["media_id"])
					assert.NotEmpty(t, respData["server_name"])
					assert.NotEmpty(t, respData["content_uri"])
				}
			})
		}
	})
}

func (suite *UploadTestSuite) Test_ParseAndValidateRequest() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		cfg := &config.FilesConfig{
			MaxFileSizeBytes: config.FileSizeBytes(1024 * 1024), // 1MB
		}
		ownerID := types.OwnerID("test-user")

		testCases := []struct {
			name           string
			requestBody    string
			contentType    string
			expectedError  string
			expectedUpload *uploadRequest
		}{
			{
				name:          "empty request body",
				requestBody:   "",
				contentType:   "multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW",
				expectedError: "unable to parse request body",
			},
			{
				name:          "file too large",
				requestBody:   strings.Repeat("x", 2*1024*1024), // 2MB
				contentType:   "multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW",
				expectedError: "file size exceeds limit",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest("POST", "/upload", strings.NewReader(tc.requestBody))
				req.Header.Set("Content-Type", tc.contentType)

				uploadReq, errResp := parseAndValidateRequest(req, cfg, ownerID)

				if tc.expectedError != "" {
					assert.Nil(t, uploadReq)
					assert.NotNil(t, errResp)
					assert.Contains(t, errResp.JSON.(map[string]interface{})["error"], tc.expectedError)
				} else {
					assert.NotNil(t, uploadReq)
					assert.Nil(t, errResp)
					if tc.expectedUpload != nil {
						assert.Equal(t, tc.expectedUpload.UploadName, uploadReq.UploadName)
						assert.Equal(t, tc.expectedUpload.ContentType, uploadReq.ContentType)
					}
				}
			})
		}
	})
}

func (suite *UploadTestSuite) Test_UploadRequest_Validate() {
	cfg := &config.FilesConfig{
		MaxFileSizeBytes: config.FileSizeBytes(1024), // 1KB
	}

	testCases := []struct {
		name          string
		uploadReq     *uploadRequest
		expectedError string
	}{
		{
			name: "valid request",
			uploadReq: &uploadRequest{
				MediaMetadata: types.MediaMetadata{
					UploadName:    "test.txt",
					ContentType:   "text/plain",
					FileSizeBytes: types.FileSizeBytes(100),
				},
			},
			expectedError: "",
		},
		{
			name: "file too large",
			uploadReq: &uploadRequest{
				MediaMetadata: types.MediaMetadata{
					UploadName:    "test.txt",
					ContentType:   "text/plain",
					FileSizeBytes: types.FileSizeBytes(2048),
				},
			},
			expectedError: "file size exceeds limit",
		},
		{
			name: "empty upload name",
			uploadReq: &uploadRequest{
				MediaMetadata: types.MediaMetadata{
					UploadName:    "",
					ContentType:   "text/plain",
					FileSizeBytes: types.FileSizeBytes(100),
				},
			},
			expectedError: "filename is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errResp := tc.uploadReq.Validate(cfg.MaxFileSizeBytes)

			if tc.expectedError != "" {
				assert.NotNil(t, errResp)
				assert.Contains(t, errResp.JSON.(map[string]interface{})["error"], tc.expectedError)
			} else {
				assert.Nil(t, errResp)
			}
		})
	}
}

func (suite *UploadTestSuite) Test_QueueThumbnailGeneration() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		service := dep.Service
		mediaID := types.MediaID("test-media-id")

		// Test queue thumbnail generation
		err := queueThumbnailGeneration(ctx, service, mediaID)
		
		// This should not error as it just queues the job
		// In a real test environment, we might want to verify the job was queued
		// but for now we just ensure it doesn't panic
		assert.NoError(t, err)
	})
}

func (suite *UploadTestSuite) Test_RequestEntityTooLargeJSONResponse() {
	maxSize := config.FileSizeBytes(1024 * 1024) // 1MB

	resp := requestEntityTooLargeJSONResponse(maxSize)

	require.NotNil(t, resp)
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)

	respData := resp.JSON.(map[string]interface{})
	assert.Contains(t, respData["error"], "file size exceeds limit")
	assert.Equal(t, float64(maxSize), respData["limit"])
}
