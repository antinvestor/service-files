package routing

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type DownloadTestSuite struct {
	tests.BaseTestSuite
}

func TestDownloadTestSuite(t *testing.T) {
	suite.Run(t, new(DownloadTestSuite))
}

func (suite *DownloadTestSuite) Test_Download() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx := t.Context()
		
		// Create test dependencies
		db := dep.Database
		provider := dep.StorageProvider
		service := dep.Service
		mediaService := business.NewMediaService(db, provider)

		// Test downloading a valid file
		t.Run("download valid file", func(t *testing.T) {
			// First upload a test file
			testData := []byte("test file content")
			req := httptest.NewRequest("POST", "/upload", bytes.NewReader(testData))
			req.Header.Set("Content-Type", "text/plain")
			req.Header.Set("Authorization", "Bearer test-token")
			
			// Set up authenticated context
			ctx = util.SetAuthClaims(ctx, "test-user")
			req = req.WithContext(ctx)

			uploadResp := Upload(req, service, db, provider, mediaService)
			require.NotNil(t, uploadResp)
			require.Equal(t, http.StatusOK, uploadResp.Code)

			// Extract media ID from upload response
			uploadData := uploadResp.JSON.(map[string]interface{})
			mediaID := uploadData["media_id"].(string)
			serverName := uploadData["server_name"].(string)

			// Test download
			downloadReq := httptest.NewRequest("GET", "/download/"+serverName+"/"+mediaID, nil)
			downloadReq = downloadReq.WithContext(ctx)

			w := httptest.NewRecorder()
			Download(w, downloadReq, service, db, provider, mediaService)

			require.Equal(t, http.StatusOK, w.Code)
			downloadedData, err := io.ReadAll(w.Body)
			require.NoError(t, err)
			assert.Equal(t, testData, downloadedData)
		})

		// Test downloading non-existent file
		t.Run("download non-existent file", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/download/invalid-server/invalid-media-id", nil)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			Download(w, req, service, db, provider, mediaService)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		// Test downloading with invalid media ID format
		t.Run("download invalid media ID", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/download/server/invalid@id", nil)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			Download(w, req, service, db, provider, mediaService)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	})
}

func (suite *DownloadTestSuite) Test_AddDownloadHeaders() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		testCases := []struct {
			name               string
			result             *business.DownloadResult
			customFilename     string
			isThumbnailRequest bool
			expectedHeaders    map[string]string
		}{
			{
				name: "regular file download",
				result: &business.DownloadResult{
					ContentType:   "text/plain",
					Filename:      "test.txt",
					ContentLength: 100,
				},
				customFilename:     "",
				isThumbnailRequest: false,
				expectedHeaders: map[string]string{
					"Content-Type":        "text/plain",
					"Content-Disposition": "inline; filename=\"test.txt\"",
				},
			},
			{
				name: "thumbnail download",
				result: &business.DownloadResult{
					ContentType:   "image/jpeg",
					Filename:      "thumb.jpg",
					ContentLength: 50,
				},
				customFilename:     "",
				isThumbnailRequest: true,
				expectedHeaders: map[string]string{
					"Content-Type":        "image/jpeg",
					"Content-Disposition": "attachment; filename=\"thumb.jpg\"",
				},
			},
			{
				name: "download with custom filename",
				result: &business.DownloadResult{
					ContentType:   "application/octet-stream",
					Filename:      "original.bin",
					ContentLength: 200,
				},
				customFilename:     "custom.bin",
				isThumbnailRequest: false,
				expectedHeaders: map[string]string{
					"Content-Type":        "application/octet-stream",
					"Content-Disposition": "attachment; filename=\"custom.bin\"",
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				addDownloadHeaders(w, tc.result, tc.customFilename, tc.isThumbnailRequest)

				for key, expectedValue := range tc.expectedHeaders {
					assert.Equal(t, expectedValue, w.Header().Get(key), "Header %s should match", key)
				}
			})
		}
	})
}

func (suite *DownloadTestSuite) Test_HandleDownloadError() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		testCases := []struct {
			name           string
			err            error
			expectedStatus int
		}{
			{
				name:           "not found error",
				err:            util.JSONResponse{Code: http.StatusNotFound},
				expectedStatus: http.StatusNotFound,
			},
			{
				name:           "internal server error",
				err:            util.JSONResponse{Code: http.StatusInternalServerError},
				expectedStatus: http.StatusInternalServerError,
			},
			{
				name:           "generic error",
				err:            assert.AnError,
				expectedStatus: http.StatusInternalServerError,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				handleDownloadError(w, tc.err)
				assert.Equal(t, tc.expectedStatus, w.Code)
			})
		}
	})
}

func (suite *DownloadTestSuite) Test_IsValidMediaID() {
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
			name:     "too short media ID",
			mediaID:  "abc",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidMediaID(tc.mediaID)
			assert.Equal(t, tc.expected, result)
		})
	}
}
