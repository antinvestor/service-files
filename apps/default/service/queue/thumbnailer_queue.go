package queue

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/queue/thumbnailer"
	storage2 "github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame"
)

type ThumbnailQueueHandler struct {
	service       *frame.Service
	mediaDatabase storage2.Database
	provider      storage2.Provider
}

func (fq *ThumbnailQueueHandler) Handle(ctx context.Context, _ map[string]string, payload []byte) error {

	logger := fq.service.Log(ctx)

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

func NewThumbnailQueueHandler(service *frame.Service, mediaDatabase storage2.Database, mediaProvider storage2.Provider) ThumbnailQueueHandler {
	return ThumbnailQueueHandler{
		service:       service,
		mediaDatabase: mediaDatabase,
		provider:      mediaProvider,
	}
}
