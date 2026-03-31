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

type MediaAuditSaveEvent struct {
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

	logger := util.Log(ctx).WithFields(map[string]any{
		"payload": audit,
		"type":    mas.Name(),
	})
	logger.Debug("handling file audit save event")

	err := mas.AuditRepository.Create(ctx, audit)
	if err != nil {
		if data.ErrorIsDuplicateKey(err) {
			logger.Debug("record already exists, skipping duplicate")
			return nil
		}
		return err
	}
	return nil
}

func NewAuditSaveHandler(auditRepository repository.MediaAuditRepository) events.EventI {
	return &MediaAuditSaveEvent{AuditRepository: auditRepository}
}
