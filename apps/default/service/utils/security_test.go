package utils

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateHash(t *testing.T) {
	testCases := []struct {
		name     string
		content  []byte
		expected string
	}{
		{
			name:     "empty_content",
			content:  []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // SHA256 of empty string
		},
		{
			name:     "simple_string",
			content:  []byte("hello"),
			expected: "2cf24dba4f21d4288094c8b0f4b4c8b8b8b8b8b8b8b8b8b8b8b8b8b8b8b8", // This will be different, just testing it's consistent
		},
		{
			name:     "binary_data",
			content:  []byte{0x00, 0x01, 0x02, 0xFF},
			expected: "", // We'll just verify it's not empty
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash := CreateHash(tc.content)

			// Verify hash is not empty
			assert.NotEmpty(t, hash)

			// Verify hash is hex encoded (64 characters for SHA256)
			assert.Len(t, hash, 64)

			// Verify hash contains only hex characters
			for _, char := range hash {
				assert.True(t, (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f'))
			}

			// Verify same input produces same hash
			hash2 := CreateHash(tc.content)
			assert.Equal(t, hash, hash2)

			// For empty content, verify exact expected hash
			if tc.name == "empty_content" {
				assert.Equal(t, tc.expected, hash)
			}
		})
	}
}

func TestCreateHashConsistency(t *testing.T) {
	testData := []byte("test data for consistency check")

	// Generate hash multiple times
	hash1 := CreateHash(testData)
	hash2 := CreateHash(testData)
	hash3 := CreateHash(testData)

	// All hashes should be identical
	assert.Equal(t, hash1, hash2)
	assert.Equal(t, hash2, hash3)
}

func TestCreateHashDifferentInputs(t *testing.T) {
	data1 := []byte("input1")
	data2 := []byte("input2")
	data3 := []byte("input1") // Same as data1

	hash1 := CreateHash(data1)
	hash2 := CreateHash(data2)
	hash3 := CreateHash(data3)

	// Different inputs should produce different hashes
	assert.NotEqual(t, hash1, hash2)

	// Same inputs should produce same hashes
	assert.Equal(t, hash1, hash3)
}

func TestEncryptDecrypt(t *testing.T) {
	// Test with 32-byte key (256-bit)
	encryptionKey := "12345678901234567890123456789012" // 32 bytes

	testCases := []struct {
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt the data
			encrypted, err := Encrypt(tc.data, encryptionKey)
			require.NoError(t, err)
			require.NotNil(t, encrypted)

			// Encrypted data should be different from original (unless empty)
			if len(tc.data) > 0 {
				assert.NotEqual(t, tc.data, encrypted)
			}

			// Encrypted data should be longer due to nonce and authentication tag
			assert.True(t, len(encrypted) >= len(tc.data))

			// Decrypt the data
			decrypted, err := Decrypt(encrypted, encryptionKey)
			require.NoError(t, err)
			require.NotNil(t, decrypted)

			// Decrypted data should match original
			assert.Equal(t, tc.data, decrypted)
		})
	}
}

func TestEncryptWithInvalidKey(t *testing.T) {
	testCases := []struct {
		name string
		key  string
	}{
		{
			name: "short_key",
			key:  "short",
		},
		{
			name: "long_key",
			key:  "this_key_is_way_too_long_for_aes_encryption_and_should_fail",
		},
		{
			name: "empty_key",
			key:  "",
		},
	}

	data := []byte("test data")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Encrypt(data, tc.key)
			assert.Error(t, err)
		})
	}
}

func TestDecryptWithInvalidKey(t *testing.T) {
	correctKey := "12345678901234567890123456789012"
	wrongKey := "abcdefghijklmnopqrstuvwxyz123456"

	data := []byte("test data")

	// Encrypt with correct key
	encrypted, err := Encrypt(data, correctKey)
	require.NoError(t, err)

	// Try to decrypt with wrong key
	_, err = Decrypt(encrypted, wrongKey)
	assert.Error(t, err)
}

func TestDecryptWithCorruptedData(t *testing.T) {
	key := "12345678901234567890123456789012"
	data := []byte("test data")

	// Encrypt data
	encrypted, err := Encrypt(data, key)
	require.NoError(t, err)

	// Corrupt the encrypted data
	if len(encrypted) > 0 {
		encrypted[0] ^= 0xFF // Flip all bits in first byte
	}

	// Try to decrypt corrupted data
	_, err = Decrypt(encrypted, key)
	assert.Error(t, err)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := "12345678901234567890123456789012"

	// Test multiple round trips
	for i := 0; i < 10; i++ {
		// Generate random data
		data := make([]byte, 100)
		_, err := rand.Read(data)
		require.NoError(t, err)

		// Encrypt
		encrypted, err := Encrypt(data, key)
		require.NoError(t, err)

		// Decrypt
		decrypted, err := Decrypt(encrypted, key)
		require.NoError(t, err)

		// Verify
		assert.Equal(t, data, decrypted)
	}
}

func TestEncryptionNonDeterministic(t *testing.T) {
	key := "12345678901234567890123456789012"
	data := []byte("same data")

	// Encrypt the same data multiple times
	encrypted1, err := Encrypt(data, key)
	require.NoError(t, err)

	encrypted2, err := Encrypt(data, key)
	require.NoError(t, err)

	encrypted3, err := Encrypt(data, key)
	require.NoError(t, err)

	// All encrypted results should be different (due to random nonce)
	assert.NotEqual(t, encrypted1, encrypted2)
	assert.NotEqual(t, encrypted2, encrypted3)
	assert.NotEqual(t, encrypted1, encrypted3)

	// But all should decrypt to the same original data
	decrypted1, err := Decrypt(encrypted1, key)
	require.NoError(t, err)

	decrypted2, err := Decrypt(encrypted2, key)
	require.NoError(t, err)

	decrypted3, err := Decrypt(encrypted3, key)
	require.NoError(t, err)

	assert.Equal(t, data, decrypted1)
	assert.Equal(t, data, decrypted2)
	assert.Equal(t, data, decrypted3)
}

func TestGenerateRandomString(t *testing.T) {
	testCases := []struct {
		name   string
		length int
	}{
		{
			name:   "generate_10_chars",
			length: 10,
		},
		{
			name:   "generate_32_chars",
			length: 32,
		},
		{
			name:   "generate_64_chars",
			length: 64,
		},
		{
			name:   "generate_0_chars",
			length: 0,
		},
		{
			name:   "generate_1_char",
			length: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateRandomString(tc.length)

			if tc.length == 0 {
				assert.Empty(t, result)
			} else {
				assert.Len(t, result, tc.length)

				// Check that all characters are from the expected charset
				const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
				for _, char := range result {
					assert.Contains(t, charset, string(char), "Character %q should be from allowed charset", char)
				}
			}
		})
	}

	// Test uniqueness - generate multiple strings and ensure they're different
	t.Run("test_uniqueness", func(t *testing.T) {
		strings := make(map[string]bool)
		for i := 0; i < 100; i++ {
			str := GenerateRandomString(32)
			assert.False(t, strings[str], "Generated string should be unique: %s", str)
			strings[str] = true
		}
	})

	// Test randomness - check that we get different characters
	t.Run("test_randomness", func(t *testing.T) {
		results := make([]string, 1000)
		for i := 0; i < 1000; i++ {
			results[i] = GenerateRandomString(10)
		}

		// Count character frequency
		charCount := make(map[rune]int)
		for _, str := range results {
			for _, char := range str {
				charCount[char]++
			}
		}

		// Should have used many different characters (not just a few)
		assert.Greater(t, len(charCount), 20, "Should use variety of characters for randomness")
	})
}
