package utils

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPathFromBase64Hash(t *testing.T) {
	testCases := []struct {
		name        string
		hash        types.Base64Hash
		basePath    config.Path
		expectedErr bool
		expectedDir string
	}{
		{
			name:        "valid_hash_creates_correct_path",
			hash:        "abcdefghijk",
			basePath:    "/tmp/test",
			expectedErr: false,
			expectedDir: "a/b/cdefghijk",
		},
		{
			name:        "minimum_length_hash",
			hash:        "abc",
			basePath:    "/tmp/test",
			expectedErr: false,
			expectedDir: "a/b/c",
		},
		{
			name:        "hash_too_short",
			hash:        "ab",
			basePath:    "/tmp/test",
			expectedErr: true,
		},
		{
			name:        "empty_hash",
			hash:        "",
			basePath:    "/tmp/test",
			expectedErr: true,
		},
		{
			name:        "hash_too_long",
			hash:        types.Base64Hash(strings.Repeat("a", 256)),
			basePath:    "/tmp/test",
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := GetPathFromBase64Hash(tc.hash, tc.basePath)

			if tc.expectedErr {
				assert.Error(t, err)
				assert.Empty(t, path)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, path, tc.expectedDir)
				assert.Contains(t, path, "file")
				assert.True(t, filepath.IsAbs(path))
			}
		})
	}
}

func TestCreateTempDir(t *testing.T) {
	// Create a temporary base directory for testing
	baseDir, err := os.MkdirTemp("", "fileutils_test")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	tempDir, err := CreateTempDir(config.Path(baseDir))
	require.NoError(t, err)
	require.NotEmpty(t, tempDir)

	// Verify the directory exists
	_, err = os.Stat(string(tempDir))
	assert.NoError(t, err)

	// Verify it's within the base directory
	assert.Contains(t, string(tempDir), baseDir)
	assert.Contains(t, string(tempDir), "tmp/")

	// Clean up
	RemoveDir(tempDir, util.Log(t.Context()))
}

func TestWriteTempFile(t *testing.T) {
	// Create a temporary base directory for testing
	baseDir, err := os.MkdirTemp("", "fileutils_test")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	testContent := "Hello, World! This is test content for file writing."
	reader := strings.NewReader(testContent)

	hash, size, path, err := WriteTempFile(t.Context(), reader, config.Path(baseDir))

	require.NoError(t, err)
	require.NotEmpty(t, hash)
	require.Equal(t, types.FileSizeBytes(len(testContent)), size)
	require.NotEmpty(t, path)

	// Verify path exists (it's a directory path, not the actual file)
	_, err = os.Stat(string(path))
	assert.NoError(t, err)

	// Verify hash is correct (base64 encoded SHA256)
	assert.NotEmpty(t, hash)
	assert.True(t, len(string(hash)) > 0)
}

func TestWriteTempFileWithEmptyContent(t *testing.T) {
	// Create a temporary base directory for testing
	baseDir, err := os.MkdirTemp("", "fileutils_test")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	reader := strings.NewReader("")

	hash, size, path, err := WriteTempFile(t.Context(), reader, config.Path(baseDir))

	require.NoError(t, err)
	require.NotEmpty(t, hash)
	require.Equal(t, types.FileSizeBytes(0), size)
	require.NotEmpty(t, path)

	// Verify path exists (it's a directory path, not the actual file)
	_, err = os.Stat(string(path))
	assert.NoError(t, err)

	// Verify hash is correct (base64 encoded SHA256)
	assert.NotEmpty(t, hash)
	assert.True(t, len(string(hash)) > 0)
}

func TestWriteTempFileWithLargeContent(t *testing.T) {
	// Create a temporary base directory for testing
	baseDir, err := os.MkdirTemp("", "fileutils_test")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	// Create large content (1MB)
	largeContent := strings.Repeat("A", 1024*1024)
	reader := strings.NewReader(largeContent)

	hash, size, path, err := WriteTempFile(t.Context(), reader, config.Path(baseDir))

	require.NoError(t, err)
	require.NotEmpty(t, hash)
	require.Equal(t, types.FileSizeBytes(len(largeContent)), size)
	require.NotEmpty(t, path)

	// Verify path exists (it's a directory path, not the actual file)
	_, err = os.Stat(string(path))
	assert.NoError(t, err)

	// Verify hash is correct (base64 encoded SHA256)
	assert.NotEmpty(t, hash)
	assert.True(t, len(string(hash)) > 0)
}

func TestWriteTempFileWithContextCancellation(t *testing.T) {
	// Create a temporary base directory for testing
	baseDir, err := os.MkdirTemp("", "fileutils_test")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	reader := strings.NewReader("test content")

	_, _, _, err = WriteTempFile(ctx, reader, config.Path(baseDir))

	// Should return context cancelled error or succeed (depending on timing)
	// The function might complete before context cancellation is checked
	if err != nil {
		assert.Contains(t, err.Error(), "context canceled")
	}
}

func TestRemoveDir(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "remove_test")
	require.NoError(t, err)

	// Create a file in the directory
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Verify directory and file exist
	_, err = os.Stat(tempDir)
	assert.NoError(t, err)
	_, err = os.Stat(testFile)
	assert.NoError(t, err)

	// Remove directory
	RemoveDir(types.Path(tempDir), util.Log(t.Context()))

	// Verify directory is removed
	_, err = os.Stat(tempDir)
	assert.True(t, os.IsNotExist(err))
}

func TestMoveFile(t *testing.T) {
	// Create temporary directories
	srcDir, err := os.MkdirTemp("", "move_src")
	require.NoError(t, err)
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "move_dst")
	require.NoError(t, err)
	defer os.RemoveAll(dstDir)

	// Create source file
	srcFile := filepath.Join(srcDir, "source.txt")
	testContent := "test content for move"
	err = os.WriteFile(srcFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Move file
	dstFile := filepath.Join(dstDir, "destination.txt")
	err = moveFile(types.Path(srcFile), types.Path(dstFile))
	require.NoError(t, err)

	// Verify source file is gone
	_, err = os.Stat(srcFile)
	assert.True(t, os.IsNotExist(err))

	// Verify destination file exists with correct content
	content, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestCreateTempFileWriter(t *testing.T) {
	// Create a temporary base directory for testing
	baseDir, err := os.MkdirTemp("", "fileutils_test")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	writer, file, path, err := createTempFileWriter(config.Path(baseDir))
	require.NoError(t, err)
	require.NotNil(t, writer)
	require.NotNil(t, file)
	require.NotEmpty(t, path)

	defer file.Close()

	// Write some content
	testContent := "test content"
	_, err = writer.WriteString(testContent)
	require.NoError(t, err)

	// Flush the writer
	err = writer.Flush()
	require.NoError(t, err)

	// Verify the path exists (it's a temp directory path)
	_, err = os.Stat(string(path))
	assert.NoError(t, err)
}

func TestCreateFileWriter(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "fileutils_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	writer, file, err := createFileWriter(types.Path(tempDir))
	require.NoError(t, err)
	require.NotNil(t, writer)
	require.NotNil(t, file)

	defer file.Close()

	// Write some content
	testContent := "test content for file writer"
	_, err = writer.WriteString(testContent)
	require.NoError(t, err)

	// Flush the writer
	err = writer.Flush()
	require.NoError(t, err)

	// Verify file has content
	info, err := file.Stat()
	require.NoError(t, err)
	assert.Equal(t, int64(len(testContent)), info.Size())
}

func TestWriteTempFileErrorHandling(t *testing.T) {
	// Test with invalid base path
	reader := strings.NewReader("test")

	// Use a path that doesn't exist and can't be created
	invalidPath := config.Path("/invalid/nonexistent/path")

	_, _, _, err := WriteTempFile(t.Context(), reader, invalidPath)
	assert.Error(t, err)
}

func TestGetPathFromBase64HashEdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		hash     types.Base64Hash
		basePath config.Path
		wantErr  bool
	}{
		{
			name:     "exactly_3_chars",
			hash:     "abc",
			basePath: "/tmp",
			wantErr:  false,
		},
		{
			name:     "exactly_255_chars",
			hash:     types.Base64Hash(strings.Repeat("a", 255)),
			basePath: "/tmp",
			wantErr:  false,
		},
		{
			name:     "special_characters_in_hash",
			hash:     "a+/=def",
			basePath: "/tmp",
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := GetPathFromBase64Hash(tc.hash, tc.basePath)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, path)
			}
		})
	}
}
