package routing

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
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

type SearchRoutingTestSuite struct {
	tests.BaseTestSuite
}

func TestSearchRoutingTestSuite(t *testing.T) {
	suite.Run(t, new(SearchRoutingTestSuite))
}

func (suite *SearchRoutingTestSuite) TestSearch() {
	testCases := []struct {
		name       string
		subject    string
		query      string
		page       string
		limit      string
		seedUpload bool
		wantCode   int
	}{
		{
			name:       "unauthenticated_request",
			query:      "invoice",
			wantCode:   http.StatusUnauthorized,
			seedUpload: false,
		},
		{
			name:       "authenticated_search_success",
			subject:    "@searcher:example.com",
			query:      "",
			page:       "bad-page",
			limit:      "99999",
			seedUpload: true,
			wantCode:   http.StatusOK,
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

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.seedUpload {
					content := "invoice body"
					_, uploadErr := mediaService.UploadFile(ctx, &business.UploadRequest{
						OwnerID:       types.OwnerID(tc.subject),
						MediaID:       "searchRoutingMedia",
						UploadName:    "invoice-2026.pdf",
						ContentType:   "application/pdf",
						FileSizeBytes: types.FileSizeBytes(len(content)),
						FileData:      strings.NewReader(content),
						Config:        cfg,
					})
					require.NoError(t, uploadErr)
				}

				req := httptest.NewRequest(
					http.MethodGet,
					"/v1/media/search?query="+tc.query+"&page="+tc.page+"&limit="+tc.limit,
					nil,
				)
				if tc.subject != "" {
					claims := &security.AuthenticationClaims{
						RegisteredClaims: jwt.RegisteredClaims{Subject: tc.subject},
					}
					req = req.WithContext(claims.ClaimsToContext(req.Context()))
				}

				resp := Search(req, svc, db, mediaService)
				assert.Equal(t, tc.wantCode, resp.Code)
				if tc.wantCode == http.StatusOK {
					typed, ok := resp.JSON.(searchResponse)
					require.True(t, ok)
					assert.GreaterOrEqual(t, typed.Count, 0)
				}
			})
		}
	})
}
