package repository

import (
	"context"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

// StorageStatsRepository defines the interface for storage statistics operations
type StorageStatsRepository interface {
	datastore.BaseRepository[*models.StorageStats]
	GetByDate(ctx context.Context, date time.Time) (*models.StorageStats, error)
	GetRange(ctx context.Context, start, end time.Time) ([]*models.StorageStats, error)
	GetLatest(ctx context.Context) (*models.StorageStats, error)
	AggregateCurrentStats(ctx context.Context) (*models.StorageStats, error)
}

// NewStorageStatsRepository creates a new storage stats repository instance
func NewStorageStatsRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) StorageStatsRepository {
	repo := storageStatsRepository{
		BaseRepository: datastore.NewBaseRepository[*models.StorageStats](
			ctx, dbPool, workMan, func() *models.StorageStats { return &models.StorageStats{} },
		),
	}
	return &repo
}

type storageStatsRepository struct {
	datastore.BaseRepository[*models.StorageStats]
}

// GetByDate retrieves storage stats for a given date
func (r *storageStatsRepository) GetByDate(ctx context.Context, date time.Time) (*models.StorageStats, error) {
	stats := &models.StorageStats{}
	truncatedDate := date.Truncate(24 * time.Hour)
	err := r.Pool().DB(ctx, true).Where("date(created_at) = date(?)", truncatedDate).First(stats).Error
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// GetRange retrieves storage stats for a date range
func (r *storageStatsRepository) GetRange(ctx context.Context, start, end time.Time) ([]*models.StorageStats, error) {
	var statsList []*models.StorageStats
	err := r.Pool().DB(ctx, true).Where("created_at >= ? AND created_at <= ?", start, end).
		Order("created_at ASC").
		Find(&statsList).Error
	if err != nil {
		return nil, err
	}
	return statsList, nil
}

// GetLatest retrieves the most recent storage stats
func (r *storageStatsRepository) GetLatest(ctx context.Context) (*models.StorageStats, error) {
	stats := &models.StorageStats{}
	err := r.Pool().DB(ctx, true).Order("created_at DESC").First(stats).Error
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// AggregateCurrentStats calculates current stats from the media table
func (r *storageStatsRepository) AggregateCurrentStats(ctx context.Context) (*models.StorageStats, error) {
	stats := &models.StorageStats{}

	// Aggregate stats from media_metadata table
	type result struct {
		TotalSize    int64
		FileCount    int
		UserCount    int
		PublicBytes  int64
		PrivateBytes int64
	}
	var agg result

	err := r.Pool().DB(ctx, true).Model(&models.MediaMetadata{}).
		Select(`
			COALESCE(SUM(size), 0) as total_size,
			COUNT(*) as file_count,
			COUNT(DISTINCT owner_id) as user_count,
			COALESCE(SUM(CASE WHEN public = true THEN size ELSE 0 END), 0) as public_bytes,
			COALESCE(SUM(CASE WHEN public = false THEN size ELSE 0 END), 0) as private_bytes
		`).
		Scan(&agg).Error
	if err != nil {
		return nil, err
	}

	stats.TotalBytes = agg.TotalSize
	stats.FileCount = agg.FileCount
	stats.UserCount = agg.UserCount
	stats.PublicBytes = agg.PublicBytes
	stats.PrivateBytes = agg.PrivateBytes

	return stats, nil
}
