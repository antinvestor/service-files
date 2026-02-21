package storage_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EncryptionTestSuite struct {
	tests.BaseTestSuite
}

func TestEncryptionTestSuite(t *testing.T) {
	suite.Run(t, new(EncryptionTestSuite))
}

func (s *EncryptionTestSuite) TestEncryptDecryptStream() {
	cases := []struct {
		name      string
		masterKey []byte
		payload   []byte
	}{
		{
			name:      "round_trip",
			masterKey: []byte("0123456789abcdef0123456789abcdef"),
			payload:   bytes.Repeat([]byte("hello-world-"), 1024),
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			var encrypted bytes.Buffer
			info, err := storage.EncryptStream(t.Context(), bytes.NewReader(tc.payload), &encrypted, tc.masterKey)
			require.NoError(t, err)
			require.NotNil(t, info)
			require.Equal(t, 1, info.Version)
			require.NotEmpty(t, info.WrappedKey)

			reader, err := storage.NewDecryptingReader(bytes.NewReader(encrypted.Bytes()), tc.masterKey, info)
			require.NoError(t, err)

			decrypted, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.Equal(t, tc.payload, decrypted)
		})
	}
}

func (s *EncryptionTestSuite) TestDecryptReader_InvalidKey() {
	cases := []struct {
		name      string
		info      *types.EncryptionInfo
		masterKey []byte
	}{
		{
			name: "invalid_key",
			info: &types.EncryptionInfo{
				Version:         1,
				Algorithm:       "AES-256-GCM-CHUNKED",
				ChunkSizeBytes:  1024,
				WrappedKey:      "invalid",
				WrappedKeyNonce: "invalid",
				NoncePrefix:     "invalid",
			},
			masterKey: []byte("short"),
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			_, err := storage.NewDecryptingReader(bytes.NewReader(nil), tc.masterKey, tc.info)
			require.Error(t, err)
		})
	}
}

func (s *EncryptionTestSuite) TestDecryptReaderAndHelpersEdgeCases() {
	testCases := []struct {
		name      string
		run       func(t *testing.T) error
		expectErr bool
	}{
		{
			name: "nil_info_returns_original_reader",
			run: func(t *testing.T) error {
				src := bytes.NewReader([]byte("plain"))
				reader, err := storage.NewDecryptingReader(src, []byte("0123456789abcdef0123456789abcdef"), nil)
				require.NoError(t, err)
				data, err := io.ReadAll(reader)
				require.NoError(t, err)
				require.Equal(t, "plain", string(data))
				return nil
			},
			expectErr: false,
		},
		{
			name: "invalid_nonce_prefix_length",
			run: func(t *testing.T) error {
				_, err := storage.NewDecryptingReader(bytes.NewReader(nil),
					[]byte("0123456789abcdef0123456789abcdef"),
					&types.EncryptionInfo{
						Version:         1,
						Algorithm:       "AES-256-GCM-CHUNKED",
						ChunkSizeBytes:  1024,
						WrappedKey:      "AA",
						WrappedKeyNonce: "AA",
						NoncePrefix:     "AA",
					},
				)
				return err
			},
			expectErr: true,
		},
		{
			name: "encrypt_stream_rejects_bad_key_length",
			run: func(t *testing.T) error {
				_, err := storage.EncryptStream(t.Context(), bytes.NewReader([]byte("abc")), &bytes.Buffer{}, []byte("short"))
				return err
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			err := tc.run(t)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
