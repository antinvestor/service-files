package connection_test

import (
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
)

// File versions record only a content hash, not the bucket, object key or
// encryption parameters a row now needs to locate and decrypt its content.
// Restoring one would point the row at bytes it cannot read, so restore is
// refused and the row is left as it was.
func (suite *DatastoreTestSuite) TestRestoreMediaToVersion_IsRefusedAndLeavesRowUntouched() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res := suite.CreateService(t, dep)
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: res.MediaRepository,
			FileVersionRepo: res.FileVersionRepo,
		}

		mediaID := types.MediaID(util.IDString())
		require.NoError(t, db.StoreMediaMetadata(ctx, &types.MediaMetadata{
			MediaID: mediaID, Base64Hash: "currenthash", FileSizeBytes: 3, OwnerID: "@o:example.com",
			BucketName: "private-bkt", ObjectKey: "c/u/rrenthash/obj1",
			Encryption: &types.EncryptionInfo{Version: 1, Algorithm: "AES-256-GCM"},
		}))
		require.NoError(t, res.FileVersionRepo.Create(ctx, &models.FileVersion{
			MediaID: string(mediaID), VersionNumber: 1, ContentHash: "oldhash", FileSize: 9, StoragePath: "oldhash",
		}))

		_, err := db.RestoreMediaToVersion(ctx, string(mediaID), 1, "@o:example.com")
		require.ErrorIs(t, err, connection.ErrVersionRestoreUnsupported)

		after, err := db.GetMediaMetadata(ctx, mediaID)
		require.NoError(t, err)
		require.Equal(t, types.Base64Hash("currenthash"), after.Base64Hash)
		require.Equal(t, "c/u/rrenthash/obj1", after.ObjectKey)
		require.Equal(t, "private-bkt", after.BucketName)
		require.NotNil(t, after.Encryption)
	})
}
