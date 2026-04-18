package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/antinvestor/service-files/apps/redirect/service/analytics"
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

	// Analytics + liveness wiring. analytics may be nil in local dev;
	// the jobsBaseURL drives the "Find other jobs" CTA on the dead-
	// link page. Liveness probing shares a single gate per handler
	// instance so concurrent clicks coalesce into one inflight probe.
	analytics    *analytics.Client
	jobsBaseURL  string
	livenessGate *livenessGate

	// expiredWebhooks are the URLs to POST when a link flips to
	// EXPIRED. Each one receives {link_id, slug, affiliate_id,
	// destination_url, expired_at}. Subscribers filter on
	// affiliate_id to decide relevance; calls are best-effort.
	expiredWebhooks []string

	clickCh chan *models.Click
	wg      sync.WaitGroup
}

func NewRedirectHandler(
	linkBiz business.LinkBusiness,
	rawCache cache.RawCache,
	dbPool pool.Pool,
	analyticsClient *analytics.Client,
	jobsBaseURL string,
	expiredWebhooks []string,
) *RedirectHandler {
	return &RedirectHandler{
		linkBiz:         linkBiz,
		rawCache:        rawCache,
		dbPool:          dbPool,
		analytics:       analyticsClient,
		jobsBaseURL:     jobsBaseURL,
		livenessGate:    newLivenessGate(),
		expiredWebhooks: expiredWebhooks,
		clickCh:         make(chan *models.Click, clickBufferSize),
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

// ServeHTTP handles GET /r/{slug} — the hot redirect path. In order:
// (1) resolve the link by slug; (2) if dead, render a branded page;
// (3) record the click for attribution; (4) opportunistically fire a
// throttled destination-URL probe in the background; (5) 302 to the
// destination. X-Robots-Tag: noindex so search engines don't index
// tracking hops.
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

	// Dead link: render the nice "this posting has been removed" page
	// instead of a raw 410. The redirect service has no job-domain
	// knowledge, but we can still show the link's title + a CTA back
	// to jobs.stawi.org to keep the user moving.
	if !link.IsActive() {
		renderDeadLinkPage(w, deadLinkData{
			Title:       link.Title,
			JobsBaseURL: rh.jobsBaseURL,
		})
		rh.emitClickAnalytics(r.Context(), link, slug, r, "link_inactive")
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
	rh.emitClickAnalytics(r.Context(), link, slug, r, "redirect")

	// Fire-and-forget destination liveness probe. The gate bounds fan-
	// out to one inflight probe per link + throttles to once per
	// livenessProbeThrottle window. Detached context so the user's
	// redirect latency isn't coupled to a third-party HEAD.
	rh.maybeProbeAsync(link)

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Location", link.DestinationURL)
	w.WriteHeader(http.StatusFound)
}

// maybeProbeAsync runs a throttled liveness probe in the background.
// On repeated failure the link is flipped to EXPIRED so subsequent
// clicks see the dead-link page. Click accounting is unaffected.
func (rh *RedirectHandler) maybeProbeAsync(link *models.Link) {
	ok, release := rh.livenessGate.acquire(link.GetID())
	if !ok {
		return
	}
	go func() {
		defer release()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		status, perr := probeDestination(ctx, link.DestinationURL)
		reachable := perr == nil && isReachableStatus(status)

		var nextFailures int
		if reachable {
			nextFailures = 0
		} else {
			nextFailures = link.ConsecutiveCheckFailures + 1
		}

		linkRepo := repository.NewLinkRepository(rh.dbPool)
		if err := linkRepo.RecordCheck(ctx, link.GetID(), status, nextFailures); err != nil {
			util.Log(ctx).Warn("record link check failed", "error", err, "link_id", link.GetID())
		}

		terminal := status == 404 || status == 410
		exhausted := nextFailures >= consecutiveFailuresToExpire
		if !reachable && (terminal || exhausted) {
			if err := linkRepo.ExpireLink(ctx, link.GetID()); err != nil {
				util.Log(ctx).Warn("expire link failed", "error", err, "link_id", link.GetID())
			} else {
				// Cache invalidation so the dead-link page shows up on
				// the very next request, not after cache TTL.
				rh.InvalidateCache(ctx, link.Slug)

				// Fire a best-effort link.expired webhook to any
				// subscribers configured to react (e.g. stawi-jobs
				// flipping canonical_jobs.status + emailing saved-job
				// bookmarkers). Detached goroutine with its own short
				// timeout — the redirect state change is already
				// persisted, webhook failures don't revert it.
				go rh.postLinkExpired(link)
			}
			rh.emitLivenessEvent(ctx, link, status, perr, reachable, nextFailures, true)
			return
		}
		rh.emitLivenessEvent(ctx, link, status, perr, reachable, nextFailures, false)
	}()
}

// postLinkExpired notifies every configured subscriber that a link has
// been marked EXPIRED. Calls are POST with a small JSON body — the
// receiving service is expected to match on `affiliate_id` to decide
// whether the signal is theirs.
func (rh *RedirectHandler) postLinkExpired(link *models.Link) {
	if len(rh.expiredWebhooks) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]any{
		"link_id":         link.GetID(),
		"slug":            link.Slug,
		"affiliate_id":    link.AffiliateID,
		"destination_url": link.DestinationURL,
		"expired_at":      time.Now().UTC().Format(time.RFC3339),
	})

	for _, url := range rh.expiredWebhooks {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			util.Log(ctx).Warn("link.expired webhook: build request failed",
				"error", err, "url", url)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			util.Log(ctx).Warn("link.expired webhook: request failed",
				"error", err, "url", url, "link_id", link.GetID())
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 {
			util.Log(ctx).Warn("link.expired webhook: non-2xx",
				"status", resp.StatusCode, "url", url, "link_id", link.GetID())
		}
	}
}

func (rh *RedirectHandler) emitClickAnalytics(ctx context.Context, link *models.Link, slug string, r *http.Request, kind string) {
	if rh.analytics == nil {
		return
	}
	rh.analytics.Send(ctx, "stawi_jobs_applies", map[string]any{
		"event":        kind, // "redirect" or "link_inactive"
		"link_id":      link.GetID(),
		"affiliate_id": link.AffiliateID, // canonical_job_<id> for stawi-jobs
		"slug":         slug,
		"campaign":     link.Campaign,
		"source":       link.Source,
		"medium":       link.Medium,
		"ip_address":   extractIP(r),
		"user_agent":   r.UserAgent(),
		"referer":      r.Referer(),
		"cf_country":   r.Header.Get("CF-IPCountry"),
		"cf_ray":       r.Header.Get("CF-Ray"),
	})
}

func (rh *RedirectHandler) emitLivenessEvent(ctx context.Context, link *models.Link, status int, perr error, reachable bool, consecutiveFailures int, expired bool) {
	if rh.analytics == nil {
		return
	}
	errStr := ""
	if perr != nil {
		errStr = perr.Error()
	}
	rh.analytics.Send(ctx, "stawi_jobs_events", map[string]any{
		"event":                "redirect_probe",
		"link_id":              link.GetID(),
		"affiliate_id":         link.AffiliateID,
		"slug":                 link.Slug,
		"destination":          link.DestinationURL,
		"probe_status":         status,
		"probe_error":          errStr,
		"reachable":            reachable,
		"consecutive_failures": consecutiveFailures,
		"link_expired":         expired,
	})
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
