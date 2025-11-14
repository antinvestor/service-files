package connection_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type DatastoreTestSuite struct {
	tests.BaseTestSuite
}

func TestDatastoreTestSuite(t *testing.T) {
	suite.Run(t, new(DatastoreTestSuite))
}

func (suite *DatastoreTestSuite) TestMediaRepository() {
	testCases := []struct {
		name     string
		metadata *types.MediaMetadata
	}{
		{
			name: "can insert media & query media",
			metadata: &types.MediaMetadata{
				MediaID:       "testing",
				ContentType:   "image/png",
				FileSizeBytes: 10,
				UploadName:    "upload test",
				Base64Hash:    "dGVzdGluZw==",
				OwnerID:       "@alice:localhost",
			},
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()

				_, svc, res := suite.CreateService(t, dep)
				db, err := connection.NewMediaDatabase(svc.WorkManager(), res.MediaRepository)
				assert.NoErrorf(t, err, "failed to open media database")

				err = db.StoreMediaMetadata(ctx, tc.metadata)
				if err != nil {
					t.Fatalf("unable to store media metadata: %v", err)
				}

				// query by media id
				gotMetadata, err := db.GetMediaMetadata(ctx, tc.metadata.MediaID)
				if err != nil {
					t.Fatalf("unable to query media metadata: %v", err)
				}
				if !reflect.DeepEqual(tc.metadata, gotMetadata) {
					t.Fatalf("expected metadata %+v, got %v", tc.metadata, gotMetadata)
				}

				// query by media hash
				gotMetadata, err = db.GetMediaMetadataByHash(ctx, tc.metadata.OwnerID, tc.metadata.Base64Hash)
				if err != nil {
					t.Fatalf("unable to query media metadata by hash: %v", err)
				}
				if !reflect.DeepEqual(tc.metadata, gotMetadata) {
					t.Fatalf("expected metadata %+v, got %v", tc.metadata, gotMetadata)
				}
			})
		}
	})
}

func (suite *DatastoreTestSuite) TestThumbnailsStorage() {
	testCases := []struct {
		name       string
		thumbnails []*types.ThumbnailMetadata
	}{
		{
			name: "can insert thumbnails & query media",
			thumbnails: []*types.ThumbnailMetadata{
				{
					MediaMetadata: &types.MediaMetadata{
						MediaID:       "curerv4pf2t9jvceefgg",
						ParentID:      "testing",
						ContentType:   "image/png",
						FileSizeBytes: 6,
						ThumbnailSize: &types.ThumbnailSize{
							Width:        5,
							Height:       5,
							ResizeMethod: types.Crop,
						},
					},
				},
				{
					MediaMetadata: &types.MediaMetadata{
						MediaID:       "curerv4pf2t9jvceefgx",
						ParentID:      "testing",
						ContentType:   "image/png",
						FileSizeBytes: 10,
						ThumbnailSize: &types.ThumbnailSize{
							Width:        10,
							Height:       10,
							ResizeMethod: types.Scale,
						},
					},
				},
			},
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()

				_, svc, res := suite.CreateService(t, dep)
				db, err := connection.NewMediaDatabase(svc.WorkManager(), res.MediaRepository)
				assert.NoErrorf(t, err, "failed to open media database")

				for _, thumbnail := range tc.thumbnails {
					err = db.StoreThumbnail(ctx, thumbnail)
					if err != nil {
						t.Fatalf("unable to store thumbnail metadata: %v", err)
					}
				}

				// query thumbnails by parent id
				gotThumbnails, err := db.GetThumbnails(ctx, "testing")
				if err != nil {
					t.Fatalf("unable to query thumbnail metadata: %v", err)
				}
				if len(gotThumbnails) != len(tc.thumbnails) {
					t.Fatalf("expected %d thumbnails, got %d", len(tc.thumbnails), len(gotThumbnails))
				}
			})
		}
	})
}
