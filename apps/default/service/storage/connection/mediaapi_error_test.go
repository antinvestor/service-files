package connection_test

import (
	"context"
	"errors"
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/stretchr/testify/require"
)

var errLookupFailed = errors.New("connection reset")

// failingMediaRepo returns (nil, err) for lookups, as a repository does on any
// database error other than not-found. Unused methods are left to the nil
// embedded interface.
type failingMediaRepo struct {
	repository.MediaRepository
}

func (failingMediaRepo) GetByID(context.Context, string) (*models.MediaMetadata, error) {
	return nil, errLookupFailed
}

func (failingMediaRepo) GetByHash(context.Context, types.OwnerID, types.Base64Hash) (*models.MediaMetadata, error) {
	return nil, errLookupFailed
}

func TestMediaMetadataLookupErrorsDoNotPanic(t *testing.T) {
	db := &connection.Database{MediaRepository: failingMediaRepo{}}

	t.Run("by_id", func(t *testing.T) {
		var (
			got *types.MediaMetadata
			err error
		)
		require.NotPanics(t, func() { got, err = db.GetMediaMetadata(t.Context(), "media-1") })
		require.ErrorIs(t, err, errLookupFailed)
		require.Nil(t, got)
	})

	t.Run("by_hash", func(t *testing.T) {
		var (
			got *types.MediaMetadata
			err error
		)
		require.NotPanics(t, func() { got, err = db.GetMediaMetadataByHash(t.Context(), "owner", "hash") })
		require.ErrorIs(t, err, errLookupFailed)
		require.Nil(t, got)
	})
}
