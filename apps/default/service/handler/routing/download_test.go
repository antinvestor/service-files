package routing

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// denyByDefaultAuthorizer wraps an authorizer and denies all Check requests
// that would otherwise be permitted by permissive mode (when Keto is unconfigured).
// Owner checks still pass because they happen before the authorizer is called.
type denyByDefaultAuthorizer struct {
	security.Authorizer
}

func (d *denyByDefaultAuthorizer) Check(_ context.Context, _ security.CheckRequest) (security.CheckResult, error) {
	return security.CheckResult{
		Allowed:   false,
		Reason:    "deny-by-default test authorizer",
		CheckedAt: time.Now().Unix(),
	}, nil
}

func (d *denyByDefaultAuthorizer) BatchCheck(_ context.Context, requests []security.CheckRequest) ([]security.CheckResult, error) {
	results := make([]security.CheckResult, len(requests))
	for i := range results {
		results[i] = security.CheckResult{
			Allowed:   false,
			Reason:    "deny-by-default test authorizer",
			CheckedAt: time.Now().Unix(),
		}
	}
	return results, nil
}

type DownloadRoutingTestSuite struct {
	tests.BaseTestSuite
}

func TestDownloadRoutingTestSuite(t *testing.T) {
	suite.Run(t, new(DownloadRoutingTestSuite))
}

func (suite *DownloadRoutingTestSuite) TestAddDownloadHeaders() {
	testCases := []struct {
		name                 string
		result               *business.DownloadResult
		customFilename       string
		isThumbnailRequest   bool
		expectDispositionSet bool
		expectCacheControl   string
	}{
		{
			name: "cached_file_download",
			result: &business.DownloadResult{
				ContentType:   "text/plain",
				ContentLength: 5,
				IsCached:      true,
			},
			customFilename:       "a.txt",
			isThumbnailRequest:   false,
			expectDispositionSet: true,
			expectCacheControl:   "public, max-age=31536000",
		},
		{
			name: "thumbnail_ignores_disposition",
			result: &business.DownloadResult{
				ContentType:   "image/jpeg",
				ContentLength: 100,
				IsCached:      false,
			},
			customFilename:       "thumb.jpg",
			isThumbnailRequest:   true,
			expectDispositionSet: false,
			expectCacheControl:   "public, max-age=3600",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			addDownloadHeaders(rec, tc.result, tc.customFilename, tc.isThumbnailRequest)
			assert.Equal(t, tc.result.ContentType, rec.Header().Get("Content-Type"))
			assert.Equal(t, tc.expectCacheControl, rec.Header().Get("Cache-Control"))
			if tc.expectDispositionSet {
				assert.Contains(t, rec.Header().Get("Content-Disposition"), tc.customFilename)
			} else {
				assert.Empty(t, rec.Header().Get("Content-Disposition"))
			}
		})
	}
}

func (suite *DownloadRoutingTestSuite) TestHandleDownloadError() {
	testCases := []struct {
		name string
		err  error
	}{
		{
			name: "writes_error_json",
			err:  errors.New("boom"),
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handleDownloadError(suite.T().Context(), rec, tc.err)
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.Contains(t, rec.Body.String(), "boom")
		})
	}
}

func (suite *DownloadRoutingTestSuite) TestDownload() {
	testCases := []struct {
		name        string
		subject     string
		owner       string
		mediaID     string
		setupUpload bool
		wantCode    int
	}{
		{
			name:        "unauthenticated",
			owner:       "@owner:example.com",
			mediaID:     "downloadMediaA",
			setupUpload: true,
			wantCode:    http.StatusUnauthorized,
		},
		{
			name:        "non_owner_denied",
			subject:     "@other:example.com",
			owner:       "@owner:example.com",
			mediaID:     "downloadMediaB",
			setupUpload: true,
			wantCode:    http.StatusForbidden,
		},
		{
			name:        "owner_downloads_file",
			subject:     "@owner:example.com",
			owner:       "@owner:example.com",
			mediaID:     "downloadMediaC",
			setupUpload: true,
			wantCode:    http.StatusOK,
		},
		{
			name:     "invalid_media_missing_file",
			subject:  "@owner:example.com",
			owner:    "@owner:example.com",
			mediaID:  "missingMediaD",
			wantCode: http.StatusForbidden,
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
		// Use deny-by-default authorizer to prevent permissive mode from masking authz gaps
		baseAuthorizer := svc.SecurityManager().GetAuthorizer(ctx)
		denyAuthorizer := &denyByDefaultAuthorizer{Authorizer: baseAuthorizer}
		authzMiddleware := authz.NewMiddleware(denyAuthorizer, db)

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.setupUpload {
					content := "download-content-" + tc.mediaID
					_, uploadErr := mediaService.UploadFile(ctx, &business.UploadRequest{
						OwnerID:       types.OwnerID(tc.owner),
						MediaID:       types.MediaID(tc.mediaID),
						UploadName:    "file.txt",
						ContentType:   "text/plain",
						FileSizeBytes: types.FileSizeBytes(len(content)),
						FileData:      bytes.NewReader([]byte(content)),
						Config:        cfg,
					})
					require.NoError(t, uploadErr)
				}

				req := httptest.NewRequest(http.MethodGet, "/v1/media/download/"+cfg.ServerName+"/"+tc.mediaID, nil)
				if tc.subject != "" {
					claims := &security.AuthenticationClaims{
						RegisteredClaims: jwt.RegisteredClaims{Subject: tc.subject},
					}
					req = req.WithContext(claims.ClaimsToContext(req.Context()))
				}

				rec := httptest.NewRecorder()
				Download(rec, req, types.MediaID(tc.mediaID), cfg, db, storageProvider, mediaService, authzMiddleware, false, "")
				assert.Equal(t, tc.wantCode, rec.Code)
				if tc.wantCode == http.StatusOK {
					assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
					assert.True(t, strings.Contains(rec.Body.String(), "download-content-"))
				}
			})
		}
	})
}
