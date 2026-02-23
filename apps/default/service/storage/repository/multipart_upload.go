package repository

import (
	"context"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

// MultipartUploadRepository defines the interface for multipart upload operations
type MultipartUploadRepository interface {
	datastore.BaseRepository[*models.MultipartUpload]
	GetByUploadID(ctx context.Context, uploadID string) (*models.MultipartUpload, error)
	GetByMediaID(ctx context.Context, mediaID string) (*models.MultipartUpload, error)
	UpdateState(ctx context.Context, uploadID string, state string) error
	HardDeleteByID(ctx context.Context, uploadID string) error
	GetExpiredUploads(ctx context.Context, before time.Time) ([]*models.MultipartUpload, error)
}

// NewMultipartUploadRepository creates a new multipart upload repository instance
func NewMultipartUploadRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) MultipartUploadRepository {
	repo := multipartUploadRepository{
		BaseRepository: datastore.NewBaseRepository[*models.MultipartUpload](
			ctx, dbPool, workMan, func() *models.MultipartUpload { return &models.MultipartUpload{} },
		),
	}
	return &repo
}

type multipartUploadRepository struct {
	datastore.BaseRepository[*models.MultipartUpload]
}

// GetByUploadID retrieves a multipart upload by its ID
func (r *multipartUploadRepository) GetByUploadID(ctx context.Context, uploadID string) (*models.MultipartUpload, error) {
	upload := &models.MultipartUpload{}
	err := r.Pool().DB(ctx, true).First(upload, "id = ?", uploadID).Error
	if err != nil {
		return nil, err
	}
	return upload, nil
}

// GetByMediaID retrieves a multipart upload by its media ID
func (r *multipartUploadRepository) GetByMediaID(ctx context.Context, mediaID string) (*models.MultipartUpload, error) {
	upload := &models.MultipartUpload{}
	err := r.Pool().DB(ctx, true).First(upload, "media_id = ?", mediaID).Error
	if err != nil {
		return nil, err
	}
	return upload, nil
}

// UpdateState updates the state of a multipart upload
func (r *multipartUploadRepository) UpdateState(ctx context.Context, uploadID string, state string) error {
	return r.Pool().DB(ctx, true).Model(&models.MultipartUpload{}).
		Where("id = ?", uploadID).
		Update("upload_state", state).Error
}

// HardDeleteByID permanently deletes a multipart upload by ID
func (r *multipartUploadRepository) HardDeleteByID(ctx context.Context, uploadID string) error {
	return r.Pool().DB(ctx, true).Delete(&models.MultipartUpload{}, "id = ?", uploadID).Error
}

// GetExpiredUploads retrieves all uploads that have expired before the given time
func (r *multipartUploadRepository) GetExpiredUploads(ctx context.Context, before time.Time) ([]*models.MultipartUpload, error) {
	var uploads []*models.MultipartUpload
	err := r.Pool().DB(ctx, true).Where("expires_at < ?", before).Find(&uploads).Error
	if err != nil {
		return nil, err
	}
	return uploads, nil
}
