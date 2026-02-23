package repository

import (
	"context"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

// MultipartUploadPartRepository defines the interface for multipart upload part operations
type MultipartUploadPartRepository interface {
	datastore.BaseRepository[*models.MultipartUploadPart]
	GetByUploadID(ctx context.Context, uploadID string) ([]*models.MultipartUploadPart, error)
	GetPart(ctx context.Context, uploadID string, partNumber int) (*models.MultipartUploadPart, error)
	DeleteByUploadID(ctx context.Context, uploadID string) error
}

// NewMultipartUploadPartRepository creates a new multipart upload part repository instance
func NewMultipartUploadPartRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) MultipartUploadPartRepository {
	repo := multipartUploadPartRepository{
		BaseRepository: datastore.NewBaseRepository[*models.MultipartUploadPart](
			ctx, dbPool, workMan, func() *models.MultipartUploadPart { return &models.MultipartUploadPart{} },
		),
	}
	return &repo
}

type multipartUploadPartRepository struct {
	datastore.BaseRepository[*models.MultipartUploadPart]
}

// GetByUploadID retrieves all parts for a given upload
func (r *multipartUploadPartRepository) GetByUploadID(ctx context.Context, uploadID string) ([]*models.MultipartUploadPart, error) {
	var parts []*models.MultipartUploadPart
	err := r.Pool().DB(ctx, true).Where("upload_id = ?", uploadID).
		Order("part_number ASC").
		Find(&parts).Error
	if err != nil {
		return nil, err
	}
	return parts, nil
}

// GetPart retrieves a specific part by upload ID and part number
func (r *multipartUploadPartRepository) GetPart(ctx context.Context, uploadID string, partNumber int) (*models.MultipartUploadPart, error) {
	part := &models.MultipartUploadPart{}
	err := r.Pool().DB(ctx, true).Where("upload_id = ? AND part_number = ?", uploadID, partNumber).First(part).Error
	if err != nil {
		return nil, err
	}
	return part, nil
}

// DeleteByUploadID deletes all parts for a given upload
func (r *multipartUploadPartRepository) DeleteByUploadID(ctx context.Context, uploadID string) error {
	return r.Pool().DB(ctx, true).Delete(&models.MultipartUploadPart{}, "upload_id = ?", uploadID).Error
}
