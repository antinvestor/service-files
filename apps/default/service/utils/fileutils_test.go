package utils

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type FileUtilsTestSuite struct {
	tests.BaseTestSuite
}

func TestFileUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(FileUtilsTestSuite))
}

func (s *FileUtilsTestSuite) TestGetPathFromBase64Hash() {
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
		{
			name:        "exactly_255_chars",
			hash:        types.Base64Hash(strings.Repeat("a", 255)),
			basePath:    "/tmp/test",
			expectedErr: false,
			expectedDir: "a/a/" + strings.Repeat("a", 253),
		},
		{
			name:        "hash_with_urlsafe_chars",
			hash:        "a-_def",
			basePath:    "/tmp/test",
			expectedErr: false,
			expectedDir: "a/-/_def",
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			path, err := GetPathFromBase64Hash(tc.hash, tc.basePath)

			if tc.expectedErr {
				assert.Error(t, err)
				assert.Empty(t, path)
				return
			}

			assert.NoError(t, err)
			assert.Contains(t, path, tc.expectedDir)
			assert.Contains(t, path, "file")
			assert.True(t, filepath.IsAbs(path))
		})
	}
}

func (s *FileUtilsTestSuite) TestCreateTempDir() {
	testCases := []struct {
		name string
	}{
		{name: "creates_temp_dir"},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			baseDir := t.TempDir()
			tempDir, err := CreateTempDir(config.Path(baseDir))
			require.NoError(t, err)
			require.NotEmpty(t, tempDir)

			_, err = os.Stat(string(tempDir))
			assert.NoError(t, err)
			assert.Contains(t, string(tempDir), baseDir)
			assert.Contains(t, string(tempDir), "tmp"+string(os.PathSeparator))

			RemoveDir(tempDir, util.Log(t.Context()))
		})
	}
}

func (s *FileUtilsTestSuite) TestWriteTempFile() {
	testCases := []struct {
		name    string
		content string
	}{
		{
			name:    "writes_regular_content",
			content: "Hello, World! This is test content for file writing.",
		},
		{
			name:    "writes_empty_content",
			content: "",
		},
		{
			name:    "writes_large_content",
			content: strings.Repeat("A", 1024*1024),
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			baseDir := t.TempDir()
			reader := strings.NewReader(tc.content)

			hash, size, path, err := WriteTempFile(t.Context(), reader, config.Path(baseDir))

			require.NoError(t, err)
			require.NotEmpty(t, hash)
			require.Equal(t, types.FileSizeBytes(len(tc.content)), size)
			require.NotEmpty(t, path)

			_, err = os.Stat(string(path))
			assert.NoError(t, err)
		})
	}
}

func (s *FileUtilsTestSuite) TestWriteTempFileWithContextCancellation() {
	testCases := []struct {
		name string
	}{
		{name: "returns_context_error"},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			baseDir := t.TempDir()

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			reader := strings.NewReader("test content")

			_, _, _, err := WriteTempFile(ctx, reader, config.Path(baseDir))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "context canceled")
		})
	}
}

func (s *FileUtilsTestSuite) TestRemoveDir() {
	testCases := []struct {
		name string
	}{
		{name: "removes_directory"},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			testFile := filepath.Join(tempDir, "test.txt")
			err := os.WriteFile(testFile, []byte("test"), 0644)
			require.NoError(t, err)

			_, err = os.Stat(tempDir)
			assert.NoError(t, err)
			_, err = os.Stat(testFile)
			assert.NoError(t, err)

			RemoveDir(types.Path(tempDir), util.Log(t.Context()))

			_, err = os.Stat(tempDir)
			assert.True(t, os.IsNotExist(err))
		})
	}
}

func (s *FileUtilsTestSuite) TestMoveFile() {
	testCases := []struct {
		name string
	}{
		{name: "moves_file"},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			srcDir := t.TempDir()
			dstDir := t.TempDir()

			srcFile := filepath.Join(srcDir, "source.txt")
			testContent := "test content for move"
			err := os.WriteFile(srcFile, []byte(testContent), 0644)
			require.NoError(t, err)

			dstFile := filepath.Join(dstDir, "destination.txt")
			err = moveFile(types.Path(srcFile), types.Path(dstFile))
			require.NoError(t, err)

			_, err = os.Stat(srcFile)
			assert.True(t, os.IsNotExist(err))

			content, err := os.ReadFile(dstFile)
			require.NoError(t, err)
			assert.Equal(t, testContent, string(content))
		})
	}
}

func (s *FileUtilsTestSuite) TestCreateTempFileWriter() {
	testCases := []struct {
		name string
	}{
		{name: "creates_temp_file_writer"},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			baseDir := t.TempDir()

			writer, file, path, err := createTempFileWriter(config.Path(baseDir))
			require.NoError(t, err)
			require.NotNil(t, writer)
			require.NotNil(t, file)
			require.NotEmpty(t, path)

			defer file.Close()

			testContent := "test content"
			_, err = writer.WriteString(testContent)
			require.NoError(t, err)

			err = writer.Flush()
			require.NoError(t, err)

			_, err = os.Stat(string(path))
			assert.NoError(t, err)
		})
	}
}

func (s *FileUtilsTestSuite) TestCreateFileWriter() {
	testCases := []struct {
		name string
	}{
		{name: "creates_file_writer"},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			writer, file, err := createFileWriter(types.Path(tempDir))
			require.NoError(t, err)
			require.NotNil(t, writer)
			require.NotNil(t, file)

			defer file.Close()

			testContent := "test content for file writer"
			_, err = writer.WriteString(testContent)
			require.NoError(t, err)

			err = writer.Flush()
			require.NoError(t, err)

			info, err := file.Stat()
			require.NoError(t, err)
			assert.Equal(t, int64(len(testContent)), info.Size())
		})
	}
}

func (s *FileUtilsTestSuite) TestWriteTempFileErrorHandling() {
	testCases := []struct {
		name       string
		invalidDir config.Path
	}{
		{
			name:       "invalid_base_path",
			invalidDir: config.Path("/invalid/nonexistent/path"),
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader("test")
			_, _, _, err := WriteTempFile(t.Context(), reader, tc.invalidDir)
			assert.Error(t, err)
		})
	}
}

func (s *FileUtilsTestSuite) TestComputeHashAndSize() {
	testCases := []struct {
		name        string
		content     string
		expectError bool
	}{
		{name: "computes_hash_for_file", content: "hash me", expectError: false},
		{name: "missing_file", expectError: true},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), "input.txt")
			if tc.content != "" {
				require.NoError(t, os.WriteFile(filePath, []byte(tc.content), 0o644))
			}

			hash, size, err := ComputeHashAndSize(types.Path(filePath))
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, hash)
			assert.Equal(t, types.FileSizeBytes(len(tc.content)), size)
		})
	}
}

func (s *FileUtilsTestSuite) TestMoveFileWithHashCheck() {
	testCases := []struct {
		name        string
		seedFinal   bool
		seedContent string
		fileContent string
		expectDup   bool
		expectError bool
	}{
		{name: "moves_new_file", fileContent: "fresh-content", expectDup: false, expectError: false},
		{name: "returns_duplicate_for_same_size", seedFinal: true, seedContent: "same-size-123", fileContent: "same-size-123", expectDup: true, expectError: false},
		{name: "returns_error_for_hash_collision", seedFinal: true, seedContent: "different-size", fileContent: "small", expectDup: true, expectError: true},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			baseDir := t.TempDir()
			tmpDir := filepath.Join(baseDir, "tmp", "case")
			require.NoError(t, os.MkdirAll(tmpDir, 0o755))

			contentPath := filepath.Join(tmpDir, "content")
			require.NoError(t, os.WriteFile(contentPath, []byte(tc.fileContent), 0o644))

			hash, size, err := ComputeHashAndSize(types.Path(contentPath))
			require.NoError(t, err)

			finalPath, err := GetPathFromBase64Hash(hash, config.Path(baseDir))
			require.NoError(t, err)

			if tc.seedFinal {
				require.NoError(t, os.MkdirAll(filepath.Dir(finalPath), 0o755))
				require.NoError(t, os.WriteFile(finalPath, []byte(tc.seedContent), 0o644))
			}

			meta := &types.MediaMetadata{
				Base64Hash:    hash,
				FileSizeBytes: size,
			}

			gotPath, duplicate, moveErr := MoveFileWithHashCheck(types.Path(tmpDir), meta, config.Path(baseDir), util.Log(t.Context()))
			if tc.expectError {
				require.Error(t, moveErr)
				assert.Equal(t, tc.expectDup, duplicate)
				return
			}

			require.NoError(t, moveErr)
			assert.Equal(t, tc.expectDup, duplicate)
			assert.Equal(t, finalPath, string(gotPath))
			_, statErr := os.Stat(finalPath)
			assert.NoError(t, statErr)
		})
	}
}
