package storage

import (
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/blobprovider"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ObjectKeyTestSuite struct {
	suite.Suite
}

func TestObjectKeyTestSuite(t *testing.T) {
	suite.Run(t, new(ObjectKeyTestSuite))
}

func (s *ObjectKeyTestSuite) TestContentObjectKey_IsStableAndRelative() {
	const hash = types.Base64Hash("Ab3-_xyzQ")
	testCases := []struct {
		name    string
		meta    *types.MediaMetadata
		wantKey string
		wantErr bool
	}{
		{
			name:    "plain_content_is_content_addressed",
			meta:    &types.MediaMetadata{MediaID: "m1", Base64Hash: hash},
			wantKey: "A/b/3-_xyzQ/file",
		},
		{
			// Each encrypted upload carries its own data key, so two rows with
			// the same plaintext hash cannot share one ciphertext object.
			name:    "encrypted_content_is_keyed_per_media",
			meta:    &types.MediaMetadata{MediaID: "m1", Base64Hash: hash, Encryption: &types.EncryptionInfo{Version: 1}},
			wantKey: "A/b/3-_xyzQ/m1",
		},
		{name: "hash_too_short", meta: &types.MediaMetadata{MediaID: "m1", Base64Hash: "ab"}, wantErr: true},
		{name: "hash_with_path_separator", meta: &types.MediaMetadata{MediaID: "m1", Base64Hash: "ab/../../etc"}, wantErr: true},
		{name: "hash_with_dot_segments", meta: &types.MediaMetadata{MediaID: "m1", Base64Hash: "a..b"}, wantErr: true},
		{
			name:    "encrypted_without_media_id",
			meta:    &types.MediaMetadata{Base64Hash: hash, Encryption: &types.EncryptionInfo{Version: 1}},
			wantErr: true,
		},
		{
			name:    "encrypted_with_unsafe_media_id",
			meta:    &types.MediaMetadata{MediaID: "../x", Base64Hash: hash, Encryption: &types.EncryptionInfo{Version: 1}},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			key, err := ContentObjectKey(tc.meta)
			if tc.wantErr {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)
			s.Equal(tc.wantKey, key)
			s.NotContains(key, "/tmp", "keys must not carry a pod-local path")
			s.NotEqual('/', rune(key[0]), "keys must be bucket-relative")

			again, err := ContentObjectKey(tc.meta)
			s.Require().NoError(err)
			s.Equal(key, again, "key derivation must be deterministic")
		})
	}
}

func (s *ObjectKeyTestSuite) TestResolveObjectLocation_PrefersStoredLocationAndFallsBackToLegacy() {
	prov := blobprovider.New("S3", "private-bkt", "public-bkt", nil)
	const legacyBase = config.Path("/tmp/media_store")

	testCases := []struct {
		name       string
		meta       *types.MediaMetadata
		wantBucket string
		wantKey    string
		wantErr    bool
	}{
		{
			name: "stored_location_wins",
			meta: &types.MediaMetadata{
				Base64Hash: "abcdef", IsPublic: false,
				BucketName: "old-private-bkt", ObjectKey: "a/b/cdef/file",
			},
			wantBucket: "old-private-bkt",
			wantKey:    "a/b/cdef/file",
		},
		{
			name:       "legacy_private_row_uses_absolute_hash_path",
			meta:       &types.MediaMetadata{Base64Hash: "abcdef", IsPublic: false},
			wantBucket: "private-bkt",
			wantKey:    "/tmp/media_store/a/b/cdef/file",
		},
		{
			name:       "legacy_public_row_uses_public_bucket",
			meta:       &types.MediaMetadata{Base64Hash: "abcdef", IsPublic: true},
			wantBucket: "public-bkt",
			wantKey:    "/tmp/media_store/a/b/cdef/file",
		},
		{
			name:       "key_without_bucket_uses_visibility_bucket",
			meta:       &types.MediaMetadata{Base64Hash: "abcdef", IsPublic: true, ObjectKey: "a/b/cdef/file"},
			wantBucket: "public-bkt",
			wantKey:    "a/b/cdef/file",
		},
		{
			name:    "legacy_row_with_invalid_hash",
			meta:    &types.MediaMetadata{Base64Hash: "ab"},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			bucket, key, err := ResolveObjectLocation(prov, tc.meta, legacyBase)
			if tc.wantErr {
				require.Error(s.T(), err)
				return
			}
			s.Require().NoError(err)
			s.Equal(tc.wantBucket, bucket)
			s.Equal(tc.wantKey, key)
		})
	}
}
