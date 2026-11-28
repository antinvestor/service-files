package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pitabwire/frame/v3/data"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/pitabwire/frame/v3/security"
	"github.com/stretchr/testify/require"
)

func tenancyCtx(ctx context.Context, profileID, tenantID, partitionID string, roles ...string) context.Context {
	return (&security.AuthenticationClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: profileID},
		ProfileID:        profileID,
		TenantID:         tenantID,
		PartitionID:      partitionID,
		ServiceName:      "service_authentication",
		Roles:            roles,
	}).ClaimsToContext(ctx)
}

// Hash dedupe must never hand back media stored in another partition: an
// internal service (which bypasses row-level tenancy) uploading the same bytes
// into a second partition would otherwise get the first partition's media ID.
func (suite *MediaRepositoryTestSuite) TestGetByHashScopedToCallerPartition() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)
		repo := res.MediaRepository

		const owner, hash = "sa-dedupe-owner", "dedupe-hash-1"
		ctxA := tenancyCtx(ctx, owner, "dedupe-tenant-a", "dedupe-partition-a", "internal")
		ctxB := tenancyCtx(ctx, owner, "dedupe-tenant-b", "dedupe-partition-b", "internal")

		media := &models.MediaMetadata{
			OwnerID: owner, Name: "avatar.png", Ext: "png", Size: 10,
			OriginTs: time.Now().Unix(), Mimetype: "image/png", Hash: hash,
			BucketName: "test-bucket", Provider: "local", Properties: data.JSONMap{},
		}
		require.NoError(t, repo.Create(ctxA, media))

		found, err := repo.GetByHash(ctxA, types.OwnerID(owner), types.Base64Hash(hash))
		require.NoError(t, err)
		require.Equal(t, media.GetID(), found.GetID())

		other, err := repo.GetByHash(ctxB, types.OwnerID(owner), types.Base64Hash(hash))
		require.Error(t, err)
		require.Nil(t, other)
	})
}
