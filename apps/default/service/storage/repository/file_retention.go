package repository

import (
	"context"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

// FileRetentionRepository defines the interface for file retention operations
type FileRetentionRepository interface {
	datastore.BaseRepository[*models.FileRetention]
	GetByID(ctx context.Context, id string) (*models.FileRetention, error)
	GetByMediaID(ctx context.Context, mediaID string) (*models.FileRetention, error)
	DeleteByMediaID(ctx context.Context, mediaID string) error
	GetExpired(ctx context.Context, before time.Time) ([]*models.FileRetention, error)
	UpdateLocked(ctx context.Context, mediaID string, locked bool) error
}

// NewFileRetentionRepository creates a new file retention repository instance
func NewFileRetentionRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) FileRetentionRepository {
	repo := fileRetentionRepository{
		BaseRepository: datastore.NewBaseRepository[*models.FileRetention](
			ctx, dbPool, workMan, func() *models.FileRetention { return &models.FileRetention{} },
		),
	}
	return &repo
}

type fileRetentionRepository struct {
	datastore.BaseRepository[*models.FileRetention]
}

// GetByID retrieves retention info by ID
func (r *fileRetentionRepository) GetByID(ctx context.Context, id string) (*models.FileRetention, error) {
	retention := &models.FileRetention{}
	err := r.Pool().DB(ctx, true).First(retention, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return retention, nil
}

// GetByMediaID retrieves retention info for a given media
func (r *fileRetentionRepository) GetByMediaID(ctx context.Context, mediaID string) (*models.FileRetention, error) {
	retention := &models.FileRetention{}
	err := r.Pool().DB(ctx, true).First(retention, "media_id = ?", mediaID).Error
	if err != nil {
		return nil, nil
	}
	return retention, nil
}

// DeleteByMediaID deletes retention info for a given media
func (r *fileRetentionRepository) DeleteByMediaID(ctx context.Context, mediaID string) error {
	return r.Pool().DB(ctx, false).Delete(&models.FileRetention{}, "media_id = ?", mediaID).Error
}

// GetExpired retrieves all retentions that have expired before the given time
func (r *fileRetentionRepository) GetExpired(ctx context.Context, before time.Time) ([]*models.FileRetention, error) {
	var retentions []*models.FileRetention
	err := r.Pool().DB(ctx, true).Where("expires_at < ?", before).Find(&retentions).Error
	if err != nil {
		return nil, err
	}
	return retentions, nil
}

// UpdateLocked updates the locked status of a file retention
func (r *fileRetentionRepository) UpdateLocked(ctx context.Context, mediaID string, locked bool) error {
	return r.Pool().DB(ctx, false).Table("file_retentions").
		Where("media_id = ? AND deleted_at IS NULL", mediaID).
		Update("is_locked", locked).Error
}
