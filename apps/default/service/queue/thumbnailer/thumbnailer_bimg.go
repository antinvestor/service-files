// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build bimg

package thumbnailer

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/pitabwire/util"
	"gopkg.in/h2non/bimg.v1"
)

// GenerateThumbnails generates the configured thumbnail sizes for the source file.
func GenerateThumbnails(
	ctx context.Context,
	configs []config.ThumbnailSize,
	mediaMetadata *types.MediaMetadata,
	absBasePath config.Path,
	db storage.Database,
	provider storage.Provider,
	logger *util.LogEntry,
	encryptionKey string,
) error {
	img, err := readFile(ctx, provider, absBasePath, mediaMetadata)
	if err != nil {
		return err
	}

	tempDir, err := utils.CreateTempDir(absBasePath)
	if err != nil {
		return err
	}
	defer utils.RemoveDir(tempDir, logger)

	for _, singleConfig := range configs {
		if err := createThumbnail(ctx, absBasePath, tempDir, img, types.ThumbnailSize(singleConfig), mediaMetadata, db, provider, logger, encryptionKey); err != nil {
			return err
		}
	}
	return nil
}

// GenerateThumbnail generates a specific thumbnail size for the source file.
func GenerateThumbnail(
	ctx context.Context,
	config types.ThumbnailSize,
	mediaMetadata *types.MediaMetadata,
	absBasePath config.Path,
	db storage.Database,
	provider storage.Provider,
	logger *util.LogEntry,
	encryptionKey string,
) error {
	img, err := readFile(ctx, provider, absBasePath, mediaMetadata)
	if err != nil {
		return err
	}

	tempDir, err := utils.CreateTempDir(absBasePath)
	if err != nil {
		return err
	}
	defer utils.RemoveDir(tempDir, logger)

	return createThumbnail(ctx, absBasePath, tempDir, img, config, mediaMetadata, db, provider, logger, encryptionKey)
}

func readFile(ctx context.Context, provider storage.Provider, absBasePath config.Path, mediaMetadata *types.MediaMetadata) (*bimg.Image, error) {
	finalPath, err := utils.GetPathFromBase64Hash(mediaMetadata.Base64Hash, absBasePath)
	if err != nil {
		return nil, err
	}

	downloadBucket := provider.GetBucket(mediaMetadata.IsPublic)
	reader, cleanup, err := provider.DownloadFile(ctx, downloadBucket, types.Path(finalPath))
	if err != nil {
		return nil, err
	}
	defer cleanup()

	buffer, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return bimg.NewImage(buffer), nil
}

// createThumbnail checks if the thumbnail exists, and if not, generates it.
func createThumbnail(
	ctx context.Context,
	absBasePath config.Path,
	temporaryPath types.Path,
	img *bimg.Image,
	config types.ThumbnailSize,
	mediaMetadata *types.MediaMetadata,
	db storage.Database,
	provider storage.Provider,
	logger *util.LogEntry,
	encryptionKey string,
) error {
	logger = logger.With(
		"Width", config.Width,
		"Height", config.Height,
		"ResizeMethod", config.ResizeMethod,
	)

	if isLargerThanOriginal(config, img) {
		return nil
	}

	tempThumbnailPath, err := GetTempThumbnailPath(temporaryPath, config)
	if err != nil {
		return err
	}

	exists, err := isThumbnailExists(ctx, config, mediaMetadata, db, logger)
	if err != nil || exists {
		return err
	}

	start := time.Now()
	width, height, err := resize(tempThumbnailPath, img, config.Width, config.Height, config.ResizeMethod == "crop", logger)
	if err != nil {
		return err
	}
	logger.With(
		"ActualWidth", width,
		"ActualHeight", height,
		"processTime", time.Since(start),
	).Info("Generated thumbnail")

	hash, size, err := utils.ComputeHashAndSize(tempThumbnailPath)
	if err != nil {
		return err
	}

	thumbnailMetadata := &types.ThumbnailMetadata{
		MediaMetadata: &types.MediaMetadata{
			MediaID:           types.MediaID(utils.GenerateRandomString(32)),
			ParentID:          mediaMetadata.MediaID,
			ContentType:       types.ContentType("image/jpeg"),
			FileSizeBytes:     size,
			Base64Hash:        hash,
			OwnerID:           mediaMetadata.OwnerID,
			ServerName:        mediaMetadata.ServerName,
			IsPublic:          mediaMetadata.IsPublic,
			CreationTimestamp: uint64(time.Now().UnixMilli()),
			ThumbnailSize: &types.ThumbnailSize{
				Width:        config.Width,
				Height:       config.Height,
				ResizeMethod: config.ResizeMethod,
			},
		},
	}

	sourcePath := tempThumbnailPath
	if !mediaMetadata.IsPublic {
		if len(encryptionKey) != 32 {
			return fmt.Errorf("invalid encryption key length")
		}
		encryptedPath := types.Path(string(tempThumbnailPath) + ".encrypted")
		srcFile, err := os.Open(string(tempThumbnailPath))
		if err != nil {
			return err
		}
		defer util.CloseAndLogOnError(ctx, srcFile)

		dstFile, err := os.Create(string(encryptedPath))
		if err != nil {
			return err
		}
		defer util.CloseAndLogOnError(ctx, dstFile)

		info, err := storage.EncryptStream(ctx, srcFile, dstFile, []byte(encryptionKey))
		if err != nil {
			return err
		}
		thumbnailMetadata.MediaMetadata.Encryption = info
		sourcePath = encryptedPath
	}

	finalPath, duplicate, err := storage.UploadFileWithHashCheck(ctx, provider, sourcePath, thumbnailMetadata.MediaMetadata, absBasePath, logger)
	if err != nil {
		return err
	}
	if duplicate {
		logger.WithField("dst", finalPath).Info("File was stored previously - discarding duplicate")
	}

	if err = db.StoreThumbnail(ctx, thumbnailMetadata); err != nil {
		logger.WithError(err).With(
			"ActualWidth", width,
			"ActualHeight", height,
		).Error("Failed to store thumbnail metadata in database.")
		return err
	}

	return nil
}

func isLargerThanOriginal(config types.ThumbnailSize, img *bimg.Image) bool {
	imgSize, err := img.Size()
	if err == nil && config.Width >= imgSize.Width && config.Height >= imgSize.Height {
		return true
	}
	return false
}

// resize scales an image to fit within the provided width and height.
func resize(dst types.Path, inImage *bimg.Image, w, h int, crop bool, logger *util.LogEntry) (int, int, error) {
	inSize, err := inImage.Size()
	if err != nil {
		return -1, -1, err
	}

	options := bimg.Options{
		Type:    bimg.JPEG,
		Quality: 85,
	}
	if crop {
		options.Width = w
		options.Height = h
		options.Crop = true
	} else {
		inAR := float64(inSize.Width) / float64(inSize.Height)
		outAR := float64(w) / float64(h)

		if inAR > outAR {
			options.Width = w
			options.Height = int(float64(w) / inAR)
		} else {
			options.Width = int(float64(h) * inAR)
			options.Height = h
		}
	}

	newImage, err := inImage.Process(options)
	if err != nil {
		return -1, -1, err
	}

	if err = bimg.Write(string(dst), newImage); err != nil {
		logger.WithError(err).Error("Failed to resize image")
		return -1, -1, err
	}

	return options.Width, options.Height, nil
}
