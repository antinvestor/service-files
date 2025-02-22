package queue

import (
	"context"
	"encoding/json"
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/queue/thumbnailer"
	"github.com/antinvestor/service-files/service/storage"
	"github.com/antinvestor/service-files/service/types"
	"github.com/pitabwire/frame"
	"strings"
)

type ThumbnailQueueHandler struct {
	service       *frame.Service
	mediaDatabase storage.Database
	provider      storage.Provider
}

func (fq *ThumbnailQueueHandler) Handle(ctx context.Context, _ map[string]string, payload []byte) error {

	logger := fq.service.L(ctx)

	mediaPayload := map[string]string{}
	err := json.Unmarshal(payload, &mediaPayload)
	if err != nil {
		return err
	}

	mediaMetadata, err := fq.mediaDatabase.GetMediaMetadata(ctx, types.MediaID(mediaPayload["media_id"]))
	if err != nil {
		return err
	}

	if !strings.HasPrefix(string(mediaMetadata.ContentType), "image") {
		return nil
	}

	cfg := fq.service.Config().(*config.FilesConfig)

	thumbnailSizes := cfg.ThumbnailSizes

	err = thumbnailer.GenerateThumbnails(
		ctx, thumbnailSizes, mediaMetadata, cfg.AbsBasePath, fq.mediaDatabase, fq.provider, logger,
	)
	if err != nil {
		logger.WithError(err).Warn("Error generating thumbnails")
	}

	return nil

}

func NewThumbnailQueueHandler(service *frame.Service, mediaDatabase storage.Database, mediaProvider storage.Provider) ThumbnailQueueHandler {
	return ThumbnailQueueHandler{
		service:       service,
		mediaDatabase: mediaDatabase,
		provider:      mediaProvider,
	}
}
