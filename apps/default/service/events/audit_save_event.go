package events

import (
	"context"
	"errors"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/pitabwire/frame"
)

type MediaAuditSaveEvent struct {
	Service         *frame.Service
	AuditRepository repository.MediaAuditRepository
}

func (mas *MediaAuditSaveEvent) Name() string {
	return "media.audit.save.event"
}

func (mas *MediaAuditSaveEvent) PayloadType() any {
	return models.MediaAudit{}
}

func (mas *MediaAuditSaveEvent) Validate(_ context.Context, payload any) error {
	if _, ok := payload.(*models.MediaAudit); !ok {
		return errors.New(" payload is not of type Media Audit")
	}

	return nil
}

func (mas *MediaAuditSaveEvent) Execute(ctx context.Context, payload any) error {
	audit := payload.(*models.MediaAudit)

	logger := mas.Service.Log(ctx).WithField("payload", audit).
		WithField("type", mas.Name())
	logger.Debug("handling file audit save event")

	return mas.AuditRepository.Save(ctx, audit)

}

func NewAuditSaveHandler(service *frame.Service) frame.EventI {
	auditRepository := repository.NewMediaAuditRepository(service)
	return &MediaAuditSaveEvent{service, auditRepository}
}
