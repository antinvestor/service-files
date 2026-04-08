package business

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/antinvestor/service-files/apps/redirect/service/models"
	"github.com/antinvestor/service-files/apps/redirect/service/repository"
	"github.com/pitabwire/util"
)

const uniqueClickWindow = 24 * time.Hour

type clickBusiness struct {
	clickRepo repository.ClickRepository
	linkRepo  repository.LinkRepository
}

func (cb *clickBusiness) RecordClick(ctx context.Context, click *models.Click) error {
	log := util.Log(ctx)

	click.GenID(ctx)
	ParseUserAgent(click)

	// Check uniqueness BEFORE inserting the click so the count query
	// does not include the click we are about to insert.
	isUnique := true
	if click.IPAddress != "" {
		exists, err := cb.clickRepo.ExistsRecentByIP(ctx, click.LinkID, click.IPAddress, uniqueClickWindow)
		if err != nil {
			log.Warn("uniqueness check failed, assuming unique", "error", err, "link_id", click.LinkID)
		} else if exists {
			isUnique = false
		}
	}

	if err := cb.clickRepo.Create(ctx, click); err != nil {
		return fmt.Errorf("record click for link %s: %w", click.LinkID, err)
	}

	var uniqueDelta int64
	if isUnique {
		uniqueDelta = 1
	}
	if err := cb.linkRepo.IncrementClickCount(ctx, click.LinkID, 1, uniqueDelta); err != nil {
		log.Warn("failed to increment click count", "error", err, "link_id", click.LinkID)
	}

	return nil
}

func (cb *clickBusiness) GetStats(ctx context.Context, linkID string, startTime time.Time, endTime time.Time) (*LinkStats, error) {
	log := util.Log(ctx)

	total, err := cb.clickRepo.CountByLinkID(ctx, linkID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get stats for link %s: %w", linkID, err)
	}

	unique, err := cb.clickRepo.CountUniqueByLinkID(ctx, linkID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get unique stats for link %s: %w", linkID, err)
	}

	referrers, err := cb.clickRepo.AggregateByField(ctx, linkID, "referer", startTime, endTime)
	if err != nil {
		log.Warn("failed to aggregate referrers", "error", err, "link_id", linkID)
	}
	countries, err := cb.clickRepo.AggregateByField(ctx, linkID, "country", startTime, endTime)
	if err != nil {
		log.Warn("failed to aggregate countries", "error", err, "link_id", linkID)
	}
	devices, err := cb.clickRepo.AggregateByField(ctx, linkID, "device_type", startTime, endTime)
	if err != nil {
		log.Warn("failed to aggregate devices", "error", err, "link_id", linkID)
	}
	browsers, err := cb.clickRepo.AggregateByField(ctx, linkID, "browser", startTime, endTime)
	if err != nil {
		log.Warn("failed to aggregate browsers", "error", err, "link_id", linkID)
	}
	oses, err := cb.clickRepo.AggregateByField(ctx, linkID, "os", startTime, endTime)
	if err != nil {
		log.Warn("failed to aggregate operating systems", "error", err, "link_id", linkID)
	}
	perDay, err := cb.clickRepo.AggregateByDay(ctx, linkID, startTime, endTime)
	if err != nil {
		log.Warn("failed to aggregate clicks per day", "error", err, "link_id", linkID)
	}

	return &LinkStats{
		LinkID:           linkID,
		TotalClicks:      total,
		UniqueClicks:     unique,
		Referrers:        referrers,
		Countries:        countries,
		Devices:          devices,
		Browsers:         browsers,
		OperatingSystems: oses,
		ClicksPerDay:     perDay,
	}, nil
}

func (cb *clickBusiness) ListClicks(ctx context.Context, linkID string, affiliateID string, startTime time.Time, endTime time.Time, limit int, offset int) ([]models.Click, error) {
	if affiliateID != "" {
		clicks, err := cb.clickRepo.ListByAffiliateID(ctx, affiliateID, startTime, endTime, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("list clicks by affiliate %s: %w", affiliateID, err)
		}
		return clicks, nil
	}

	clicks, err := cb.clickRepo.ListByLinkID(ctx, linkID, startTime, endTime, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list clicks for link %s: %w", linkID, err)
	}
	return clicks, nil
}

// ParseUserAgent does lightweight user-agent classification.
func ParseUserAgent(click *models.Click) {
	ua := strings.ToLower(click.UserAgent)
	if ua == "" {
		return
	}
	click.DeviceType = detectDevice(ua)
	click.Browser = detectBrowser(ua)
	click.OS = detectOS(ua)
}

func detectDevice(ua string) models.DeviceType {
	switch {
	case strings.Contains(ua, "bot") || strings.Contains(ua, "crawler") || strings.Contains(ua, "spider"):
		return models.DeviceTypeBot
	case strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad"):
		return models.DeviceTypeTablet
	case strings.Contains(ua, "mobile") || strings.Contains(ua, "android"):
		return models.DeviceTypeMobile
	default:
		return models.DeviceTypeDesktop
	}
}

func detectBrowser(ua string) string {
	switch {
	case strings.Contains(ua, "edg"):
		return "Edge"
	case strings.Contains(ua, "opr") || strings.Contains(ua, "opera"):
		return "Opera"
	case strings.Contains(ua, "chrome") || strings.Contains(ua, "crios"):
		return "Chrome"
	case strings.Contains(ua, "firefox") || strings.Contains(ua, "fxios"):
		return "Firefox"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		return "Safari"
	default:
		return "Other"
	}
}

func detectOS(ua string) string {
	switch {
	case strings.Contains(ua, "windows"):
		return "Windows"
	case strings.Contains(ua, "mac os") || strings.Contains(ua, "macos"):
		return "macOS"
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ios"):
		return "iOS"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return "Other"
	}
}
