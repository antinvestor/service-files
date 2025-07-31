package repository

import (
	"context"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/pitabwire/frame"
)

type MediaAuditRepository interface {
	GetByID(ctx context.Context, id string) (*models.MediaAudit, error)
	Save(ctx context.Context, file *models.MediaAudit) error
	Delete(ctx context.Context, id string) error
}

func NewMediaAuditRepository(service *frame.Service) MediaAuditRepository {
	fileAuditRepo := fileAuditRepository{
		service: service,
	}
	return &fileAuditRepo
}

type fileAuditRepository struct {
	service *frame.Service
}

func (far *fileAuditRepository) GetByID(ctx context.Context, id string) (*models.MediaAudit, error) {
	file := &models.MediaAudit{}
	err := far.service.DB(ctx, true).First(file, " id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (far *fileAuditRepository) Save(ctx context.Context, file *models.MediaAudit) error {
	return far.service.DB(ctx, false).Save(file).Error
}

func (far *fileAuditRepository) Delete(ctx context.Context, id string) error {

	auditFile, err := far.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return far.service.DB(ctx, false).Delete(auditFile).Error
}
