package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/local"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type StorageProviderTestSuite struct {
	tests.BaseTestSuite
}

func TestStorageProviderTestSuite(t *testing.T) {
	suite.Run(t, new(StorageProviderTestSuite))
}

func (suite *StorageProviderTestSuite) TestUploadFileWithHashCheck() {
	testCases := []struct {
		name        string
		hash        types.Base64Hash
		sourceData  string
		isPublic    bool
		missingFile bool
		expectErr   bool
		expectDup   bool
		runTwice    bool
	}{
		{
			name:       "stores_file_and_returns_final_path",
			hash:       "abcde12345",
			sourceData: "payload",
			isPublic:   false,
		},
		{
			name:       "duplicate_is_detected",
			hash:       "abcde12345",
			sourceData: "payload",
			isPublic:   true,
			runTwice:   true,
			expectDup:  true,
		},
		{
			name:       "invalid_hash_fails",
			hash:       "ab",
			sourceData: "payload",
			expectErr:  true,
		},
		{
			name:        "missing_source_file_fails",
			hash:        "abcde12345",
			missingFile: true,
			expectErr:   true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			baseDir := t.TempDir()
			privateBucket := filepath.Join(baseDir, "private")
			publicBucket := filepath.Join(baseDir, "public")
			prov := local.NewProvider("local", privateBucket, publicBucket)
			require.NoError(t, prov.Setup(ctx))

			sourcePath := filepath.Join(baseDir, "source.bin")
			if !tc.missingFile {
				require.NoError(t, os.WriteFile(sourcePath, []byte(tc.sourceData), 0o644))
			}

			metadata := &types.MediaMetadata{
				Base64Hash:    tc.hash,
				FileSizeBytes: types.FileSizeBytes(len(tc.sourceData)),
				IsPublic:      tc.isPublic,
			}

			finalPath, duplicate, err := UploadFileWithHashCheck(
				ctx,
				prov,
				types.Path(sourcePath),
				metadata,
				config.Path(baseDir),
				util.Log(ctx),
			)
			if tc.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tc.runTwice {
				finalPath, duplicate, err = UploadFileWithHashCheck(
					ctx,
					prov,
					types.Path(sourcePath),
					metadata,
					config.Path(baseDir),
					util.Log(ctx),
				)
				require.NoError(t, err)
			}

			assert.NotEmpty(t, finalPath)
			assert.Equal(t, tc.expectDup, duplicate)
		})
	}
}
