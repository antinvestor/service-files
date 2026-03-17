package events

import (
	"context"
	"errors"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/events"
	"github.com/pitabwire/util"
)

type MediaMetadataSaveEvent struct {
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

	logger := util.Log(ctx).WithField("payload", metadata).
		WithField("type", fms.Name())
	logger.Debug("handling file metadata save event")

	err := fms.MediaRepository.Create(ctx, metadata)
	if err != nil {
		if data.ErrorIsDuplicateKey(err) {
			logger.Debug("record already exists, skipping duplicate")
			return nil
		}
		logger.WithError(err).Error("could not save to db")
		return err
	}
	return nil
}

func NewMetadataSaveHandler(mediaRepository repository.MediaRepository) events.EventI {
	return &MediaMetadataSaveEvent{MediaRepository: mediaRepository}
}
