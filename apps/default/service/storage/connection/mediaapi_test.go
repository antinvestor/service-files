package connection_test

import (
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ConnectionTestSuite struct {
	tests.BaseTestSuite
}

func TestConnectionTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionTestSuite))
}

func (suite *ConnectionTestSuite) TestStoreMediaMetadata() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {

		ctx, svc, res := suite.CreateService(t, dep)

		mediaRepo := res.MediaRepository
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: mediaRepo,
		}

		testCases := []struct {
			name     string
			metadata *types.MediaMetadata
			wantErr  bool
		}{
			{
				name: "successful store",
				metadata: &types.MediaMetadata{
					MediaID:           "test-media-1",
					UploadName:        "test-file.jpg",
					ContentType:       "image/jpeg",
					FileSizeBytes:     1024,
					CreationTimestamp: uint64(time.Now().Unix()),
					IsPublic:          false,
					Base64Hash:        "test-hash-123",
					ServerName:        "test-bucket",
					OwnerID:           "test-owner-1",
				},
				wantErr: false,
			},
			{
				name: "store with parent ID",
				metadata: &types.MediaMetadata{
					MediaID:           "test-media-2",
					UploadName:        "thumbnail.jpg",
					ContentType:       "image/jpeg",
					FileSizeBytes:     256,
					CreationTimestamp: uint64(time.Now().Unix()),
					IsPublic:          true,
					Base64Hash:        "thumbnail-hash-456",
					ServerName:        "test-bucket",
					OwnerID:           "test-owner-2",
					ParentID:          "parent-media-id",
				},
				wantErr: false,
			},
			{
				name: "store with empty media ID",
				metadata: &types.MediaMetadata{
					MediaID:           "",
					UploadName:        "empty-id.jpg",
					ContentType:       "image/jpeg",
					FileSizeBytes:     512,
					CreationTimestamp: uint64(time.Now().Unix()),
					Base64Hash:        "empty-id-hash",
					ServerName:        "test-bucket",
					OwnerID:           "test-owner-3",
				},
				wantErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := db.StoreMediaMetadata(ctx, tc.metadata)
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func (suite *ConnectionTestSuite) TestGetMediaMetadata() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {

		ctx, svc, res := suite.CreateService(t, dep)

		mediaRepo := res.MediaRepository
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: mediaRepo,
		}

		// First, store a test media
		testMetadata := &types.MediaMetadata{
			MediaID:           "get-test-media",
			UploadName:        "get-test.jpg",
			ContentType:       "image/jpeg",
			FileSizeBytes:     2048,
			CreationTimestamp: uint64(time.Now().Unix()),
			Base64Hash:        "get-test-hash",
			ServerName:        "test-bucket",
			OwnerID:           "test-owner-get",
		}
		err := db.StoreMediaMetadata(ctx, testMetadata)
		assert.NoError(t, err)

		testCases := []struct {
			name    string
			mediaID types.MediaID
			wantNil bool
			wantErr bool
		}{
			{
				name:    "existing media",
				mediaID: types.MediaID("get-test-media"),
				wantNil: false,
				wantErr: false,
			},
			{
				name:    "non-existent media",
				mediaID: types.MediaID("non-existent-id"),
				wantNil: true,
				wantErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := db.GetMediaMetadata(ctx, tc.mediaID)
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					if tc.wantNil {
						assert.Nil(t, result)
					} else {
						assert.NotNil(t, result)
						assert.Equal(t, testMetadata.MediaID, result.MediaID)
						assert.Equal(t, testMetadata.OwnerID, result.OwnerID)
						assert.Equal(t, testMetadata.UploadName, result.UploadName)
					}
				}
			})
		}
	})
}

func (suite *ConnectionTestSuite) TestGetMediaMetadataByHash() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {

		ctx, svc, res := suite.CreateService(t, dep)

		mediaRepo := res.MediaRepository
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: mediaRepo,
		}

		// First, store a test media
		testMetadata := &types.MediaMetadata{
			MediaID:           "hash-test-media",
			UploadName:        "hash-test.jpg",
			ContentType:       "image/jpeg",
			FileSizeBytes:     3072,
			CreationTimestamp: uint64(time.Now().Unix()),
			Base64Hash:        "unique-hash-123",
			ServerName:        "test-bucket",
			OwnerID:           "test-owner-hash",
		}
		err := db.StoreMediaMetadata(ctx, testMetadata)
		assert.NoError(t, err)

		testCases := []struct {
			name    string
			ownerID types.OwnerID
			hash    types.Base64Hash
			wantNil bool
			wantErr bool
		}{
			{
				name:    "existing hash for owner",
				ownerID: types.OwnerID("test-owner-hash"),
				hash:    types.Base64Hash("unique-hash-123"),
				wantNil: false,
				wantErr: false,
			},
			{
				name:    "non-existent hash",
				ownerID: types.OwnerID("test-owner-hash"),
				hash:    types.Base64Hash("non-existent-hash"),
				wantNil: true,
				wantErr: false,
			},
			{
				name:    "wrong owner",
				ownerID: types.OwnerID("wrong-owner"),
				hash:    types.Base64Hash("unique-hash-123"),
				wantNil: true,
				wantErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := db.GetMediaMetadataByHash(ctx, tc.ownerID, tc.hash)
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					if tc.wantNil {
						assert.Nil(t, result)
					} else {
						assert.NotNil(t, result)
						assert.Equal(t, testMetadata.Base64Hash, result.Base64Hash)
						assert.Equal(t, testMetadata.OwnerID, result.OwnerID)
					}
				}
			})
		}
	})
}

func (suite *ConnectionTestSuite) TestSearch() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {

		ctx, svc, res := suite.CreateService(t, dep)

		mediaRepo := res.MediaRepository
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: mediaRepo,
		}

		ownerID := "test-owner-search"

		// Store test media with different properties
		testMetadata := &types.MediaMetadata{
			MediaID:           "searchable-media",
			UploadName:        "searchable-file.jpg",
			ContentType:       "image/jpeg",
			FileSizeBytes:     4096,
			CreationTimestamp: uint64(time.Now().Unix()),
			Base64Hash:        "search-hash",
			ServerName:        "test-bucket",
			OwnerID:           types.OwnerID(ownerID),
		}
		err := db.StoreMediaMetadata(ctx, testMetadata)
		assert.NoError(t, err)

		testCases := []struct {
			name    string
			query   *data.SearchQuery
			wantErr bool
		}{
			{
				name: "basic search query",
				query: data.NewSearchQuery(
					data.WithSearchFiltersOrByValue(map[string]interface{}{
						"owner_id": ownerID,
					}),
					data.WithSearchLimit(10),
					data.WithSearchOffset(0),
				),
				wantErr: false,
			},
			{
				name: "empty search query",
				query: data.NewSearchQuery(
					data.WithSearchFiltersOrByValue(map[string]interface{}{
						"owner_id": ownerID,
					}),
					data.WithSearchLimit(10),
					data.WithSearchOffset(0),
				),
				wantErr: false,
			},
			{
				name: "search with date range",
				query: data.NewSearchQuery(
					data.WithSearchFiltersOrByValue(map[string]interface{}{
						"owner_id":   ownerID,
						"start_date": time.Now().Add(-24 * time.Hour).Unix(),
						"end_date":   time.Now().Add(24 * time.Hour).Unix(),
					}),
					data.WithSearchLimit(10),
					data.WithSearchOffset(0),
				),
				wantErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resultPipe, err := db.Search(ctx, tc.query)
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
						item := result.Item()
						assert.NotNil(t, item)
						t.Logf("Found search result: %s", item.MediaID)
					}
				}
			})
		}
	})
}

func (suite *ConnectionTestSuite) TestStoreThumbnail() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {

		ctx, svc, res := suite.CreateService(t, dep)

		mediaRepo := res.MediaRepository
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: mediaRepo,
		}

		testCases := []struct {
			name              string
			thumbnailMetadata *types.ThumbnailMetadata
			wantErr           bool
		}{
			{
				name: "successful thumbnail store",
				thumbnailMetadata: &types.ThumbnailMetadata{
					MediaMetadata: &types.MediaMetadata{
						MediaID:           "test-media-thumb",
						UploadName:        "thumb.jpg",
						ContentType:       "image/jpeg",
						FileSizeBytes:     512,
						CreationTimestamp: uint64(time.Now().Unix()),
						Base64Hash:        "thumb-hash-123",
						ServerName:        "test-bucket",
						OwnerID:           "test-owner-thumb",
					},
				},
				wantErr: false,
			},
			{
				name: "thumbnail with scale method",
				thumbnailMetadata: &types.ThumbnailMetadata{
					MediaMetadata: &types.MediaMetadata{
						MediaID:           "test-media-thumb-2",
						UploadName:        "thumb2.jpg",
						ContentType:       "image/png",
						FileSizeBytes:     1024,
						CreationTimestamp: uint64(time.Now().Unix()),
						Base64Hash:        "thumb-hash-456",
						ServerName:        "test-bucket",
						OwnerID:           "test-owner-thumb-2",
					},
				},
				wantErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := db.StoreThumbnail(ctx, tc.thumbnailMetadata)
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func (suite *ConnectionTestSuite) TestGetThumbnail() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {

		ctx, svc, res := suite.CreateService(t, dep)

		mediaRepo := res.MediaRepository
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: mediaRepo,
		}

		// First, store a test thumbnail
		testThumbnail := &types.ThumbnailMetadata{
			MediaMetadata: &types.MediaMetadata{
				MediaID:           "get-thumb-media-thumb",
				ParentID:          "get-thumb-media",
				UploadName:        "get-thumb.jpg",
				ContentType:       "image/jpeg",
				FileSizeBytes:     768,
				CreationTimestamp: uint64(time.Now().Unix()),
				Base64Hash:        "get-thumb-hash",
				ServerName:        "test-bucket",
				OwnerID:           "test-owner-thumb",
				ThumbnailSize: &types.ThumbnailSize{
					Width:        150,
					Height:       150,
					ResizeMethod: "crop",
				},
			},
		}
		err := db.StoreThumbnail(ctx, testThumbnail)
		assert.NoError(t, err)

		testCases := []struct {
			name         string
			mediaID      types.MediaID
			width        int
			height       int
			resizeMethod string
			wantNil      bool
			wantErr      bool
		}{
			{
				name:         "existing thumbnail",
				mediaID:      types.MediaID("get-thumb-media"),
				width:        150,
				height:       150,
				resizeMethod: "crop",
				wantNil:      false,
				wantErr:      false,
			},
			{
				name:         "non-existent thumbnail",
				mediaID:      types.MediaID("non-existent-media"),
				width:        150,
				height:       150,
				resizeMethod: "crop",
				wantNil:      true,
				wantErr:      false,
			},
			{
				name:         "wrong dimensions",
				mediaID:      types.MediaID("get-thumb-media"),
				width:        200,
				height:       200,
				resizeMethod: "crop",
				wantNil:      true,
				wantErr:      false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := db.GetThumbnail(ctx, tc.mediaID, tc.width, tc.height, tc.resizeMethod)
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					if tc.wantNil {
						assert.Nil(t, result)
					} else {
						assert.NotNil(t, result)
						if result != nil {
							assert.Equal(t, testThumbnail.MediaID, result.MediaID)
						}
					}
				}
			})
		}
	})
}

func (suite *ConnectionTestSuite) TestGetThumbnails() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {

		ctx, svc, res := suite.CreateService(t, dep)

		mediaRepo := res.MediaRepository
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: mediaRepo,
		}

		mediaID := types.MediaID("list-thumb-media")

		// Store multiple thumbnails for the same media
		testThumbnails := []*types.ThumbnailMetadata{
			{
				MediaMetadata: &types.MediaMetadata{
					MediaID:           "list-thumb-media-thumb-1",
					ParentID:          mediaID,
					UploadName:        "thumb1.jpg",
					ContentType:       "image/jpeg",
					FileSizeBytes:     512,
					CreationTimestamp: uint64(time.Now().Unix()),
					Base64Hash:        "thumb-hash-1",
					ServerName:        "test-bucket",
					OwnerID:           "test-owner-thumb",
					ThumbnailSize: &types.ThumbnailSize{
						Width:        150,
						Height:       150,
						ResizeMethod: "crop",
					},
				},
			},
			{
				MediaMetadata: &types.MediaMetadata{
					MediaID:           "list-thumb-media-thumb-2",
					ParentID:          mediaID,
					UploadName:        "thumb2.jpg",
					ContentType:       "image/jpeg",
					FileSizeBytes:     1024,
					CreationTimestamp: uint64(time.Now().Unix()),
					Base64Hash:        "thumb-hash-2",
					ServerName:        "test-bucket",
					OwnerID:           "test-owner-thumb",
					ThumbnailSize: &types.ThumbnailSize{
						Width:        200,
						Height:       200,
						ResizeMethod: "crop",
					},
				},
			},
		}

		for _, thumb := range testThumbnails {
			err := db.StoreThumbnail(ctx, thumb)
			assert.NoError(t, err)
		}

		testCases := []struct {
			name          string
			mediaID       types.MediaID
			expectedCount int
			wantErr       bool
		}{
			{
				name:          "get all thumbnails for media",
				mediaID:       types.MediaID(mediaID),
				expectedCount: 2,
				wantErr:       false,
			},
			{
				name:          "non-existent media",
				mediaID:       types.MediaID("non-existent-media"),
				expectedCount: 0,
				wantErr:       false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				results, err := db.GetThumbnails(ctx, tc.mediaID)
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Len(t, results, tc.expectedCount)
					for _, result := range results {
						assert.Equal(t, tc.mediaID, result.ParentID)
					}
				}
			})
		}
	})
}

func (suite *ConnectionTestSuite) TestNewMediaDatabase() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, svc, res := suite.CreateService(t, dep)

		testCases := []struct {
			name    string
			wantErr bool
		}{
			{
				name:    "successful database creation",
				wantErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				db, err := connection.NewMediaDatabase(svc.WorkManager(), res.MediaRepository)
				if tc.wantErr {
					assert.Error(t, err)
					assert.Nil(t, db)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, db)
				}
			})
		}
	})
}
