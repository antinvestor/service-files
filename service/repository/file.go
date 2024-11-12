package repository

import (
	"context"
	"github.com/antinvestor/service-files/service/models"
	"github.com/pitabwire/frame"
)

type FileRepository interface {
	GetByID(ctx context.Context, id string) (*models.File, error)
	GetBySubscriptionAndGroup(ctx context.Context, subscriptionId string, groupId string, page int32, limit int32) ([]*models.File, error)
	Save(ctx context.Context, file *models.File) error
	Delete(ctx context.Context, id string) error
}

func NewFileRepository(service *frame.Service) FileRepository {
	fileRepo := fileRepository{
		service: service,
	}
	return &fileRepo
}

type fileRepository struct {
	service *frame.Service
}

func (ar *fileRepository) GetByID(ctx context.Context, id string) (*models.File, error) {
	file := &models.File{}
	err := ar.service.DB(ctx, true).First(file, " id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (ar *fileRepository) GetBySubscriptionAndGroup(ctx context.Context, subscriptionId string, groupId string, page int32, limit int32) ([]*models.File, error) {
	fileList := make([]*models.File, 0)
	tx := ar.service.DB(ctx, true).Where(" subscription_id = ?", subscriptionId)

	if groupId != "" {
		tx = tx.Where("group_id = ?", groupId)
	}

	tx = tx.Offset(int(page))
	tx = tx.Limit(int(limit))

	tx.Find(&fileList)

	return fileList, nil
}

func (ar *fileRepository) Save(ctx context.Context, file *models.File) error {
	return ar.service.DB(ctx, false).Save(file).Error
}

func (ar *fileRepository) Delete(ctx context.Context, id string) error {

	file, err := ar.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return ar.service.DB(ctx, false).Delete(file).Error
}
