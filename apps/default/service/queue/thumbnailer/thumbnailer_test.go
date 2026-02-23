package thumbnailer

import (
	"image"
	color "image/color" //nolint:misspell
	"image/jpeg"
	"os"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ThumbnailerTestSuite struct {
	tests.BaseTestSuite
}

func TestThumbnailerTestSuite(t *testing.T) {
	suite.Run(t, new(ThumbnailerTestSuite))
}

func (suite *ThumbnailerTestSuite) TestSelectThumbnail() {
	testCases := []struct {
		name            string
		desired         types.ThumbnailSize
		thumbnails      []*types.ThumbnailMetadata
		configuredSizes []types.ThumbnailSize
		wantThumb       bool
		wantSize        bool
	}{
		{
			name: "returns_existing_thumbnail",
			desired: types.ThumbnailSize{
				Width: 32, Height: 32, ResizeMethod: types.Crop,
			},
			thumbnails: []*types.ThumbnailMetadata{
				{MediaMetadata: &types.MediaMetadata{
					FileSizeBytes: 20,
					ThumbnailSize: &types.ThumbnailSize{Width: 64, Height: 64, ResizeMethod: types.Crop},
				}},
			},
			wantThumb: true,
			wantSize:  false,
		},
		{
			name: "returns_configured_size_when_missing",
			desired: types.ThumbnailSize{
				Width: 64, Height: 64, ResizeMethod: types.Scale,
			},
			configuredSizes: []types.ThumbnailSize{
				{Width: 96, Height: 96, ResizeMethod: types.Scale},
			},
			wantThumb: false,
			wantSize:  true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			libCfg := make([]config.ThumbnailSize, 0, len(tc.configuredSizes))
			for _, item := range tc.configuredSizes {
				libCfg = append(libCfg, config.ThumbnailSize(item))
			}

			gotThumb, gotSize := SelectThumbnail(tc.desired, tc.thumbnails, libCfg)
			assert.Equal(t, tc.wantThumb, gotThumb != nil)
			assert.Equal(t, tc.wantSize, gotSize != nil)
		})
	}
}

func (suite *ThumbnailerTestSuite) TestGenerateThumbnail() {
	testCases := []struct {
		name         string
		resizeMethod string
	}{
		{
			name:         "generates_scaled_thumbnail",
			resizeMethod: types.Scale,
		},
		{
			name:         "generates_cropped_thumbnail",
			resizeMethod: types.Crop,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, svc, res := suite.CreateService(t, dep)
				cfg := svc.Config().(*config.FilesConfig)
				log := util.Log(ctx)

				mediaDB, err := connection.NewMediaDatabase(
					svc.WorkManager(),
					res.MediaRepository,
					res.MultipartUploadRepo,
					res.MultipartUploadPartRepo,
					res.FileVersionRepo,
					res.RetentionPolicyRepo,
					res.FileRetentionRepo,
					res.StorageStatsRepo,
				)
				require.NoError(t, err)

				storageProvider, err := provider.GetStorageProvider(ctx, cfg)
				require.NoError(t, err)

				srcFile := createTestImageFile(t, 200, 120)
				hash, size, err := utils.ComputeHashAndSize(types.Path(srcFile))
				require.NoError(t, err)

				mediaMeta := &types.MediaMetadata{
					MediaID:       types.MediaID("media-thumb-" + tc.resizeMethod),
					Base64Hash:    hash,
					UploadName:    "origin.jpg",
					ContentType:   "image/jpeg",
					FileSizeBytes: size,
					OwnerID:       "owner-1",
					ServerName:    cfg.ServerName,
				}

				finalPath, err := utils.GetPathFromBase64Hash(hash, cfg.AbsBasePath)
				require.NoError(t, err)

				_, err = storageProvider.UploadFile(ctx, storageProvider.GetBucket(false), types.Path(srcFile), types.Path(finalPath))
				require.NoError(t, err)

				err = mediaDB.StoreMediaMetadata(ctx, mediaMeta)
				require.NoError(t, err)

				err = GenerateThumbnail(
					ctx,
					types.ThumbnailSize{Width: 64, Height: 64, ResizeMethod: tc.resizeMethod},
					mediaMeta,
					cfg.AbsBasePath,
					mediaDB,
					storageProvider,
					log,
					cfg.EnvStorageEncryptionPhrase,
				)
				require.NoError(t, err)

				thumb, err := mediaDB.GetThumbnail(ctx, mediaMeta.MediaID, 64, 64, tc.resizeMethod)
				require.NoError(t, err)
				require.NotNil(t, thumb)
				assert.Equal(t, mediaMeta.MediaID, thumb.ParentID)
			})
		}
	})
}

func (suite *ThumbnailerTestSuite) TestGenerateThumbnails() {
	testCases := []struct {
		name          string
		sourceWidth   int
		sourceHeight  int
		thumbnailList []config.ThumbnailSize
		expectErr     bool
	}{
		{
			name:         "generates_multiple_sizes",
			sourceWidth:  256,
			sourceHeight: 256,
			thumbnailList: []config.ThumbnailSize{
				{Width: 64, Height: 64, ResizeMethod: types.Scale},
				{Width: 96, Height: 96, ResizeMethod: types.Crop},
			},
			expectErr: false,
		},
		{
			name:         "invalid_source_hash_fails_read",
			sourceWidth:  64,
			sourceHeight: 64,
			thumbnailList: []config.ThumbnailSize{
				{Width: 32, Height: 32, ResizeMethod: types.Scale},
			},
			expectErr: true,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, svc, res := suite.CreateService(t, dep)
				cfg := svc.Config().(*config.FilesConfig)
				log := util.Log(ctx)

				mediaDB, err := connection.NewMediaDatabase(
					svc.WorkManager(),
					res.MediaRepository,
					res.MultipartUploadRepo,
					res.MultipartUploadPartRepo,
					res.FileVersionRepo,
					res.RetentionPolicyRepo,
					res.FileRetentionRepo,
					res.StorageStatsRepo,
				)
				require.NoError(t, err)
				storageProvider, err := provider.GetStorageProvider(ctx, cfg)
				require.NoError(t, err)

				srcFile := createTestImageFile(t, tc.sourceWidth, tc.sourceHeight)
				hash, size, err := utils.ComputeHashAndSize(types.Path(srcFile))
				require.NoError(t, err)
				if tc.expectErr {
					hash = "ab"
				}

				mediaMeta := &types.MediaMetadata{
					MediaID:       types.MediaID("media-thumbs-" + tc.name),
					Base64Hash:    hash,
					UploadName:    "origin.jpg",
					ContentType:   "image/jpeg",
					FileSizeBytes: size,
					OwnerID:       "owner-1",
					ServerName:    cfg.ServerName,
				}

				if !tc.expectErr {
					finalPath, pathErr := utils.GetPathFromBase64Hash(hash, cfg.AbsBasePath)
					require.NoError(t, pathErr)
					_, pathErr = storageProvider.UploadFile(ctx, storageProvider.GetBucket(false), types.Path(srcFile), types.Path(finalPath))
					require.NoError(t, pathErr)
				}

				err = GenerateThumbnails(
					ctx,
					tc.thumbnailList,
					mediaMeta,
					cfg.AbsBasePath,
					mediaDB,
					storageProvider,
					log,
					cfg.EnvStorageEncryptionPhrase,
				)
				if tc.expectErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
			})
		}
	})
}

func (suite *ThumbnailerTestSuite) TestThumbnailFitnessHelpers() {
	testCases := []struct {
		name        string
		a           thumbnailFitness
		b           thumbnailFitness
		desiredCrop bool
		wantBetter  bool
	}{
		{
			name:       "prefers_not_smaller",
			a:          thumbnailFitness{isSmaller: 0},
			b:          thumbnailFitness{isSmaller: 1},
			wantBetter: true,
		},
		{
			name:        "crop_prefers_aspect",
			a:           thumbnailFitness{isSmaller: 0, aspect: 0.1},
			b:           thumbnailFitness{isSmaller: 0, aspect: 1.5},
			desiredCrop: true,
			wantBetter:  true,
		},
		{
			name:        "prefers_size_when_aspect_same",
			a:           thumbnailFitness{isSmaller: 0, aspect: 1, size: 5},
			b:           thumbnailFitness{isSmaller: 0, aspect: 1, size: 20},
			desiredCrop: true,
			wantBetter:  true,
		},
		{
			name:       "prefers_method_match",
			a:          thumbnailFitness{isSmaller: 0, size: 1, methodMismatch: 0},
			b:          thumbnailFitness{isSmaller: 0, size: 1, methodMismatch: 1},
			wantBetter: true,
		},
		{
			name:       "prefers_smaller_file",
			a:          thumbnailFitness{isSmaller: 0, size: 1, methodMismatch: 0, fileSize: 10},
			b:          thumbnailFitness{isSmaller: 0, size: 1, methodMismatch: 0, fileSize: 20},
			wantBetter: true,
		},
		{
			name:       "equal_fitness_not_better",
			a:          thumbnailFitness{},
			b:          thumbnailFitness{},
			wantBetter: false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantBetter, tc.a.betterThan(tc.b, tc.desiredCrop))
		})
	}

	boolCases := []struct {
		name     string
		input    bool
		expected int
	}{
		{name: "true_maps_to_one", input: true, expected: 1},
		{name: "false_maps_to_zero", input: false, expected: 0},
	}
	for _, tc := range boolCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, boolToInt(tc.input))
		})
	}
}

func createTestImageFile(t *testing.T, width, height int) string {
	t.Helper()

	file, err := os.CreateTemp("", "thumb-src-*.jpg")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(file.Name()) })

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 200, A: 255}) //nolint:misspell
		}
	}

	require.NoError(t, jpeg.Encode(file, img, &jpeg.Options{Quality: 90}))
	require.NoError(t, file.Close())
	return file.Name()
}
