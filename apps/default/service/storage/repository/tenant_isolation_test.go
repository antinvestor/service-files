package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// TenantIsolationTestSuite exercises Postgres row level security across
// tenants. The base suite wires frametests/rlstest so application queries
// run as a non-superuser role — without that, the testcontainer superuser
// bypasses FORCE ROW LEVEL SECURITY and these assertions would pass
// vacuously even if the policies were broken.
type TenantIsolationTestSuite struct {
	tests.BaseTestSuite
}

func TestTenantIsolationTestSuite(t *testing.T) {
	suite.Run(t, new(TenantIsolationTestSuite))
}

// tenantCtx derives a context carrying authentication claims for the given
// tenant/partition on top of the service context, so repository queries run
// tenancy-scoped under RLS.
func tenantCtx(ctx context.Context, tenantID, partitionID string) context.Context {
	claims := &security.AuthenticationClaims{
		TenantID:    tenantID,
		PartitionID: partitionID,
		AccessID:    util.IDString(),
	}
	claims.Subject = "user-" + tenantID
	return claims.ClaimsToContext(ctx)
}

func (suite *TenantIsolationTestSuite) TestMediaMetadata_CrossTenantReadsReturnNothing() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)
		repo := res.MediaRepository

		ctxA := tenantCtx(ctx, "tenant-iso-a", "partition-iso-a")
		ctxB := tenantCtx(ctx, "tenant-iso-b", "partition-iso-b")

		media := &models.MediaMetadata{
			OwnerID:    "owner-iso-a",
			Name:       "tenant-a-secret.pdf",
			Ext:        "pdf",
			Size:       512,
			OriginTs:   time.Now().Unix(),
			Mimetype:   "application/pdf",
			Hash:       "iso-hash-a",
			BucketName: "test-bucket",
			Provider:   "local",
			Properties: data.JSONMap{},
		}
		require.NoError(t, repo.Create(ctxA, media))
		require.NotEmpty(t, media.ID)
		require.Equal(t, "tenant-iso-a", media.TenantID,
			"row must be stamped with the creating tenant")

		// Tenant A sees its own row through the restricted role.
		own, err := repo.GetByID(ctxA, media.ID)
		require.NoError(t, err)
		require.Equal(t, media.ID, own.ID)

		// Tenant B must not see tenant A's row by any lookup path.
		_, err = repo.GetByID(ctxB, media.ID)
		require.Error(t, err, "cross-tenant GetByID must not return tenant A's media")

		_, err = repo.GetByHash(ctxB, types.OwnerID(media.OwnerID), types.Base64Hash(media.Hash))
		require.Error(t, err, "cross-tenant GetByHash must not return tenant A's media")

		listed, err := repo.GetByOwnerID(ctxB, types.OwnerID(media.OwnerID), "", 0, 10)
		require.NoError(t, err)
		assert.Empty(t, listed, "cross-tenant list must not contain tenant A's media")

		// A claim-less context keeps match-all semantics (system path).
		all, err := repo.GetByID(ctx, media.ID)
		require.NoError(t, err)
		assert.Equal(t, media.ID, all.ID)
	})
}

func (suite *TenantIsolationTestSuite) TestFileVersion_CrossTenantReadsReturnNothing() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)
		repo := res.FileVersionRepo

		ctxA := tenantCtx(ctx, "tenant-iso-a", "partition-iso-a")
		ctxB := tenantCtx(ctx, "tenant-iso-b", "partition-iso-b")

		version := &models.FileVersion{
			MediaID:       "media-iso-a",
			VersionNumber: 1,
			ContentHash:   "version-hash-a",
			FileSize:      2048,
			UploadName:    "tenant-a-versioned.txt",
			ContentType:   "text/plain",
			StoragePath:   "/tenant-a/media-iso-a/v1",
			CreatedBy:     "user-tenant-iso-a",
		}
		require.NoError(t, repo.Create(ctxA, version))
		require.Equal(t, "tenant-iso-a", version.TenantID,
			"row must be stamped with the creating tenant")

		// Tenant A sees its own version.
		ownVersions, err := repo.GetByMediaID(ctxA, version.MediaID)
		require.NoError(t, err)
		require.Len(t, ownVersions, 1)

		// Tenant B sees nothing for the same media.
		crossVersions, err := repo.GetByMediaID(ctxB, version.MediaID)
		require.NoError(t, err)
		assert.Empty(t, crossVersions, "cross-tenant version list must be empty")

		crossPaginated, total, err := repo.GetVersionsPaginated(ctxB, version.MediaID, 10, 0)
		require.NoError(t, err)
		assert.Empty(t, crossPaginated)
		assert.Zero(t, total, "cross-tenant version count must be zero")

		// A claim-less context keeps match-all semantics (system path).
		allVersions, err := repo.GetByMediaID(ctx, version.MediaID)
		require.NoError(t, err)
		assert.Len(t, allVersions, 1)
	})
}
