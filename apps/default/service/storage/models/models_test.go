package models_test

import (
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ModelsTestSuite struct {
	tests.BaseTestSuite
}

func TestModelsTestSuite(t *testing.T) {
	suite.Run(t, new(ModelsTestSuite))
}

func (s *ModelsTestSuite) TestMediaMetadata_ToApi() {
	testCases := []struct {
		name     string
		model    *models.MediaMetadata
		expected *types.MediaMetadata
	}{
		{
			name: "basic_media_metadata_conversion",
			model: &models.MediaMetadata{
				BaseModel: data.BaseModel{ID: "test-id-123"},
				OwnerID:   "owner-123",
				Name:      "test-file.jpg",
				Size:      1024,
				OriginTs:  1640995200,
				Mimetype:  "image/jpeg",
				Hash:      "abc123hash",
			},
			expected: &types.MediaMetadata{
				MediaID:           types.MediaID("test-id-123"),
				ContentType:       types.ContentType("image/jpeg"),
				FileSizeBytes:     types.FileSizeBytes(1024),
				CreationTimestamp: uint64(1640995200),
				UploadName:        types.Filename("test-file.jpg"),
				Base64Hash:        types.Base64Hash("abc123hash"),
				OwnerID:           types.OwnerID("owner-123"),
			},
		},
		{
			name: "media_with_parent_and_thumbnail_properties",
			model: &models.MediaMetadata{
				BaseModel: data.BaseModel{ID: "thumb-id-456"},
				OwnerID:   "owner-456",
				ParentID:  "parent-id-789",
				Name:      "thumbnail.jpg",
				Size:      256,
				OriginTs:  1640995300,
				Mimetype:  "image/jpeg",
				Hash:      "thumb-hash-456",
				Properties: map[string]interface{}{
					"h": "100",
					"w": "150",
					"m": "crop",
				},
			},
			expected: &types.MediaMetadata{
				MediaID:           types.MediaID("thumb-id-456"),
				ContentType:       types.ContentType("image/jpeg"),
				FileSizeBytes:     types.FileSizeBytes(256),
				CreationTimestamp: uint64(1640995300),
				UploadName:        types.Filename("thumbnail.jpg"),
				Base64Hash:        types.Base64Hash("thumb-hash-456"),
				OwnerID:           types.OwnerID("owner-456"),
				ParentID:          types.MediaID("parent-id-789"),
				ThumbnailSize: &types.ThumbnailSize{
					Height:       100,
					Width:        150,
					ResizeMethod: "crop",
				},
			},
		},
		{
			name: "media_with_invalid_thumbnail_properties",
			model: &models.MediaMetadata{
				BaseModel: data.BaseModel{ID: "invalid-thumb-id"},
				OwnerID:   "owner-789",
				ParentID:  "parent-id-abc",
				Name:      "invalid-thumbnail.jpg",
				Size:      128,
				OriginTs:  1640995400,
				Mimetype:  "image/jpeg",
				Hash:      "invalid-thumb-hash",
				Properties: map[string]interface{}{
					"h": "invalid-height",
					"w": "invalid-width",
					"m": "scale",
				},
			},
			expected: &types.MediaMetadata{
				MediaID:           types.MediaID("invalid-thumb-id"),
				ContentType:       types.ContentType("image/jpeg"),
				FileSizeBytes:     types.FileSizeBytes(128),
				CreationTimestamp: uint64(1640995400),
				UploadName:        types.Filename("invalid-thumbnail.jpg"),
				Base64Hash:        types.Base64Hash("invalid-thumb-hash"),
				OwnerID:           types.OwnerID("owner-789"),
				ParentID:          types.MediaID("parent-id-abc"),
				ThumbnailSize: &types.ThumbnailSize{
					Height:       0,
					Width:        0,
					ResizeMethod: "scale",
				},
			},
		},
		{
			name: "media_without_parent_id",
			model: &models.MediaMetadata{
				BaseModel: data.BaseModel{ID: "no-parent-id"},
				OwnerID:   "owner-no-parent",
				Name:      "standalone-file.png",
				Size:      2048,
				OriginTs:  1640995500,
				Mimetype:  "image/png",
				Hash:      "standalone-hash",
			},
			expected: &types.MediaMetadata{
				MediaID:           types.MediaID("no-parent-id"),
				ContentType:       types.ContentType("image/png"),
				FileSizeBytes:     types.FileSizeBytes(2048),
				CreationTimestamp: uint64(1640995500),
				UploadName:        types.Filename("standalone-file.png"),
				Base64Hash:        types.Base64Hash("standalone-hash"),
				OwnerID:           types.OwnerID("owner-no-parent"),
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			result := tc.model.ToApi()

			require.NotNil(t, result)
			assert.Equal(t, tc.expected.MediaID, result.MediaID)
			assert.Equal(t, tc.expected.ContentType, result.ContentType)
			assert.Equal(t, tc.expected.FileSizeBytes, result.FileSizeBytes)
			assert.Equal(t, tc.expected.CreationTimestamp, result.CreationTimestamp)
			assert.Equal(t, tc.expected.UploadName, result.UploadName)
			assert.Equal(t, tc.expected.Base64Hash, result.Base64Hash)
			assert.Equal(t, tc.expected.OwnerID, result.OwnerID)
			assert.Equal(t, tc.expected.ParentID, result.ParentID)

			if tc.expected.ThumbnailSize != nil {
				require.NotNil(t, result.ThumbnailSize)
				assert.Equal(t, tc.expected.ThumbnailSize.Height, result.ThumbnailSize.Height)
				assert.Equal(t, tc.expected.ThumbnailSize.Width, result.ThumbnailSize.Width)
				assert.Equal(t, tc.expected.ThumbnailSize.ResizeMethod, result.ThumbnailSize.ResizeMethod)
			} else {
				assert.Nil(t, result.ThumbnailSize)
			}
		})
	}
}

func (s *ModelsTestSuite) TestMediaMetadata_Fill() {
	testCases := []struct {
		name     string
		input    *types.MediaMetadata
		expected *models.MediaMetadata
	}{
		{
			name: "basic_api_to_model_conversion",
			input: &types.MediaMetadata{
				MediaID:           types.MediaID("api-id-123"),
				ContentType:       types.ContentType("image/jpeg"),
				FileSizeBytes:     types.FileSizeBytes(1024),
				CreationTimestamp: uint64(1640995200),
				UploadName:        types.Filename("api-file.jpg"),
				Base64Hash:        types.Base64Hash("api-hash-123"),
				OwnerID:           types.OwnerID("api-owner-123"),
			},
			expected: &models.MediaMetadata{
				OwnerID:  "api-owner-123",
				Name:     "api-file.jpg",
				Size:     1024,
				OriginTs: 1640995200,
				Mimetype: "image/jpeg",
				Hash:     "api-hash-123",
			},
		},
		{
			name: "api_with_parent_and_thumbnail_size",
			input: &types.MediaMetadata{
				MediaID:           types.MediaID("api-thumb-id"),
				ContentType:       types.ContentType("image/png"),
				FileSizeBytes:     types.FileSizeBytes(512),
				CreationTimestamp: uint64(1640995300),
				UploadName:        types.Filename("api-thumbnail.png"),
				Base64Hash:        types.Base64Hash("api-thumb-hash"),
				OwnerID:           types.OwnerID("api-owner-456"),
				ParentID:          types.MediaID("api-parent-id"),
				ThumbnailSize: &types.ThumbnailSize{
					Height:       200,
					Width:        300,
					ResizeMethod: "fit",
				},
			},
			expected: &models.MediaMetadata{
				OwnerID:  "api-owner-456",
				ParentID: "api-parent-id",
				Name:     "api-thumbnail.png",
				Size:     512,
				OriginTs: 1640995300,
				Mimetype: "image/png",
				Hash:     "api-thumb-hash",
				Properties: map[string]interface{}{
					"h": "200",
					"w": "300",
					"m": "fit",
				},
			},
		},
		{
			name: "api_without_thumbnail_size",
			input: &types.MediaMetadata{
				MediaID:           types.MediaID("api-no-thumb"),
				ContentType:       types.ContentType("video/mp4"),
				FileSizeBytes:     types.FileSizeBytes(10240),
				CreationTimestamp: uint64(1640995400),
				UploadName:        types.Filename("video.mp4"),
				Base64Hash:        types.Base64Hash("video-hash"),
				OwnerID:           types.OwnerID("video-owner"),
			},
			expected: &models.MediaMetadata{
				OwnerID:  "video-owner",
				Name:     "video.mp4",
				Size:     10240,
				OriginTs: 1640995400,
				Mimetype: "video/mp4",
				Hash:     "video-hash",
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			model := &models.MediaMetadata{}
			model.Fill(tc.input)

			assert.Equal(t, tc.expected.OwnerID, model.OwnerID)
			assert.Equal(t, tc.expected.ParentID, model.ParentID)
			assert.Equal(t, tc.expected.Name, model.Name)
			assert.Equal(t, tc.expected.Size, model.Size)
			assert.Equal(t, tc.expected.OriginTs, model.OriginTs)
			assert.Equal(t, tc.expected.Mimetype, model.Mimetype)
			assert.Equal(t, tc.expected.Hash, model.Hash)

			if tc.expected.Properties != nil {
				require.NotNil(t, model.Properties)
				assert.Equal(t, tc.expected.Properties["h"], model.Properties["h"])
				assert.Equal(t, tc.expected.Properties["w"], model.Properties["w"])
				assert.Equal(t, tc.expected.Properties["m"], model.Properties["m"])
			}
		})
	}
}

func (s *ModelsTestSuite) TestMediaMetadata_RoundTripConversion() {
	testCases := []struct {
		name     string
		original *models.MediaMetadata
	}{
		{
			name: "round_trip",
			original: &models.MediaMetadata{
				BaseModel: data.BaseModel{ID: "round-trip-id"},
				OwnerID:   "round-trip-owner",
				ParentID:  "round-trip-parent",
				Name:      "round-trip.jpg",
				Size:      4096,
				OriginTs:  1640995600,
				Mimetype:  "image/jpeg",
				Hash:      "round-trip-hash",
				Properties: map[string]interface{}{
					"h": "400",
					"w": "600",
					"m": "crop",
				},
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			apiModel := tc.original.ToApi()
			require.NotNil(t, apiModel)

			backToModel := &models.MediaMetadata{}
			backToModel.Fill(apiModel)

			assert.Equal(t, tc.original.OwnerID, backToModel.OwnerID)
			assert.Equal(t, tc.original.ParentID, backToModel.ParentID)
			assert.Equal(t, tc.original.Name, backToModel.Name)
			assert.Equal(t, tc.original.Size, backToModel.Size)
			assert.Equal(t, tc.original.OriginTs, backToModel.OriginTs)
			assert.Equal(t, tc.original.Mimetype, backToModel.Mimetype)
			assert.Equal(t, tc.original.Hash, backToModel.Hash)

			require.NotNil(t, backToModel.Properties)
			assert.Equal(t, "400", backToModel.Properties["h"])
			assert.Equal(t, "600", backToModel.Properties["w"])
			assert.Equal(t, "crop", backToModel.Properties["m"])
		})
	}
}

func (s *ModelsTestSuite) TestMediaMetadata_EmptyValues() {
	testCases := []struct {
		name  string
		model *models.MediaMetadata
	}{
		{
			name:  "empty_values",
			model: &models.MediaMetadata{},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			apiModel := tc.model.ToApi()
			require.NotNil(t, apiModel)

			assert.Empty(t, apiModel.MediaID)
			assert.Empty(t, apiModel.OwnerID)
			assert.Empty(t, apiModel.ParentID)
			assert.Nil(t, apiModel.ThumbnailSize)
		})
	}
}

func (s *ModelsTestSuite) TestMediaAudit_StructureAndFields() {
	testCases := []struct {
		name  string
		audit *models.MediaAudit
	}{
		{
			name: "audit_fields",
			audit: &models.MediaAudit{
				BaseModel: data.BaseModel{ID: "audit-id-123"},
				FileID:    "file-id-456",
				Action:    "download",
				Source:    "web-client",
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			assert.Equal(t, "audit-id-123", tc.audit.GetID())
			assert.Equal(t, "file-id-456", tc.audit.FileID)
			assert.Equal(t, "download", tc.audit.Action)
			assert.Equal(t, "web-client", tc.audit.Source)
		})
	}
}

func (s *ModelsTestSuite) TestMediaMetadata_EncryptionRoundTrip() {
	testCases := []struct {
		name        string
		model       *models.MediaMetadata
		input       *types.MediaMetadata
		expectNil   bool
		expectChunk int
	}{
		{
			name: "fill_writes_encryption_properties",
			input: &types.MediaMetadata{
				MediaID:           "enc-media",
				ContentType:       "application/octet-stream",
				FileSizeBytes:     12,
				CreationTimestamp: 100,
				UploadName:        "enc.bin",
				Base64Hash:        "abcde12345",
				OwnerID:           "owner",
				Encryption: &types.EncryptionInfo{
					Version:         1,
					Algorithm:       "aes-256-gcm",
					ChunkSizeBytes:  65536,
					WrappedKey:      "wk",
					WrappedKeyNonce: "wn",
					NoncePrefix:     "np",
				},
			},
			expectNil:   false,
			expectChunk: 65536,
		},
		{
			name: "to_api_ignores_invalid_encryption_version",
			model: &models.MediaMetadata{
				BaseModel: data.BaseModel{ID: "enc-media-2"},
				Properties: map[string]interface{}{
					"enc_v": "bad",
				},
			},
			expectNil: true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			if tc.input != nil {
				model := &models.MediaMetadata{}
				model.Fill(tc.input)
				api := model.ToApi()
				if tc.expectNil {
					assert.Nil(t, api.Encryption)
					return
				}
				require.NotNil(t, api.Encryption)
				assert.Equal(t, tc.expectChunk, api.Encryption.ChunkSizeBytes)
				assert.Equal(t, "aes-256-gcm", api.Encryption.Algorithm)
				assert.Equal(t, "wk", api.Encryption.WrappedKey)
				return
			}

			api := tc.model.ToApi()
			if tc.expectNil {
				assert.Nil(t, api.Encryption)
			} else {
				require.NotNil(t, api.Encryption)
			}
		})
	}
}
