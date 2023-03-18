package repository

import (
	"context"
	"github.com/antinvestor/files/service/models"
	"github.com/pitabwire/frame"
)

type FileAuditRepository interface {
	GetByID(ctx context.Context, id string) (*models.FileAudit, error)
	Save(ctx context.Context, file *models.FileAudit) error
	Delete(ctx context.Context, id string) error
}

func NewFileAuditRepository(service *frame.Service) FileAuditRepository {
	fileAuditRepo := fileAuditRepository{
		service: service,
	}
	return &fileAuditRepo
}

type fileAuditRepository struct {
	service *frame.Service
}

func (far *fileAuditRepository) GetByID(ctx context.Context, id string) (*models.FileAudit, error) {
	file := &models.FileAudit{}
	err := far.service.DB(ctx, true).First(file, " id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (far *fileAuditRepository) Save(ctx context.Context, file *models.FileAudit) error {
	return far.service.DB(ctx, false).Save(file).Error
}

func (far *fileAuditRepository) Delete(ctx context.Context, id string) error {

	auditFile, err := far.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return far.service.DB(ctx, false).Delete(auditFile).Error
}
