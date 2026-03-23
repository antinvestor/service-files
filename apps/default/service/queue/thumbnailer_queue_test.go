package queue

import (
	"encoding/json"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ThumbnailQueueTestSuite struct {
	tests.BaseTestSuite
}

func TestThumbnailQueueTestSuite(t *testing.T) {
	suite.Run(t, new(ThumbnailQueueTestSuite))
}

func (suite *ThumbnailQueueTestSuite) TestHandle() {
	testCases := []struct {
		name      string
		payload   []byte
		setupMeta *types.MediaMetadata
		wantErr   bool
	}{
		{
			name:    "invalid_json_payload",
			payload: []byte("{invalid"),
			wantErr: true,
		},
		{
			name: "missing_media_id",
			payload: mustJSON(map[string]string{
				"other": "value",
			}),
			wantErr: false,
		},
		{
			name: "non_image_media_noop",
			payload: mustJSON(map[string]string{
				"media_id": "media-text-1",
			}),
			setupMeta: &types.MediaMetadata{
				MediaID:       "media-text-1",
				OwnerID:       "owner",
				UploadName:    "doc.txt",
				ContentType:   "text/plain",
				Base64Hash:    "abc12345",
				FileSizeBytes: 10,
				ServerName:    "service_file",
			},
			wantErr: false,
		},
		{
			name: "image_media_generation_error_is_swallowed",
			payload: mustJSON(map[string]string{
				"media_id": "media-image-1",
			}),
			setupMeta: &types.MediaMetadata{
				MediaID:       "media-image-1",
				OwnerID:       "owner",
				UploadName:    "img.jpg",
				ContentType:   "image/jpeg",
				Base64Hash:    "abc12345",
				FileSizeBytes: 10,
				ServerName:    "service_file",
			},
			wantErr: false,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res := suite.CreateService(t, dep)
		cfg := svc.Config().(*config.FilesConfig)
		storageProvider, err := provider.GetStorageProvider(ctx, cfg)
		require.NoError(t, err)

		db, err := connection.NewMediaDatabase(
			svc.WorkManager(),
			res.MediaRepository,
			res.MultipartUploadRepo,
			res.MultipartUploadPartRepo,
			res.FileVersionRepo,
			res.RetentionPolicyRepo,
			res.FileRetentionRepo,
			res.StorageStatsRepo,
		)
		require.NoError(t, err)

		handler := NewThumbnailQueueHandler(svc, db, storageProvider)

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.setupMeta != nil {
					require.NoError(t, db.StoreMediaMetadata(ctx, tc.setupMeta))
				}

				err := handler.Handle(ctx, map[string]string{}, tc.payload)
				if tc.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
			})
		}
	})
}

func mustJSON(v map[string]string) []byte {
	b, _ := json.Marshal(v)
	return b
}
