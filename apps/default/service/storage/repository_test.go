package storage

import (
	"context"
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type MediaRepositoryTestSuite struct {
	tests.BaseTestSuite
}

func TestMediaRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(MediaRepositoryTestSuite))
}

func (suite *MediaRepositoryTestSuite) TestMediaRepository() {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T, dep *definition.DependancyOption)
	}{
		{
			name: "can_create_and_retrieve_media_by_id",
			testFunc: func(t *testing.T, dep *definition.DependancyOption) {
				ctx := context.Background()
				service, _ := suite.CreateService(t, dep)
				repo := repository.NewMediaRepository(service)

				// Create test media metadata
				media := &models.MediaMetadata{
					OwnerID:    "test-owner",
					Name:       "test-file.jpg",
					Ext:        "jpg",
					Size:       1024,
					OriginTs:   1640995200,
					Public:     true,
					Mimetype:   "image/jpeg",
					Hash:       "test-hash-123",
					BucketName: "test-bucket",
					Provider:   "local",
				}

				// Save media
				err := repo.Save(ctx, media)
				suite.Require().NoError(err)
				suite.Require().NotEmpty(media.GetID())

				// Retrieve by ID
				retrieved, err := repo.GetByID(ctx, types.MediaID(media.GetID()))
				suite.Require().NoError(err)
				suite.Require().NotNil(retrieved)
				suite.Equal(media.OwnerID, retrieved.OwnerID)
				suite.Equal(media.Name, retrieved.Name)
				suite.Equal(media.Hash, retrieved.Hash)
			},
		},
		{
			name: "can_retrieve_media_by_hash",
			testFunc: func(t *testing.T, dep *definition.DependancyOption) {
				ctx := context.Background()
				service, _ := suite.CreateService(t, dep)
				repo := repository.NewMediaRepository(service)

				// Create test media metadata
				media := &models.MediaMetadata{
					OwnerID:    "test-owner-2",
					Name:       "test-file-2.png",
					Ext:        "png",
					Size:       2048,
					OriginTs:   1640995300,
					Public:     false,
					Mimetype:   "image/png",
					Hash:       "unique-hash-456",
					BucketName: "test-bucket-2",
					Provider:   "s3",
				}

				// Save media
				err := repo.Save(ctx, media)
				suite.Require().NoError(err)

				// Retrieve by hash
				retrieved, err := repo.GetByHash(ctx, types.OwnerID("test-owner-2"), types.Base64Hash("unique-hash-456"))
				suite.Require().NoError(err)
				suite.Require().NotNil(retrieved)
				suite.Equal(media.Name, retrieved.Name)
				suite.Equal(media.Hash, retrieved.Hash)
				suite.Equal(media.OwnerID, retrieved.OwnerID)
			},
		},
		{
			name: "can_delete_media",
			testFunc: func(t *testing.T, dep *definition.DependancyOption) {
				ctx := context.Background()
				service, _ := suite.CreateService(t, dep)
				repo := repository.NewMediaRepository(service)

				// Create test media
				media := &models.MediaMetadata{
					OwnerID:    "test-owner-6",
					Name:       "delete-me.jpg",
					Ext:        "jpg",
					Size:       512,
					OriginTs:   1640996100,
					Public:     true,
					Mimetype:   "image/jpeg",
					Hash:       "delete-hash",
					BucketName: "test-bucket-6",
					Provider:   "local",
				}

				err := repo.Save(ctx, media)
				suite.Require().NoError(err)

				mediaID := types.MediaID(media.GetID())

				// Verify media exists
				retrieved, err := repo.GetByID(ctx, mediaID)
				suite.Require().NoError(err)
				suite.Require().NotNil(retrieved)

				// Delete media
				err = repo.Delete(ctx, mediaID)
				suite.Require().NoError(err)

				// Verify media is deleted (should return error)
				_, err = repo.GetByID(ctx, mediaID)
				suite.Require().Error(err)
				suite.Equal(gorm.ErrRecordNotFound, err)
			},
		},
		{
			name: "returns_error_for_non_existent_media",
			testFunc: func(t *testing.T, dep *definition.DependancyOption) {
				ctx := context.Background()
				service, _ := suite.CreateService(t, dep)
				repo := repository.NewMediaRepository(service)

				// Try to retrieve non-existent media
				_, err := repo.GetByID(ctx, types.MediaID("non-existent-id"))
				suite.Require().Error(err)
				suite.Equal(gorm.ErrRecordNotFound, err)

				// Try to retrieve by non-existent hash
				_, err = repo.GetByHash(ctx, types.OwnerID("owner"), types.Base64Hash("non-existent-hash"))
				suite.Require().Error(err)
				suite.Equal(gorm.ErrRecordNotFound, err)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.WithTestDependancies(suite.T(), tc.testFunc)
		})
	}
}
