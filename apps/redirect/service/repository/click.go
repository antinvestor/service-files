package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/antinvestor/service-files/apps/redirect/service/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"gorm.io/gorm"
)

// allowedAggregateColumns whitelists columns safe for aggregation queries.
var allowedAggregateColumns = map[string]bool{
	"referer":     true,
	"country":     true,
	"city":        true,
	"device_type": true,
	"browser":     true,
	"os":          true,
	"slug":        true,
}

type ClickRepository interface {
	datastore.BaseRepository[*models.Click]
	ListByLinkID(ctx context.Context, linkID string, startTime time.Time, endTime time.Time, limit int, offset int) ([]models.Click, error)
	ListByAffiliateID(ctx context.Context, affiliateID string, startTime time.Time, endTime time.Time, limit int, offset int) ([]models.Click, error)
	CountByLinkID(ctx context.Context, linkID string, startTime time.Time, endTime time.Time) (int64, error)
	CountUniqueByLinkID(ctx context.Context, linkID string, startTime time.Time, endTime time.Time) (int64, error)
	ExistsRecentByIP(ctx context.Context, linkID string, ip string, within time.Duration) (bool, error)
	AggregateByField(ctx context.Context, linkID string, field string, startTime time.Time, endTime time.Time) (map[string]int64, error)
	AggregateByDay(ctx context.Context, linkID string, startTime time.Time, endTime time.Time) (map[string]int64, error)
}

type clickRepository struct {
	datastore.BaseRepository[*models.Click]
}

func NewClickRepository(dbPool pool.Pool) ClickRepository {
	return &clickRepository{
		BaseRepository: datastore.NewBaseRepository[*models.Click](
			context.Background(), dbPool, nil,
			func() *models.Click { return &models.Click{} },
		),
	}
}

func (repo *clickRepository) ListByLinkID(ctx context.Context, linkID string, startTime time.Time, endTime time.Time, limit int, offset int) ([]models.Click, error) {
	var clicks []models.Click
	db := repo.Pool().DB(ctx, true).Where("link_id = ?", linkID)
	db = applyTimeRange(db, startTime, endTime)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&clicks).Error
	if err != nil {
		return nil, fmt.Errorf("list clicks by link: %w", err)
	}
	return clicks, nil
}

func (repo *clickRepository) ListByAffiliateID(ctx context.Context, affiliateID string, startTime time.Time, endTime time.Time, limit int, offset int) ([]models.Click, error) {
	var clicks []models.Click
	db := repo.Pool().DB(ctx, true).Where("affiliate_id = ?", affiliateID)
	db = applyTimeRange(db, startTime, endTime)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&clicks).Error
	if err != nil {
		return nil, fmt.Errorf("list clicks by affiliate: %w", err)
	}
	return clicks, nil
}

func (repo *clickRepository) CountByLinkID(ctx context.Context, linkID string, startTime time.Time, endTime time.Time) (int64, error) {
	var count int64
	db := repo.Pool().DB(ctx, true).Model(&models.Click{}).Where("link_id = ?", linkID)
	db = applyTimeRange(db, startTime, endTime)
	err := db.Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count clicks by link: %w", err)
	}
	return count, nil
}

func (repo *clickRepository) CountUniqueByLinkID(ctx context.Context, linkID string, startTime time.Time, endTime time.Time) (int64, error) {
	var count int64
	db := repo.Pool().DB(ctx, true).Model(&models.Click{}).Where("link_id = ?", linkID)
	db = applyTimeRange(db, startTime, endTime)
	err := db.Distinct("ip_address").Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count unique clicks by link: %w", err)
	}
	return count, nil
}

func (repo *clickRepository) ExistsRecentByIP(ctx context.Context, linkID string, ip string, within time.Duration) (bool, error) {
	cutoff := time.Now().Add(-within)
	var id string
	err := repo.Pool().DB(ctx, true).Model(&models.Click{}).
		Select("id").
		Where("link_id = ? AND ip_address = ? AND created_at > ?", linkID, ip, cutoff).
		Limit(1).
		Pluck("id", &id).Error
	if err != nil {
		return false, fmt.Errorf("check recent click by IP: %w", err)
	}
	return id != "", nil
}

func (repo *clickRepository) AggregateByField(ctx context.Context, linkID string, field string, startTime time.Time, endTime time.Time) (map[string]int64, error) {
	if !allowedAggregateColumns[field] {
		return nil, fmt.Errorf("aggregate by field: column %q not allowed", field)
	}

	type result struct {
		Value string
		Count int64
	}
	var results []result

	db := repo.Pool().DB(ctx, true).Model(&models.Click{}).
		Select(field+" AS value, COUNT(*) AS count").
		Where("link_id = ?", linkID)
	db = applyTimeRange(db, startTime, endTime)
	err := db.Group(field).Order("count DESC").Limit(50).Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("aggregate clicks by %s: %w", field, err)
	}

	m := make(map[string]int64, len(results))
	for _, r := range results {
		if r.Value != "" {
			m[r.Value] = r.Count
		}
	}
	return m, nil
}

func (repo *clickRepository) AggregateByDay(ctx context.Context, linkID string, startTime time.Time, endTime time.Time) (map[string]int64, error) {
	type result struct {
		Day   string
		Count int64
	}
	var results []result

	db := repo.Pool().DB(ctx, true).Model(&models.Click{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') AS day, COUNT(*) AS count").
		Where("link_id = ?", linkID)
	db = applyTimeRange(db, startTime, endTime)
	err := db.Group("day").Order("day ASC").Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("aggregate clicks by day: %w", err)
	}

	m := make(map[string]int64, len(results))
	for _, r := range results {
		m[r.Day] = r.Count
	}
	return m, nil
}

func applyTimeRange(db *gorm.DB, startTime time.Time, endTime time.Time) *gorm.DB {
	if !startTime.IsZero() {
		db = db.Where("created_at >= ?", startTime)
	}
	if !endTime.IsZero() {
		db = db.Where("created_at <= ?", endTime)
	}
	return db
}
