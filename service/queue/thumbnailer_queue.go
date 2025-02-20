package queue

import (
	"context"
	"encoding/json"
	"github.com/antinvestor/service-files/service/storage/models"
	"github.com/antinvestor/service-files/service/storage/repository"
	"github.com/pitabwire/frame"
)

type ThumbnailQueueHandler struct {
	Service         *frame.Service
	mediaRepository repository.MediaRepository
}

func (fq *ThumbnailQueueHandler) Handle(ctx context.Context, _ map[string]string, payload []byte) error {

	file := &models.MediaMetadata{}
	err := json.Unmarshal(payload, file)
	if err != nil {
		return err
	}

	return fq.mediaRepository.Save(ctx, file)

}

func NewThumbnailQueueHandler(service *frame.Service) ThumbnailQueueHandler {
	mediaRepo := repository.NewMediaRepository(service)
	return ThumbnailQueueHandler{service, mediaRepo}
}
