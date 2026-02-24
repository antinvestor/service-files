package repository_test

import (
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MultipartUploadRepositoryTestSuite struct {
	tests.BaseTestSuite
}

func TestMultipartUploadRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(MultipartUploadRepositoryTestSuite))
}

func (suite *MultipartUploadRepositoryTestSuite) TestCreateMultipartUpload() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)
		repo := res.MultipartUploadRepo

		expiresAt := time.Now().Add(24 * time.Hour)
		testUpload := &models.MultipartUpload{
			OwnerID:     "test-owner",
			MediaID:     "media-456",
			UploadName:  "test-file.jpg",
			ContentType: "image/jpeg",
			TotalSize:   10485760,
			PartSize:    5242880,
			PartCount:   2,
			UploadState: "initiated",
			ExpiresAt:   &expiresAt,
		}

		err := repo.Create(ctx, testUpload)
		assert.NoError(t, err)
		assert.NotEmpty(t, testUpload.ID)
	})
}

func (suite *MultipartUploadRepositoryTestSuite) TestUpdateState() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)
		repo := res.MultipartUploadRepo

		expiresAt := time.Now().Add(24 * time.Hour)
		testUpload := &models.MultipartUpload{
			OwnerID:     "test-owner",
			MediaID:     "media-abc",
			UploadName:  "test-file.jpg",
			ContentType: "image/jpeg",
			TotalSize:   10485760,
			PartSize:    5242880,
			PartCount:   2,
			UploadState: "initiated",
			ExpiresAt:   &expiresAt,
		}

		err := repo.Create(ctx, testUpload)
		require.NoError(t, err)

		uploadID := testUpload.ID
		err = repo.UpdateState(ctx, uploadID, "completed")
		assert.NoError(t, err)
	})
}

func (suite *MultipartUploadRepositoryTestSuite) TestHardDeleteByID() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)
		repo := res.MultipartUploadRepo

		expiresAt := time.Now().Add(24 * time.Hour)
		testUpload := &models.MultipartUpload{
			OwnerID:     "test-owner",
			MediaID:     "media-xyz",
			UploadName:  "test-file.jpg",
			ContentType: "image/jpeg",
			TotalSize:   10485760,
			PartSize:    5242880,
			PartCount:   2,
			UploadState: "initiated",
			ExpiresAt:   &expiresAt,
		}

		err := repo.Create(ctx, testUpload)
		require.NoError(t, err)

		uploadID := testUpload.ID
		err = repo.HardDeleteByID(ctx, uploadID)
		assert.NoError(t, err)
	})
}

func (suite *MultipartUploadRepositoryTestSuite) TestGetExpiredUploads() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)
		repo := res.MultipartUploadRepo

		now := time.Now()
		expiredTime := now.Add(-1 * time.Hour)
		futureTime := now.Add(1 * time.Hour)

		expiredUpload := &models.MultipartUpload{
			OwnerID:     "test-owner",
			MediaID:     "media-expired",
			UploadName:  "expired.jpg",
			ContentType: "image/jpeg",
			TotalSize:   5242880,
			PartSize:    5242880,
			PartCount:   1,
			UploadState: "completed",
			ExpiresAt:   &expiredTime,
		}

		futureUpload := &models.MultipartUpload{
			OwnerID:     "test-owner",
			MediaID:     "media-future",
			UploadName:  "future.jpg",
			ContentType: "image/jpeg",
			TotalSize:   5242880,
			PartSize:    5242880,
			PartCount:   1,
			UploadState: "completed",
			ExpiresAt:   &futureTime,
		}

		err := repo.Create(ctx, expiredUpload)
		require.NoError(t, err)
		err = repo.Create(ctx, futureUpload)
		require.NoError(t, err)

		results, err := repo.GetExpiredUploads(ctx, now)
		assert.NoError(t, err)
		assert.Len(t, results, 1)
	})
}
