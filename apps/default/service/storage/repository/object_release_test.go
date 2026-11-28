package repository_test

import (
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/pitabwire/frame/v3/data"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type ObjectReleaseTestSuite struct {
	tests.BaseTestSuite
}

func TestObjectReleaseTestSuite(t *testing.T) {
	suite.Run(t, new(ObjectReleaseTestSuite))
}

// The reference count must see every tenant's rows even when the connection
// the transaction runs on carries another principal's session-level RLS
// scope, as happens when a transaction pooler hands out a server connection
// last bound for a different client.
func (suite *ObjectReleaseTestSuite) TestLockObjectGlobally_UnscopesTheTransaction() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)
		repo := res.MediaRepository
		ctxA := tenantCtx(ctx, "tenant-rel-a", "partition-rel-a")
		ctxB := tenantCtx(ctx, "tenant-rel-b", "partition-rel-b")

		key := "a/b/" + util.IDString() + "/file"
		other := &models.MediaMetadata{
			OwnerID: "owner-b", Hash: "hash-rel", Public: true,
			BucketName: "public-bkt", ObjectKey: key, Properties: data.JSONMap{},
		}
		require.NoError(t, repo.Create(ctxB, other))

		count := func(tx *gorm.DB) int64 {
			var n int64
			require.NoError(t, tx.Model(&models.MediaMetadata{}).
				Where("bucket_name = ? AND object_key = ?", "public-bkt", key).Count(&n).Error)
			return n
		}
		ref := repository.ObjectRef{Bucket: "public-bkt", Key: key}

		// Precondition: a transaction bound to tenant A cannot see B's row.
		require.NoError(t, repo.Pool().DB(ctxA, false).Transaction(func(tx *gorm.DB) error {
			require.Zero(t, count(tx), "RLS must hide tenant B's row from tenant A")
			return nil
		}))

		require.NoError(t, repo.Pool().DB(ctxA, false).Transaction(func(tx *gorm.DB) error {
			require.NoError(t, repository.LockObjectGlobally(tx, ref))
			require.Equal(t, int64(1), count(tx), "after unscoping, the count must include every tenant's references")
			return nil
		}))

		// The unscoping is transaction-local: the connection's scope is back
		// for the next transaction.
		require.NoError(t, repo.Pool().DB(ctxA, false).Transaction(func(tx *gorm.DB) error {
			require.Zero(t, count(tx), "unscoping must not leak past its transaction")
			return nil
		}))
	})
}
