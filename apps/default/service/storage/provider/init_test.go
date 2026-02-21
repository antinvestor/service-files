package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	aconfig "github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/local"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProviderTestSuite struct {
	tests.BaseTestSuite
}

func TestProviderTestSuite(t *testing.T) {
	suite.Run(t, new(ProviderTestSuite))
}

func (suite *ProviderTestSuite) TestGetStorageProvider() {
	testCases := []struct {
		name          string
		expectedType  string
		shouldSucceed bool
	}{
		{
			name:          "should return local provider",
			expectedType:  "*local.ProviderLocal",
			shouldSucceed: true,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()

				cfg, err := config.FromEnv[aconfig.FilesConfig]()
				if err != nil {
					t.Errorf("Could not get file config : %v", err)
				}

				storageProvider, err := GetStorageProvider(ctx, &cfg)
				if !tc.shouldSucceed {
					if err == nil {
						t.Errorf("Expected error but got none")
					}
					return
				}

				if err != nil {
					t.Errorf("A file storageProvider should not have issues : %v", err)
				}

				_, ok := storageProvider.(*local.ProviderLocal)
				if !ok {
					t.Errorf("The storageProvider is supposed to be a local instance only")
				}
			})
		}
	})
}

func (suite *ProviderTestSuite) TestUploadFileWithHashCheck() {
	testCases := []struct {
		name        string
		hash        types.Base64Hash
		isPublic    bool
		runTwice    bool
		missingFile bool
		expectDup   bool
		expectError bool
	}{
		{name: "uploads_new_file", hash: "abcde12345", isPublic: false, expectDup: false, expectError: false},
		{name: "detects_duplicate_file", hash: "abcde12345", isPublic: true, runTwice: true, expectDup: true, expectError: false},
		{name: "invalid_hash", hash: "ab", isPublic: false, expectDup: false, expectError: true},
		{name: "missing_source_file", hash: "abcde12345", isPublic: false, missingFile: true, expectDup: false, expectError: true},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			baseDir := t.TempDir()
			privateBucket := filepath.Join(baseDir, "private")
			publicBucket := filepath.Join(baseDir, "public")
			prov := local.NewProvider("local", privateBucket, publicBucket)
			require.NoError(t, prov.Setup(ctx))

			sourceFile := filepath.Join(baseDir, "source.bin")
			if !tc.missingFile {
				require.NoError(t, os.WriteFile(sourceFile, []byte("test-data"), 0o644))
			}

			meta := &types.MediaMetadata{
				Base64Hash:    tc.hash,
				IsPublic:      tc.isPublic,
				FileSizeBytes: types.FileSizeBytes(len("test-data")),
			}

			finalPath, duplicate, err := storage.UploadFileWithHashCheck(
				ctx,
				prov,
				types.Path(sourceFile),
				meta,
				aconfig.Path(baseDir),
				util.Log(ctx),
			)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tc.runTwice {
				finalPath, duplicate, err = storage.UploadFileWithHashCheck(
					ctx,
					prov,
					types.Path(sourceFile),
					meta,
					aconfig.Path(baseDir),
					util.Log(ctx),
				)
				require.NoError(t, err)
			}
			suite.Equal(tc.expectDup, duplicate)
			suite.NotEmpty(finalPath)
		})
	}
}
