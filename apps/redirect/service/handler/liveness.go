package handler

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// livenessProbeThrottle bounds destination-URL probing to one HEAD per
// link per window. Without this, a viral redirect (thousands of clicks
// per minute) would fan every click out into a probe against the
// employer's site — a textbook DoS. 15 minutes catches broken links
// within the window of a single popular job listing while letting us
// stay well-behaved.
const livenessProbeThrottle = 15 * time.Minute

// consecutiveFailuresToExpire is how many back-to-back probe failures
// flip a link to EXPIRED. A single flaky 5xx shouldn't take the link
// down; three in a row is a strong "it's gone" signal.
const consecutiveFailuresToExpire = 3

// linkProbeState is an in-memory gate so multiple concurrent clicks on
// the same slug share a single inflight probe. The counters here are
// informational only; authoritative consecutive-failure state lives on
// the Link row so it survives restarts.
type linkProbeState struct {
	mu        sync.Mutex
	inflight  bool
	lastProbe time.Time
}

// livenessGate tracks per-link probe inflight + recency. Bounded
// implicitly by linkCacheTTL eviction from the link cache, but we also
// rely on normal Go map growth at the scale this service runs.
type livenessGate struct {
	mu     sync.Mutex
	states map[string]*linkProbeState
}

func newLivenessGate() *livenessGate {
	return &livenessGate{states: make(map[string]*linkProbeState)}
}

// acquire returns (probe-it-now, release-fn). When the caller should
// probe, the gate marks the slug inflight and records the attempt
// start time. When the caller should skip, the release-fn is a no-op.
func (g *livenessGate) acquire(linkID string) (bool, func()) {
	g.mu.Lock()
	s, ok := g.states[linkID]
	if !ok {
		s = &linkProbeState{}
		g.states[linkID] = s
	}
	g.mu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inflight {
		return false, func() {}
	}
	if time.Since(s.lastProbe) < livenessProbeThrottle {
		return false, func() {}
	}
	s.inflight = true
	s.lastProbe = time.Now()
	return true, func() {
		s.mu.Lock()
		s.inflight = false
		s.mu.Unlock()
	}
}

// probeDestination runs a short-timeout HEAD against url, falling back
// to a ranged GET for hosts that reject HEAD (common with Cloudflare-
// fronted sites). Returns the best-observed status and any transport
// error; a zero status with non-nil error means the request never
// reached a server.
//
// The client is Frame's managed HTTP client (OTel-instrumented,
// pooled connections); RedirectHandler injects it once at startup
// instead of each probe allocating its own. The 10s ctx timeout is
// the per-probe safety net so one slow destination can't hold a
// livenessGate slot indefinitely.
func probeDestination(ctx context.Context, client *http.Client, url string) (int, error) {
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequestWithContext(pctx, method, url, nil)
		if err != nil {
			return 0, fmt.Errorf("build %s: %w", method, err)
		}
		req.Header.Set("User-Agent", "stawi.jobs-liveness/1.0 (+https://stawi.jobs)")
		if method == http.MethodGet {
			req.Header.Set("Range", "bytes=0-0")
		}

		resp, err := client.Do(req)
		if err != nil {
			if method == http.MethodHead {
				continue
			}
			return 0, err
		}
		_ = resp.Body.Close()

		if method == http.MethodHead {
			switch {
			case resp.StatusCode >= 200 && resp.StatusCode < 400:
				return resp.StatusCode, nil
			case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
				return resp.StatusCode, nil
			default:
				continue
			}
		}
		return resp.StatusCode, nil
	}
	return 0, fmt.Errorf("probe %s: no successful method", url)
}

// isReachableStatus classifies an HTTP status as "destination is up".
// Mirrors the logic in stawi.jobs/pkg/pipeline/handlers/liveness.go so
// the two services interpret probe outcomes identically.
func isReachableStatus(status int) bool {
	if status >= 200 && status < 400 {
		return true
	}
	return status == 401 || status == 403 || status == 405
}
