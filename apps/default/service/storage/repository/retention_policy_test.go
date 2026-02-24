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

type RetentionPolicyRepositoryTestSuite struct {
	tests.BaseTestSuite
}

func TestRetentionPolicyRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(RetentionPolicyRepositoryTestSuite))
}

func (suite *RetentionPolicyRepositoryTestSuite) TestCreateRetentionPolicy() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.RetentionPolicyRepo
		testPolicy := &models.RetentionPolicy{
			Name:          "Standard Retention",
			Description:   "Keep files for 30 days",
			RetentionDays: 30,
			IsDefault:     false,
			IsSystem:      false,
			OwnerID:       "test-owner",
		}

		err := repo.Create(ctx, testPolicy)
		assert.NoError(t, err)
		assert.NotEmpty(t, testPolicy.ID)
	})
}

func (suite *RetentionPolicyRepositoryTestSuite) TestGetByID() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.RetentionPolicyRepo

		testPolicy := &models.RetentionPolicy{
			Name:          "Test Policy",
			Description:   "For testing",
			RetentionDays: 7,
			IsDefault:     false,
			IsSystem:      false,
			OwnerID:       "test-owner",
		}

		err := repo.Create(ctx, testPolicy)
		require.NoError(t, err)

		result, err := repo.GetByID(ctx, testPolicy.ID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Test Policy", result.Name)
		assert.Equal(t, 7, result.RetentionDays)
	})
}

func (suite *RetentionPolicyRepositoryTestSuite) TestGetByIDNotFound() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.RetentionPolicyRepo

		result, err := repo.GetByID(ctx, "non-existent-policy")
		assert.NoError(t, err)
		assert.Nil(t, result)
	})
}

func (suite *RetentionPolicyRepositoryTestSuite) TestGetDefault() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.RetentionPolicyRepo

		testPolicy := &models.RetentionPolicy{
			Name:          "Default Policy",
			Description:   "Default retention",
			RetentionDays: 365,
			IsDefault:     true,
			IsSystem:      true,
			OwnerID:       "",
		}

		err := repo.Create(ctx, testPolicy)
		require.NoError(t, err)

		result, err := repo.GetDefault(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Default Policy", result.Name)
		assert.Equal(t, 365, result.RetentionDays)
		assert.True(t, result.IsDefault)
		assert.True(t, result.IsSystem)
	})
}

func (suite *RetentionPolicyRepositoryTestSuite) TestListByOwner() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.RetentionPolicyRepo

		ownerID := "list-policies-owner"

		for i := 1; i <= 3; i++ {
			policy := &models.RetentionPolicy{
				Name:          "Policy " + string(rune('0'+i)),
				Description:   "Test description " + string(rune('0'+i)),
				RetentionDays: i * 10,
				IsDefault:     i == 1,
				IsSystem:      false,
				OwnerID:       ownerID,
			}

			err := repo.Create(ctx, policy)
			require.NoError(t, err)
		}

		results, count, err := repo.ListByOwner(ctx, ownerID, 10, 0)
		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.Len(t, results, 3)
		assert.Equal(t, 3, count)
	})
}

func (suite *RetentionPolicyRepositoryTestSuite) TestDelete() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.RetentionPolicyRepo

		testPolicy := &models.RetentionPolicy{
			Name:          "Delete Test Policy",
			Description:   "Policy to delete",
			RetentionDays: 90,
			IsDefault:     false,
			IsSystem:      false,
			OwnerID:       "test-owner",
		}

		err := repo.Create(ctx, testPolicy)
		require.NoError(t, err)

		err = repo.Delete(ctx, testPolicy.ID)
		assert.NoError(t, err)

		result, err := repo.GetByID(ctx, testPolicy.ID)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})
}

type FileRetentionRepositoryTestSuite struct {
	tests.BaseTestSuite
}

func TestFileRetentionRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(FileRetentionRepositoryTestSuite))
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func (suite *FileRetentionRepositoryTestSuite) TestCreateFileRetention() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileRetentionRepo

		now := time.Now()
		expiresAt := now.Add(30 * 24 * time.Hour)

		testRetention := &models.FileRetention{
			MediaID:   "retain-media",
			PolicyID:  "policy-123",
			AppliedAt: now,
			ExpiresAt: &expiresAt,
			IsLocked:  false,
		}

		err := repo.Create(ctx, testRetention)
		assert.NoError(t, err)
		assert.NotEmpty(t, testRetention.ID)
	})
}

func (suite *FileRetentionRepositoryTestSuite) TestGetByMediaID() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileRetentionRepo

		mediaID := "get-retention-media"

		testRetention := &models.FileRetention{
			MediaID:   mediaID,
			PolicyID:  "policy-456",
			AppliedAt: time.Now(),
			ExpiresAt: timePtr(time.Now().Add(7 * 24 * time.Hour)),
			IsLocked:  false,
		}

		err := repo.Create(ctx, testRetention)
		require.NoError(t, err)

		result, err := repo.GetByMediaID(ctx, mediaID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, mediaID, result.MediaID)
		assert.Equal(t, "policy-456", result.PolicyID)
	})
}

func (suite *FileRetentionRepositoryTestSuite) TestUpdateLocked() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileRetentionRepo

		mediaID := "lock-media"

		testRetention := &models.FileRetention{
			MediaID:   mediaID,
			PolicyID:  "policy-789",
			AppliedAt: time.Now(),
			ExpiresAt: timePtr(time.Now().Add(7 * 24 * time.Hour)),
			IsLocked:  false,
		}

		err := repo.Create(ctx, testRetention)
		require.NoError(t, err)

		err = repo.UpdateLocked(ctx, mediaID, true)
		assert.NoError(t, err)

		retentionID := testRetention.GetID()
		result, err := repo.GetByID(ctx, retentionID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.IsLocked)
	})
}

func (suite *FileRetentionRepositoryTestSuite) TestDeleteByMediaID() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileRetentionRepo

		mediaID := "delete-retention-media"

		testRetention := &models.FileRetention{
			MediaID:   mediaID,
			PolicyID:  "policy-delete",
			AppliedAt: time.Now(),
			ExpiresAt: timePtr(time.Now().Add(7 * 24 * time.Hour)),
			IsLocked:  false,
		}

		err := repo.Create(ctx, testRetention)
		require.NoError(t, err)

		retentionID := testRetention.GetID()
		err = repo.DeleteByMediaID(ctx, mediaID)
		assert.NoError(t, err)

		_, err = repo.GetByID(ctx, retentionID)
		assert.Error(t, err)
	})
}

func (suite *FileRetentionRepositoryTestSuite) TestGetExpired() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileRetentionRepo

		now := time.Now()
		expiredTime := now.Add(-1 * 24 * time.Hour)
		futureTime := now.Add(1 * 24 * time.Hour)

		expiredRetention := &models.FileRetention{
			MediaID:   "expired-media",
			PolicyID:  "policy-expired",
			AppliedAt: time.Now(),
			ExpiresAt: &expiredTime,
			IsLocked:  false,
		}

		futureRetention := &models.FileRetention{
			MediaID:   "future-media",
			PolicyID:  "policy-future",
			AppliedAt: time.Now(),
			ExpiresAt: &futureTime,
			IsLocked:  false,
		}

		err := repo.Create(ctx, expiredRetention)
		require.NoError(t, err)
		err = repo.Create(ctx, futureRetention)
		require.NoError(t, err)

		results, err := repo.GetExpired(ctx, now)
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "expired-media", results[0].MediaID)
	})
}
