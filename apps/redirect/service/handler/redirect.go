package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/antinvestor/service-files/apps/redirect/service/business"
	"github.com/antinvestor/service-files/apps/redirect/service/models"
	"github.com/antinvestor/service-files/apps/redirect/service/repository"
	"github.com/pitabwire/frame/cache"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/util"
)

const (
	clickBufferSize  = 8192
	clickWorkerCount = 4
	batchSize        = 100
	flushInterval    = 500 * time.Millisecond
	linkCacheTTL     = 2 * time.Minute
	uniqueIPTTL      = 24 * time.Hour

	// CacheName is the Frame cache name used by this handler.
	CacheName = "redirect"
)

// clickCounts tracks accumulated click deltas per link for batch flushing.
type clickCounts struct {
	total  int64
	unique int64
}

// RedirectHandler serves fast HTTP redirects and records clicks asynchronously.
type RedirectHandler struct {
	linkBiz  business.LinkBusiness
	rawCache cache.RawCache
	dbPool   pool.Pool

	clickCh chan *models.Click
	wg      sync.WaitGroup
}

func NewRedirectHandler(
	linkBiz business.LinkBusiness,
	rawCache cache.RawCache,
	dbPool pool.Pool,
) *RedirectHandler {
	return &RedirectHandler{
		linkBiz:  linkBiz,
		rawCache: rawCache,
		dbPool:   dbPool,
		clickCh:  make(chan *models.Click, clickBufferSize),
	}
}

// Start launches background batch click recording workers.
func (rh *RedirectHandler) Start(ctx context.Context) {
	for range clickWorkerCount {
		rh.wg.Add(1)
		go rh.batchWorker(ctx)
	}
}

// Stop closes the click channel and waits for all workers to drain.
func (rh *RedirectHandler) Stop() {
	close(rh.clickCh)
	rh.wg.Wait()
}

// batchWorker accumulates clicks and flushes in batches to the database.
// Each worker maintains its own local counter map — no cross-worker coordination.
// At 4k req/s with 4 workers, each processes ~1k/s = ~10 batch flushes/s = ~10 DB ops/s per worker.
func (rh *RedirectHandler) batchWorker(ctx context.Context) {
	defer rh.wg.Done()
	log := util.Log(ctx)

	clickRepo := repository.NewClickRepository(rh.dbPool)
	linkRepo := repository.NewLinkRepository(rh.dbPool)

	batch := make([]*models.Click, 0, batchSize)
	counts := make(map[string]*clickCounts)

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) > 0 {
			if err := clickRepo.BulkCreate(ctx, batch); err != nil {
				log.Error("failed to flush click batch", "error", err, "count", len(batch))
			}
			batch = batch[:0]
		}

		for linkID, c := range counts {
			if err := linkRepo.IncrementClickCount(ctx, linkID, c.total, c.unique); err != nil {
				log.Warn("failed to flush click counts", "error", err, "link_id", linkID)
			}
			delete(counts, linkID)
		}
	}

	for {
		select {
		case click, ok := <-rh.clickCh:
			if !ok {
				flush()
				return
			}

			click.GenID(ctx)
			business.ParseUserAgent(click)

			// Track IP uniqueness in Frame cache — O(1), no DB query.
			isUnique := false
			if click.IPAddress != "" {
				uniqueKey := fmt.Sprintf("uniq:%s:%s", click.LinkID, click.IPAddress)
				exists, _ := rh.rawCache.Exists(ctx, uniqueKey)
				if !exists {
					_ = rh.rawCache.Set(ctx, uniqueKey, []byte("1"), uniqueIPTTL)
					isUnique = true
				}
			}

			// Accumulate counters locally — flushed to DB with each batch.
			c, ok := counts[click.LinkID]
			if !ok {
				c = &clickCounts{}
				counts[click.LinkID] = c
			}
			c.total++
			if isUnique {
				c.unique++
			}

			batch = append(batch, click)
			if len(batch) >= batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// ServeHTTP handles GET /r/{slug} — the hot redirect path.
func (rh *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := extractSlug(r.URL.Path)
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	link, err := rh.resolveLink(r.Context(), slug)
	if err != nil || link == nil {
		http.NotFound(w, r)
		return
	}

	if !link.IsActive() {
		http.Error(w, "Link is no longer active", http.StatusGone)
		return
	}

	click := &models.Click{
		LinkID:         link.GetID(),
		AffiliateID:    link.AffiliateID,
		Slug:           slug,
		IPAddress:      extractIP(r),
		UserAgent:      r.UserAgent(),
		Referer:        r.Referer(),
		AcceptLanguage: r.Header.Get("Accept-Language"),
	}

	select {
	case rh.clickCh <- click:
	default:
		util.Log(r.Context()).Warn("click buffer full, dropping click", "slug", slug)
	}

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Location", link.DestinationURL)
	w.WriteHeader(http.StatusFound)
}

func (rh *RedirectHandler) resolveLink(ctx context.Context, slug string) (*models.Link, error) {
	cacheKey := "link:" + slug

	data, found, err := rh.rawCache.Get(ctx, cacheKey)
	if err == nil && found {
		var link models.Link
		if jsonErr := json.Unmarshal(data, &link); jsonErr == nil {
			return &link, nil
		}
	}

	link, err := rh.linkBiz.GetLinkBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if link.IsActive() {
		if encoded, jsonErr := json.Marshal(link); jsonErr == nil {
			_ = rh.rawCache.Set(ctx, cacheKey, encoded, linkCacheTTL)
		}
	}

	return link, nil
}

// InvalidateCache removes a slug from the cache.
func (rh *RedirectHandler) InvalidateCache(ctx context.Context, slug string) {
	_ = rh.rawCache.Delete(ctx, "link:"+slug)
}

// extractSlug parses the slug from /r/{slug} without allocating a slice.
func extractSlug(path string) string {
	if len(path) < 4 || path[0] != '/' || path[1] != 'r' || path[2] != '/' {
		return ""
	}
	slug := path[3:]
	if idx := strings.IndexByte(slug, '/'); idx >= 0 {
		slug = slug[:idx]
	}
	if slug == "" {
		return ""
	}
	return slug
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	addr := r.RemoteAddr
	if idx := strings.LastIndexByte(addr, ':'); idx >= 0 {
		return addr[:idx]
	}
	return addr
}
