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

//go:build !bimg
// +build !bimg

package thumbnailer

import (
	"context"
	"fmt"
	"image"
	"image/draw"

	// Imported for gif codec
	_ "image/gif"
	"image/jpeg"

	// Imported for png codec
	_ "image/png"
	// Imported for webp codec
	"os"
	"time"

	"github.com/antinvestor/service-files/apps/default/config"
	storage2 "github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/nfnt/resize"
	"github.com/pitabwire/util"
	_ "golang.org/x/image/webp"
)

// GenerateThumbnails generates the configured thumbnail sizes for the source file
func GenerateThumbnails(
	ctx context.Context,
	configs []config.ThumbnailSize,
	mediaMetadata *types.MediaMetadata,
	absBasePath config.Path,
	db storage2.Database,
	provider storage2.Provider,
	logger *util.LogEntry,
) (errorReturn error) {

	img, err := readFile(ctx, provider, absBasePath, mediaMetadata)
	if err != nil {
		return err
	}

	tempDir, err := utils.CreateTempDir(absBasePath)
	if err != nil {
		return err
	}

	for _, singleConfig := range configs {
		// Note: createThumbnail does locking based on activeThumbnailGeneration
		err = createThumbnail(
			ctx, absBasePath, tempDir, img, types.ThumbnailSize(singleConfig), mediaMetadata, db, provider, logger,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// GenerateThumbnail generates the configured thumbnail size for the source file
func GenerateThumbnail(
	ctx context.Context,
	config types.ThumbnailSize,
	mediaMetadata *types.MediaMetadata,
	absBasePath config.Path,
	db storage2.Database,
	provider storage2.Provider,
	logger *util.LogEntry,
) (errorReturn error) {

	img, err := readFile(ctx, provider, absBasePath, mediaMetadata)
	if err != nil {
		return err
	}

	tempDir, err := utils.CreateTempDir(absBasePath)
	if err != nil {
		return err
	}

	// Note: createThumbnail does locking based on activeThumbnailGeneration
	err = createThumbnail(
		ctx, absBasePath, tempDir, img, config, mediaMetadata, db, provider, logger,
	)
	if err != nil {
		return err
	}
	return nil
}

func readFile(ctx context.Context, provider storage2.Provider, absBasePath config.Path, mediaMetadata *types.MediaMetadata) (image.Image, error) {

	finalPath, err := utils.GetPathFromBase64Hash(mediaMetadata.Base64Hash, absBasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file path from metadata: %w", err)
	}

	downloadBucket := provider.GetBucket(mediaMetadata.IsPublic)
	downloader, finalizer, err := provider.DownloadFile(ctx, downloadBucket, types.Path(finalPath))
	if err != nil {
		return nil, err
	}
	defer finalizer()

	img, _, err := image.Decode(downloader)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func writeFile(img image.Image, dst string) (err error) {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer (func() { err = out.Close() })()

	return jpeg.Encode(out, img, &jpeg.Options{
		Quality: 85,
	})
}

// createThumbnail checks if the thumbnail exists, and if not, generates it
// Thumbnail generation is only done once for each non-existing thumbnail.
func createThumbnail(
	ctx context.Context,
	absBasePath config.Path,
	temporaryPath types.Path,
	img image.Image,
	config types.ThumbnailSize,
	mediaMetadata *types.MediaMetadata,
	db storage2.Database,
	provider storage2.Provider,
	logger *util.LogEntry,
) (errorReturn error) {
	logger = logger.With(
		"Width", config.Width,
		"Height", config.Height,
		"ResizeMethod", config.ResizeMethod,
	)

	// Check if request is larger than original
	if config.Width >= img.Bounds().Dx() && config.Height >= img.Bounds().Dy() {
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
	width, height, err := adjustSize(tempThumbnailPath, img, config.Width, config.Height, config.ResizeMethod == types.Crop, logger)
	if err != nil {
		return err
	}
	logger.With("ActualWidth", width,
		"ActualHeight", height,
		"processTime", time.Since(start),
	).Info("Generated thumbnail")

	stat, err := os.Stat(string(tempThumbnailPath))
	if err != nil {
		return err
	}

	thumbnailMetadata := &types.ThumbnailMetadata{
		MediaMetadata: &types.MediaMetadata{
			MediaID:  mediaMetadata.MediaID,
			ParentID: mediaMetadata.MediaID,

			// Note: the code currently always creates a JPEG thumbnail
			ContentType:   types.ContentType("image/jpeg"),
			FileSizeBytes: types.FileSizeBytes(stat.Size()),
			ThumbnailSize: &types.ThumbnailSize{
				Width:        config.Width,
				Height:       config.Height,
				ResizeMethod: config.ResizeMethod,
			},
		},
	}

	finalPath, duplicate, err := storage2.UploadFileWithHashCheck(ctx, provider, tempThumbnailPath, thumbnailMetadata.MediaMetadata, absBasePath, logger)
	if err != nil {
		return err
	}
	if duplicate {
		logger.WithField("dst", finalPath).Info("File was stored previously - discarding duplicate")
	}

	err = db.StoreThumbnail(ctx, thumbnailMetadata)
	if err != nil {
		logger.WithError(err).With(
			"ActualWidth", width,
			"ActualHeight", height,
		).Error("Failed to store thumbnail metadata in database.")
		return err
	}

	return nil
}

// adjustSize scales an image to fit within the provided width and height
// If the source aspect ratio is different to the target dimensions, one edge will be smaller than requested
// If crop is set to true, the image will be scaled to fill the width and height with any excess being cropped off
func adjustSize(dst types.Path, img image.Image, w, h int, crop bool, logger *util.LogEntry) (int, int, error) {
	var out image.Image
	var err error
	if crop {
		inAR := float64(img.Bounds().Dx()) / float64(img.Bounds().Dy())
		outAR := float64(w) / float64(h)

		var scaleW, scaleH uint
		if inAR > outAR {
			// input has shorter AR than requested output so use requested height and calculate width to match input AR
			scaleW = uint(float64(h) * inAR)
			scaleH = uint(h)
		} else {
			// input has taller AR than requested output so use requested width and calculate height to match input AR
			scaleW = uint(w)
			scaleH = uint(float64(w) / inAR)
		}

		scaled := resize.Resize(scaleW, scaleH, img, resize.Lanczos3)

		xoff := (scaled.Bounds().Dx() - w) / 2
		yoff := (scaled.Bounds().Dy() - h) / 2

		tr := image.Rect(0, 0, w, h)
		target := image.NewRGBA(tr)
		draw.Draw(target, tr, scaled, image.Pt(xoff, yoff), draw.Src)
		out = target
	} else {
		out = resize.Thumbnail(uint(w), uint(h), img, resize.Lanczos3)
	}

	if err = writeFile(out, string(dst)); err != nil {
		logger.WithError(err).Error("Failed to encode and write image")
		return -1, -1, err
	}

	return out.Bounds().Max.X, out.Bounds().Max.Y, nil
}
