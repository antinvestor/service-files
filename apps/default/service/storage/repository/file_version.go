package repository

import (
	"context"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

// FileVersionRepository defines the interface for file version operations
type FileVersionRepository interface {
	datastore.BaseRepository[*models.FileVersion]
	GetByMediaID(ctx context.Context, mediaID string) ([]*models.FileVersion, error)
	GetVersion(ctx context.Context, mediaID string, versionNumber int) (*models.FileVersion, error)
	GetVersionsPaginated(ctx context.Context, mediaID string, limit, offset int) ([]*models.FileVersion, int, error)
}

// NewFileVersionRepository creates a new file version repository instance
func NewFileVersionRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) FileVersionRepository {
	repo := fileVersionRepository{
		BaseRepository: datastore.NewBaseRepository[*models.FileVersion](
			ctx, dbPool, workMan, func() *models.FileVersion { return &models.FileVersion{} },
		),
	}
	return &repo
}

type fileVersionRepository struct {
	datastore.BaseRepository[*models.FileVersion]
}

// GetByMediaID retrieves all versions for a given media
func (r *fileVersionRepository) GetByMediaID(ctx context.Context, mediaID string) ([]*models.FileVersion, error) {
	var versions []*models.FileVersion
	err := r.Pool().DB(ctx, true).Where("media_id = ?", mediaID).
		Order("version_number DESC").
		Find(&versions).Error
	if err != nil {
		return nil, err
	}
	return versions, nil
}

// GetVersion retrieves a specific version by media ID and version number
func (r *fileVersionRepository) GetVersion(ctx context.Context, mediaID string, versionNumber int) (*models.FileVersion, error) {
	version := &models.FileVersion{}
	err := r.Pool().DB(ctx, true).Where("media_id = ? AND version_number = ?", mediaID, versionNumber).First(version).Error
	if err != nil {
		return nil, err
	}
	return version, nil
}

// GetVersionsPaginated retrieves versions with pagination
func (r *fileVersionRepository) GetVersionsPaginated(ctx context.Context, mediaID string, limit, offset int) ([]*models.FileVersion, int, error) {
	var versions []*models.FileVersion
	var count int64

	query := r.Pool().DB(ctx, true).Where("media_id = ?", mediaID)

	// Get total count
	if err := query.Model(&models.FileVersion{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := query.Order("version_number DESC").
		Limit(limit).
		Offset(offset).
		Find(&versions).Error
	if err != nil {
		return nil, 0, err
	}

	return versions, int(count), nil
}
