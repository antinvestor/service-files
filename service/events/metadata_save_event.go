package events

import (
	"context"
	"errors"
	"github.com/antinvestor/service-files/service/storage/models"
	"github.com/antinvestor/service-files/service/storage/repository"
	"github.com/pitabwire/frame"
)

type MediaMetadataSaveEvent struct {
	Service         *frame.Service
	MediaRepository repository.MediaRepository
}

func (fms *MediaMetadataSaveEvent) Name() string {
	return "file.metadata.save.event"
}

func (fms *MediaMetadataSaveEvent) PayloadType() any {
	return models.MediaMetadata{}
}

func (fms *MediaMetadataSaveEvent) Validate(_ context.Context, payload any) error {
	if _, ok := payload.(*models.MediaMetadata); !ok {
		return errors.New(" payload is not of type Media Metadata")
	}

	return nil
}

func (fms *MediaMetadataSaveEvent) Execute(ctx context.Context, payload any) error {
	metadata := payload.(*models.MediaMetadata)

	logger := fms.Service.L(ctx).WithField("payload", metadata).
		WithField("type", fms.Name())
	logger.Debug("handling file metadata save event")

	return fms.MediaRepository.Save(ctx, metadata)

}

func NewMetadataSaveHandler(service *frame.Service) frame.EventI {
	mediaRepository := repository.NewMediaRepository(service)
	return &MediaMetadataSaveEvent{service, mediaRepository}
}
