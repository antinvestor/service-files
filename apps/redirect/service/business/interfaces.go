package business

import (
	"context"
	"errors"
	"time"

	"github.com/antinvestor/service-files/apps/redirect/service/models"
	"github.com/antinvestor/service-files/apps/redirect/service/repository"
	"github.com/pitabwire/frame/datastore/pool"
)

var ErrorInitializationFail = errors.New("business initialization failed: nil pool")

// LinkBusiness handles link CRUD operations.
type LinkBusiness interface {
	CreateLink(ctx context.Context, link *models.Link) (*models.Link, error)
	GetLink(ctx context.Context, id string) (*models.Link, error)
	GetLinkBySlug(ctx context.Context, slug string) (*models.Link, error)
	UpdateLink(ctx context.Context, id string, updates map[string]any) (*models.Link, error)
	DeleteLink(ctx context.Context, id string) error
	ListLinks(ctx context.Context, query string, affiliateID string, campaign string, state models.LinkState, limit int, offset int) ([]models.Link, error)
}

func NewLinkBusiness(dbPool pool.Pool) (LinkBusiness, error) {
	if dbPool == nil {
		return nil, ErrorInitializationFail
	}
	return &linkBusiness{
		linkRepo: repository.NewLinkRepository(dbPool),
	}, nil
}

// ClickBusiness handles click recording and statistics.
type ClickBusiness interface {
	RecordClick(ctx context.Context, click *models.Click) error
	GetStats(ctx context.Context, linkID string, startTime time.Time, endTime time.Time) (*LinkStats, error)
	ListClicks(ctx context.Context, linkID string, affiliateID string, startTime time.Time, endTime time.Time, limit int, offset int) ([]models.Click, error)
}

func NewClickBusiness(dbPool pool.Pool) (ClickBusiness, error) {
	if dbPool == nil {
		return nil, ErrorInitializationFail
	}
	return &clickBusiness{
		clickRepo: repository.NewClickRepository(dbPool),
		linkRepo:  repository.NewLinkRepository(dbPool),
	}, nil
}

// LinkStats holds aggregated click statistics for a link.
type LinkStats struct {
	LinkID           string
	TotalClicks      int64
	UniqueClicks     int64
	Referrers        map[string]int64
	Countries        map[string]int64
	Devices          map[string]int64
	Browsers         map[string]int64
	OperatingSystems map[string]int64
	ClicksPerDay     map[string]int64
}
