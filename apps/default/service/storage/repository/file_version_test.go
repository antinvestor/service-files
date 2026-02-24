package repository_test

import (
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type FileVersionRepositoryTestSuite struct {
	tests.BaseTestSuite
}

func TestFileVersionRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(FileVersionRepositoryTestSuite))
}

func (suite *FileVersionRepositoryTestSuite) TestCreateFileVersion() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileVersionRepo

		testVersion := &models.FileVersion{
			MediaID:      "media-version",
			VersionNumber: 1,
			ContentHash:  "hash-abc123",
			FileSize:     1024,
			UploadName:   "file-v1.txt",
			ContentType:  "text/plain",
			StoragePath:  "/path/to/file",
			CreatedBy:    "test-user",
		}

		err := repo.Create(ctx, testVersion)
		assert.NoError(t, err)
		assert.NotEmpty(t, testVersion.ID)
	})
}

func (suite *FileVersionRepositoryTestSuite) TestGetByMediaID() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileVersionRepo

		mediaID := "get-by-media-versions"

		version1 := &models.FileVersion{
			MediaID:      mediaID,
			VersionNumber: 1,
			ContentHash:  "hash-v1",
			FileSize:     1024,
			UploadName:   "file-v1.txt",
			ContentType:  "text/plain",
			StoragePath:  "/path/to/file1",
			CreatedBy:    "test-user",
		}

		version2 := &models.FileVersion{
			MediaID:      mediaID,
			VersionNumber: 2,
			ContentHash:  "hash-v2",
			FileSize:     2048,
			UploadName:   "file-v2.txt",
			ContentType:  "text/plain",
			StoragePath:  "/path/to/file2",
			CreatedBy:    "test-user",
		}

		version3 := &models.FileVersion{
			MediaID:      mediaID,
			VersionNumber: 3,
			ContentHash:  "hash-v3",
			FileSize:     3072,
			UploadName:   "file-v3.txt",
			ContentType:  "text/plain",
			StoragePath:  "/path/to/file3",
			CreatedBy:    "test-user",
		}

		err := repo.Create(ctx, version1)
		require.NoError(t, err)
		err = repo.Create(ctx, version2)
		require.NoError(t, err)
		err = repo.Create(ctx, version3)
		require.NoError(t, err)

		results, err := repo.GetByMediaID(ctx, mediaID)
		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.Len(t, results, 3)

		assert.Equal(t, 3, results[0].VersionNumber)
		assert.Equal(t, 2, results[1].VersionNumber)
		assert.Equal(t, 1, results[2].VersionNumber)
	})
}

func (suite *FileVersionRepositoryTestSuite) TestGetVersion() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileVersionRepo

		mediaID := "get-specific-version"

		version1 := &models.FileVersion{
			MediaID:      mediaID,
			VersionNumber: 1,
			ContentHash:  "hash-v1",
			FileSize:     1024,
			UploadName:   "file-v1.txt",
			ContentType:  "text/plain",
			StoragePath:  "/path/to/file1",
			CreatedBy:    "test-user",
		}

		version2 := &models.FileVersion{
			MediaID:      mediaID,
			VersionNumber: 2,
			ContentHash:  "hash-v2",
			FileSize:     2048,
			UploadName:   "file-v2.txt",
			ContentType:  "text/plain",
			StoragePath:  "/path/to/file2",
			CreatedBy:    "test-user",
		}

		err := repo.Create(ctx, version1)
		require.NoError(t, err)
		err = repo.Create(ctx, version2)
		require.NoError(t, err)

		result, err := repo.GetVersion(ctx, mediaID, 2)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.VersionNumber)
	})
}

func (suite *FileVersionRepositoryTestSuite) TestGetVersionNotFound() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileVersionRepo

		result, err := repo.GetVersion(ctx, "non-existent-media", 5)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})
}

func (suite *FileVersionRepositoryTestSuite) TestGetVersionsPaginated() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileVersionRepo

		mediaID := "paginated-versions"

		for i := 1; i <= 5; i++ {
			version := &models.FileVersion{
				MediaID:      mediaID,
				VersionNumber: i,
				ContentHash: "hash-p" + string(rune('0'+i)),
				FileSize:     int64(1024 * i),
				UploadName:   "file-v" + string(rune('0'+i)) + ".txt",
				ContentType:  "text/plain",
				StoragePath:  "/path/to/file" + string(rune('0'+i)),
				CreatedBy:    "test-user",
			}
			err := repo.Create(ctx, version)
			require.NoError(t, err)
		}

		results, count, err := repo.GetVersionsPaginated(ctx, mediaID, 2, 0)
		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.Len(t, results, 2)
		assert.Equal(t, 5, count)

		assert.Equal(t, 5, results[0].VersionNumber)
		assert.Equal(t, 4, results[1].VersionNumber)

		results, count, err = repo.GetVersionsPaginated(ctx, mediaID, 2, 2)
		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.Len(t, results, 2)
		assert.Equal(t, 5, count)

		assert.Equal(t, 3, results[0].VersionNumber)
		assert.Equal(t, 2, results[1].VersionNumber)

		results, count, err = repo.GetVersionsPaginated(ctx, mediaID, 2, 4)
		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.Len(t, results, 1)
		assert.Equal(t, 5, count)
	})
}

func (suite *FileVersionRepositoryTestSuite) TestGetVersionsPaginatedNonExistent() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)

		repo := res.FileVersionRepo

		results, count, err := repo.GetVersionsPaginated(ctx, "non-existent-media", 10, 0)
		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.Len(t, results, 0)
		assert.Equal(t, 0, count)
	})
}
