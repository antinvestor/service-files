package models

import (
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaMetadata_ToApi(t *testing.T) {
	testCases := []struct {
		name     string
		model    *MediaMetadata
		expected *types.MediaMetadata
	}{
		{
			name: "basic_media_metadata_conversion",
			model: &MediaMetadata{
				BaseModel: frame.BaseModel{ID: "test-id-123"},
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
			model: &MediaMetadata{
				BaseModel: frame.BaseModel{ID: "thumb-id-456"},
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
			model: &MediaMetadata{
				BaseModel: frame.BaseModel{ID: "invalid-thumb-id"},
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
					Height:       0, // Invalid conversion results in 0
					Width:        0, // Invalid conversion results in 0
					ResizeMethod: "scale",
				},
			},
		},
		{
			name: "media_without_parent_id",
			model: &MediaMetadata{
				BaseModel: frame.BaseModel{ID: "no-parent-id"},
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
				// ParentID should be empty
				// ThumbnailSize should be nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

func TestMediaMetadata_Fill(t *testing.T) {
	testCases := []struct {
		name     string
		input    *types.MediaMetadata
		expected *MediaMetadata
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
			expected: &MediaMetadata{
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
			expected: &MediaMetadata{
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
			expected: &MediaMetadata{
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
		t.Run(tc.name, func(t *testing.T) {
			model := &MediaMetadata{}
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

func TestMediaMetadata_RoundTripConversion(t *testing.T) {
	// Test that converting from model to API and back preserves data
	original := &MediaMetadata{
		BaseModel: frame.BaseModel{ID: "round-trip-id"},
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
	}

	// Convert to API
	apiModel := original.ToApi()
	require.NotNil(t, apiModel)

	// Convert back to model
	backToModel := &MediaMetadata{}
	backToModel.Fill(apiModel)

	// Verify key fields are preserved
	assert.Equal(t, original.OwnerID, backToModel.OwnerID)
	assert.Equal(t, original.ParentID, backToModel.ParentID)
	assert.Equal(t, original.Name, backToModel.Name)
	assert.Equal(t, original.Size, backToModel.Size)
	assert.Equal(t, original.OriginTs, backToModel.OriginTs)
	assert.Equal(t, original.Mimetype, backToModel.Mimetype)
	assert.Equal(t, original.Hash, backToModel.Hash)

	// Verify properties are preserved
	require.NotNil(t, backToModel.Properties)
	assert.Equal(t, "400", backToModel.Properties["h"])
	assert.Equal(t, "600", backToModel.Properties["w"])
	assert.Equal(t, "crop", backToModel.Properties["m"])
}

func TestMediaMetadata_EmptyValues(t *testing.T) {
	// Test behaviour with empty/nil values
	model := &MediaMetadata{}

	apiModel := model.ToApi()
	require.NotNil(t, apiModel)

	// Should handle empty values gracefully
	assert.Empty(t, apiModel.MediaID)
	assert.Empty(t, apiModel.OwnerID)
	assert.Empty(t, apiModel.ParentID)
	assert.Nil(t, apiModel.ThumbnailSize)
}

func TestMediaAudit_StructureAndFields(t *testing.T) {
	// Test that MediaAudit struct can be created and fields are accessible
	audit := &MediaAudit{
		BaseModel: frame.BaseModel{ID: "audit-id-123"},
		FileID:    "file-id-456",
		AccessID:  "access-id-789",
		Action:    "download",
		Source:    "web-client",
	}

	assert.Equal(t, "audit-id-123", audit.GetID())
	assert.Equal(t, "file-id-456", audit.FileID)
	assert.Equal(t, "access-id-789", audit.AccessID)
	assert.Equal(t, "download", audit.Action)
	assert.Equal(t, "web-client", audit.Source)
}
