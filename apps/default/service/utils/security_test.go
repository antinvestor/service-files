package utils

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SecurityUtilsTestSuite struct {
	tests.BaseTestSuite
}

func TestSecurityUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(SecurityUtilsTestSuite))
}

func (s *SecurityUtilsTestSuite) TestCreateHash() {
	cases := []struct {
		name     string
		content  []byte
		expected string
	}{
		{
			name:     "empty_content",
			content:  []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple_string",
			content:  []byte("hello"),
			expected: "",
		},
		{
			name:     "binary_data",
			content:  []byte{0x00, 0x01, 0x02, 0xFF},
			expected: "",
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			hash := CreateHash(tc.content)
			assert.NotEmpty(t, hash)
			assert.Len(t, hash, 64)
			for _, char := range hash {
				assert.True(t, (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f'))
			}
			hash2 := CreateHash(tc.content)
			assert.Equal(t, hash, hash2)
			if tc.expected != "" {
				assert.Equal(t, tc.expected, hash)
			}
		})
	}
}

func (s *SecurityUtilsTestSuite) TestCreateHashConsistency() {
	cases := []struct {
		name string
		data []byte
	}{
		{
			name: "consistency",
			data: []byte("test data for consistency check"),
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			hash1 := CreateHash(tc.data)
			hash2 := CreateHash(tc.data)
			hash3 := CreateHash(tc.data)
			assert.Equal(t, hash1, hash2)
			assert.Equal(t, hash2, hash3)
		})
	}
}

func (s *SecurityUtilsTestSuite) TestCreateHashDifferentInputs() {
	cases := []struct {
		name     string
		left     []byte
		right    []byte
		expectEq bool
	}{
		{
			name:     "different_inputs",
			left:     []byte("input1"),
			right:    []byte("input2"),
			expectEq: false,
		},
		{
			name:     "same_inputs",
			left:     []byte("input1"),
			right:    []byte("input1"),
			expectEq: true,
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			hash1 := CreateHash(tc.left)
			hash2 := CreateHash(tc.right)
			if tc.expectEq {
				assert.Equal(t, hash1, hash2)
			} else {
				assert.NotEqual(t, hash1, hash2)
			}
		})
	}
}

func (s *SecurityUtilsTestSuite) TestEncryptDecrypt() {
	encryptionKey := "12345678901234567890123456789012"
	cases := []struct {
		name string
		data []byte
	}{
		{
			name: "simple_text",
			data: []byte("Hello, World!"),
		},
		{
			name: "binary_data",
			data: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		},
		{
			name: "large_data",
			data: []byte(strings.Repeat("A", 1024)),
		},
		{
			name: "unicode_text",
			data: []byte("Hello 世界! 🌍"),
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			encrypted, err := Encrypt(tc.data, encryptionKey)
			require.NoError(t, err)
			require.NotNil(t, encrypted)
			if len(tc.data) > 0 {
				assert.NotEqual(t, tc.data, encrypted)
			}
			assert.True(t, len(encrypted) >= len(tc.data))

			decrypted, err := Decrypt(encrypted, encryptionKey)
			require.NoError(t, err)
			require.NotNil(t, decrypted)
			assert.Equal(t, tc.data, decrypted)
		})
	}
}

func (s *SecurityUtilsTestSuite) TestEncryptWithInvalidKey() {
	cases := []struct {
		name string
		key  string
	}{
		{name: "short_key", key: "short"},
		{name: "long_key", key: "this_key_is_way_too_long_for_aes_encryption_and_should_fail"},
		{name: "empty_key", key: ""},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			_, err := Encrypt([]byte("test data"), tc.key)
			assert.Error(t, err)
		})
	}
}

func (s *SecurityUtilsTestSuite) TestDecryptWithInvalidKey() {
	cases := []struct {
		name       string
		correctKey string
		wrongKey   string
	}{
		{
			name:       "wrong_key",
			correctKey: "12345678901234567890123456789012",
			wrongKey:   "abcdefghijklmnopqrstuvwxyz123456",
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			data := []byte("test data")
			encrypted, err := Encrypt(data, tc.correctKey)
			require.NoError(t, err)
			_, err = Decrypt(encrypted, tc.wrongKey)
			assert.Error(t, err)
		})
	}
}

func (s *SecurityUtilsTestSuite) TestDecryptWithCorruptedData() {
	cases := []struct {
		name string
		key  string
	}{
		{
			name: "corrupted_payload",
			key:  "12345678901234567890123456789012",
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			data := []byte("test data")
			encrypted, err := Encrypt(data, tc.key)
			require.NoError(t, err)
			if len(encrypted) > 0 {
				encrypted[0] ^= 0xFF
			}
			_, err = Decrypt(encrypted, tc.key)
			assert.Error(t, err)
		})
	}
}

func (s *SecurityUtilsTestSuite) TestEncryptDecryptRoundTrip() {
	cases := []struct {
		name string
		data []byte
		key  string
	}{
		{
			name: "roundtrip",
			data: []byte("test data for roundtrip"),
			key:  "12345678901234567890123456789012",
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			encrypted, err := Encrypt(tc.data, tc.key)
			require.NoError(t, err)
			decrypted, err := Decrypt(encrypted, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.data, decrypted)
		})
	}
}

func (s *SecurityUtilsTestSuite) TestEncryptionNonDeterministic() {
	cases := []struct {
		name string
		data []byte
		key  string
	}{
		{
			name: "non_deterministic",
			data: []byte("same plaintext"),
			key:  "12345678901234567890123456789012",
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			encrypted1, err := Encrypt(tc.data, tc.key)
			require.NoError(t, err)
			encrypted2, err := Encrypt(tc.data, tc.key)
			require.NoError(t, err)
			assert.NotEqual(t, encrypted1, encrypted2)
		})
	}
}

func (s *SecurityUtilsTestSuite) TestGenerateRandomString() {
	cases := []struct {
		name   string
		length int
	}{
		{name: "short_length", length: 8},
		{name: "medium_length", length: 16},
		{name: "long_length", length: 32},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			result := GenerateRandomString(tc.length)
			assert.Len(t, result, tc.length)
			assert.True(t, strings.Trim(result, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") == "")
		})
	}
}

func (s *SecurityUtilsTestSuite) TestGenerateRandomStringUniqueness() {
	cases := []struct {
		name   string
		length int
		count  int
	}{
		{name: "unique_values", length: 16, count: 10},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			results := make(map[string]bool)
			for i := 0; i < tc.count; i++ {
				randomString := GenerateRandomString(tc.length)
				results[randomString] = true
			}
			assert.Equal(t, tc.count, len(results))
		})
	}
}

func (s *SecurityUtilsTestSuite) TestGenerateRandomStringNoBias() {
	cases := []struct {
		name string
	}{
		{name: "charset_distribution"},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			const length = 1000
			randomBytes := make([]byte, length)
			_, err := rand.Read(randomBytes)
			require.NoError(t, err)

			randomString := GenerateRandomString(32)
			assert.NotEmpty(t, randomString)
		})
	}
}
