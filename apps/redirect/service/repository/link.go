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

type LinkRepository interface {
	datastore.BaseRepository[*models.Link]
	GetBySlug(ctx context.Context, slug string) (*models.Link, error)
	SearchLinks(ctx context.Context, query string, affiliateID string, campaign string, state models.LinkState, limit int, offset int) ([]models.Link, error)
	SoftDelete(ctx context.Context, id string) error
	IncrementClickCount(ctx context.Context, id string, totalDelta int64, uniqueDelta int64) error
	// RecordCheck persists the outcome of a destination probe. Caller
	// supplies the new consecutiveFailures counter (0 on success, n+1
	// on failure) so this one call covers both transitions.
	RecordCheck(ctx context.Context, id string, status int, consecutiveFailures int) error
	// ExpireLink flips state to LinkStateExpired. Used when the probe
	// determines the destination is permanently gone.
	ExpireLink(ctx context.Context, id string) error
}

type linkRepository struct {
	datastore.BaseRepository[*models.Link]
}

func NewLinkRepository(dbPool pool.Pool) LinkRepository {
	return &linkRepository{
		BaseRepository: datastore.NewBaseRepository[*models.Link](
			context.Background(), dbPool, nil,
			func() *models.Link { return &models.Link{} },
		),
	}
}

func (repo *linkRepository) GetBySlug(ctx context.Context, slug string) (*models.Link, error) {
	link := models.Link{}
	err := repo.Pool().DB(ctx, true).Where("slug = ?", slug).First(&link).Error
	if err != nil {
		return nil, fmt.Errorf("get link by slug: %w", err)
	}
	return &link, nil
}

func (repo *linkRepository) SearchLinks(ctx context.Context, query string, affiliateID string, campaign string, state models.LinkState, limit int, offset int) ([]models.Link, error) {
	var links []models.Link

	db := repo.Pool().DB(ctx, true).Model(&models.Link{})

	if query != "" {
		q := "%" + query + "%"
		db = db.Where("slug ILIKE ? OR title ILIKE ? OR destination_url ILIKE ?", q, q, q)
	}
	if affiliateID != "" {
		db = db.Where("affiliate_id = ?", affiliateID)
	}
	if campaign != "" {
		db = db.Where("campaign = ?", campaign)
	}
	if state != models.LinkStateUnspecified {
		db = db.Where("state = ?", state)
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&links).Error
	if err != nil {
		return nil, fmt.Errorf("search links: %w", err)
	}
	return links, nil
}

func (repo *linkRepository) SoftDelete(ctx context.Context, id string) error {
	result := repo.Pool().DB(ctx, false).Model(&models.Link{}).Where("id = ?", id).
		Update("state", models.LinkStateDeleted)
	if result.Error != nil {
		return fmt.Errorf("soft delete link: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (repo *linkRepository) RecordCheck(ctx context.Context, id string, status int, consecutiveFailures int) error {
	updates := map[string]any{
		"last_checked_at":            time.Now().UTC(),
		"last_check_status":          status,
		"consecutive_check_failures": consecutiveFailures,
	}
	result := repo.Pool().DB(ctx, false).Model(&models.Link{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("record link check: %w", result.Error)
	}
	return nil
}

func (repo *linkRepository) ExpireLink(ctx context.Context, id string) error {
	result := repo.Pool().DB(ctx, false).Model(&models.Link{}).Where("id = ?", id).
		Update("state", models.LinkStateExpired)
	if result.Error != nil {
		return fmt.Errorf("expire link: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (repo *linkRepository) IncrementClickCount(ctx context.Context, id string, totalDelta int64, uniqueDelta int64) error {
	updates := map[string]any{
		"click_count": gorm.Expr("click_count + ?", totalDelta),
	}
	if uniqueDelta > 0 {
		updates["unique_click_count"] = gorm.Expr("unique_click_count + ?", uniqueDelta)
	}

	result := repo.Pool().DB(ctx, false).Model(&models.Link{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("increment click count: %w", result.Error)
	}
	return nil
}
