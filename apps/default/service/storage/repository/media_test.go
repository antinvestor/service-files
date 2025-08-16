package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/internal/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/framedata"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MediaRepositoryTestSuite struct {
	tests.BaseTestSuite
}

func TestMediaRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(MediaRepositoryTestSuite))
}

func (suite *MediaRepositoryTestSuite) createService(t *testing.T, dep *definition.DependancyOption) *frame.Service {

	ctx := t.Context()
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	profileConfig, err := frame.ConfigFromEnv[config.FilesConfig]()
	require.NoError(t, err)

	profileConfig.LogLevel = "debug"
	profileConfig.RunServiceSecurely = false
	profileConfig.ServerPort = ""

	for _, res := range dep.Database(ctx) {
		testDS, cleanup, err0 := res.GetRandomisedDS(ctx, dep.Prefix())
		require.NoError(t, err0)

		t.Cleanup(func() {
			cleanup(ctx)
		})

		profileConfig.DatabasePrimaryURL = []string{testDS.String()}
		profileConfig.DatabaseReplicaURL = []string{testDS.String()}
	}

	ctx, svc := frame.NewServiceWithContext(ctx, "repository tests",
		frame.WithConfig(&profileConfig),
		frame.WithDatastore(),
		frame.WithNoopDriver())

	svc.Init(ctx)

	err = repository.Migrate(ctx, svc, "../../../migrations/0001")
	require.NoError(t, err)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	return svc
}

func (suite *MediaRepositoryTestSuite) TestSave() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependancyOption) {
		ctx := context.Background()

		svc := suite.createService(t, dep)

		repo := repository.NewMediaRepository(svc)

		testCases := []struct {
			name     string
			metadata *models.MediaMetadata
			wantErr  bool
		}{
			{
				name: "successful save",
				metadata: &models.MediaMetadata{
					OwnerID:    "test-owner-1",
					ParentID:   "",
					Name:       "test-file.jpg",
					Ext:        "jpg",
					Size:       1024,
					OriginTs:   time.Now().Unix(),
					Public:     false,
					Mimetype:   "image/jpeg",
					Hash:       "test-hash-123",
					BucketName: "test-bucket",
					Provider:   "local",
					Properties: frame.JSONMap{},
				},
				wantErr: false,
			},
			{
				name: "save with parent ID",
				metadata: &models.MediaMetadata{
					OwnerID:    "test-owner-2",
					ParentID:   "parent-media-id",
					Name:       "thumbnail.jpg",
					Ext:        "jpg",
					Size:       256,
					OriginTs:   time.Now().Unix(),
					Public:     true,
					Mimetype:   "image/jpeg",
					Hash:       "thumbnail-hash-456",
					BucketName: "test-bucket",
					Provider:   "local",
					Properties: frame.JSONMap{"width": 100, "height": 100},
				},
				wantErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := repo.Save(ctx, tc.metadata)
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.NotEmpty(t, tc.metadata.ID)
				}
			})
		}
	})
}

func (suite *MediaRepositoryTestSuite) TestGetByID() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependancyOption) {
		ctx := context.Background()

		svc := suite.createService(t, dep)

		repo := repository.NewMediaRepository(svc)

		// First, save a test media
		testMedia := &models.MediaMetadata{
			OwnerID:    "test-owner-get",
			Name:       "get-test.jpg",
			Ext:        "jpg",
			Size:       2048,
			OriginTs:   time.Now().Unix(),
			Mimetype:   "image/jpeg",
			Hash:       "get-test-hash",
			BucketName: "test-bucket",
			Provider:   "local",
			Properties: frame.JSONMap{},
		}
		err := repo.Save(ctx, testMedia)
		assert.NoError(t, err)

		testCases := []struct {
			name    string
			mediaID types.MediaID
			wantErr bool
		}{
			{
				name:    "existing media",
				mediaID: types.MediaID(testMedia.ID),
				wantErr: false,
			},
			{
				name:    "non-existent media",
				mediaID: types.MediaID("non-existent-id"),
				wantErr: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := repo.GetByID(ctx, tc.mediaID)
				if tc.wantErr {
					assert.Error(t, err)
					assert.Nil(t, result)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, result)
					assert.Equal(t, testMedia.ID, result.ID)
					assert.Equal(t, testMedia.OwnerID, result.OwnerID)
					assert.Equal(t, testMedia.Name, result.Name)
				}
			})
		}
	})
}

func (suite *MediaRepositoryTestSuite) TestGetByHash() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependancyOption) {
		ctx := context.Background()

		svc := suite.createService(t, dep)

		repo := repository.NewMediaRepository(svc)

		// First, save a test media
		testMedia := &models.MediaMetadata{
			OwnerID:    "test-owner-hash",
			Name:       "hash-test.jpg",
			Ext:        "jpg",
			Size:       3072,
			OriginTs:   time.Now().Unix(),
			Mimetype:   "image/jpeg",
			Hash:       "unique-hash-123",
			BucketName: "test-bucket",
			Provider:   "local",
			Properties: frame.JSONMap{},
		}
		err := repo.Save(ctx, testMedia)
		assert.NoError(t, err)

		testCases := []struct {
			name    string
			ownerID types.OwnerID
			hash    types.Base64Hash
			wantErr bool
		}{
			{
				name:    "existing hash for owner",
				ownerID: types.OwnerID(testMedia.OwnerID),
				hash:    types.Base64Hash(testMedia.Hash),
				wantErr: false,
			},
			{
				name:    "non-existent hash",
				ownerID: types.OwnerID(testMedia.OwnerID),
				hash:    types.Base64Hash("non-existent-hash"),
				wantErr: true,
			},
			{
				name:    "wrong owner",
				ownerID: types.OwnerID("wrong-owner"),
				hash:    types.Base64Hash(testMedia.Hash),
				wantErr: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := repo.GetByHash(ctx, tc.ownerID, tc.hash)
				if tc.wantErr {
					assert.Error(t, err)
					assert.Nil(t, result)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, result)
					assert.Equal(t, testMedia.Hash, result.Hash)
					assert.Equal(t, testMedia.OwnerID, result.OwnerID)
				}
			})
		}
	})
}

func (suite *MediaRepositoryTestSuite) TestGetByOwnerID() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependancyOption) {
		ctx := context.Background()

		svc := suite.createService(t, dep)

		repo := repository.NewMediaRepository(svc)

		ownerID := "test-owner-list"

		// Save multiple test media for the same owner
		testMedia := []*models.MediaMetadata{
			{
				OwnerID:    ownerID,
				Name:       "file1.jpg",
				Ext:        "jpg",
				Size:       1024,
				OriginTs:   time.Now().Unix(),
				Mimetype:   "image/jpeg",
				Hash:       "hash1",
				BucketName: "test-bucket",
				Provider:   "local",
				Properties: frame.JSONMap{},
			},
			{
				OwnerID:    ownerID,
				Name:       "file2.png",
				Ext:        "png",
				Size:       2048,
				OriginTs:   time.Now().Unix(),
				Mimetype:   "image/png",
				Hash:       "hash2",
				BucketName: "test-bucket",
				Provider:   "local",
				Properties: frame.JSONMap{},
			},
		}

		for _, media := range testMedia {
			err := repo.Save(ctx, media)
			assert.NoError(t, err)
		}

		testCases := []struct {
			name          string
			ownerID       types.OwnerID
			query         string
			page          int32
			limit         int32
			expectedCount int
			wantErr       bool
		}{
			{
				name:          "get all files for owner",
				ownerID:       types.OwnerID(ownerID),
				query:         "",
				page:          0,
				limit:         10,
				expectedCount: 2,
				wantErr:       false,
			},
			{
				name:          "search with query",
				ownerID:       types.OwnerID(ownerID),
				query:         "file1",
				page:          0,
				limit:         10,
				expectedCount: 1,
				wantErr:       false,
			},
			{
				name:          "pagination test",
				ownerID:       types.OwnerID(ownerID),
				query:         "",
				page:          0,
				limit:         1,
				expectedCount: 1,
				wantErr:       false,
			},
			{
				name:          "non-existent owner",
				ownerID:       types.OwnerID("non-existent-owner"),
				query:         "",
				page:          0,
				limit:         10,
				expectedCount: 0,
				wantErr:       false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				results, err := repo.GetByOwnerID(ctx, tc.ownerID, tc.query, tc.page, tc.limit)
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Len(t, results, tc.expectedCount)
					for _, result := range results {
						assert.Equal(t, string(tc.ownerID), result.OwnerID)
					}
				}
			})
		}
	})
}

func (suite *MediaRepositoryTestSuite) TestSearch() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependancyOption) {
		ctx := context.Background()

		svc := suite.createService(t, dep)

		repo := repository.NewMediaRepository(svc)

		ownerID := "test-owner-search"

		// Save test media with different properties
		testMedia := &models.MediaMetadata{
			OwnerID:    ownerID,
			Name:       "searchable-file.jpg",
			Ext:        "jpg",
			Size:       4096,
			OriginTs:   time.Now().Unix(),
			Mimetype:   "image/jpeg",
			Hash:       "search-hash",
			BucketName: "test-bucket",
			Provider:   "local",
			Properties: frame.JSONMap{"category": "photos", "tags": "vacation"},
		}
		err := repo.Save(ctx, testMedia)
		assert.NoError(t, err)

		testCases := []struct {
			name    string
			query   *framedata.SearchQuery
			wantErr bool
		}{
			{
				name: "basic search query",
				query: framedata.NewSearchQuery("searchable", map[string]interface{}{
					"owner_id": ownerID,
				}, 10, 0),
				wantErr: false,
			},
			{
				name: "empty search query",
				query: framedata.NewSearchQuery("", map[string]interface{}{
					"owner_id": ownerID,
				}, 10, 0),
				wantErr: false,
			},
			{
				name: "search with date range",
				query: framedata.NewSearchQuery("", map[string]interface{}{
					"owner_id":   ownerID,
					"start_date": time.Now().Add(-24 * time.Hour).Unix(),
					"end_date":   time.Now().Add(24 * time.Hour).Unix(),
				}, 10, 0),
				wantErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resultPipe, err := repo.Search(ctx, tc.query)
				if tc.wantErr {
					assert.Error(t, err)
					assert.Nil(t, resultPipe)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, resultPipe)

					// Read results from the pipe
					for {
						result, ok := resultPipe.ReadResult(ctx)
						if !ok {
							break
						}
						if result.IsError() {
							t.Logf("Search result error: %v", result.Error())
							continue
						}
						results := result.Item()
						assert.NotNil(t, results)
						t.Logf("Found %d search results", len(results))
					}
				}
			})
		}
	})
}

func (suite *MediaRepositoryTestSuite) TestDelete() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependancyOption) {
		ctx := context.Background()

		svc := suite.createService(t, dep)

		repo := repository.NewMediaRepository(svc)

		// First, save a test media
		testMedia := &models.MediaMetadata{
			OwnerID:    "test-owner-delete",
			Name:       "delete-test.jpg",
			Ext:        "jpg",
			Size:       1024,
			OriginTs:   time.Now().Unix(),
			Mimetype:   "image/jpeg",
			Hash:       "delete-hash",
			BucketName: "test-bucket",
			Provider:   "local",
			Properties: frame.JSONMap{},
		}
		err := repo.Save(ctx, testMedia)
		assert.NoError(t, err)

		testCases := []struct {
			name    string
			mediaID types.MediaID
			wantErr bool
		}{
			{
				name:    "delete existing media",
				mediaID: types.MediaID(testMedia.ID),
				wantErr: false,
			},
			{
				name:    "delete non-existent media",
				mediaID: types.MediaID("non-existent-id"),
				wantErr: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err = repo.Delete(ctx, tc.mediaID)
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)

					// Verify the media is actually deleted
					_, err := repo.GetByID(ctx, tc.mediaID)
					assert.Error(t, err) // Should error because it's deleted
				}
			})
		}
	})
}
