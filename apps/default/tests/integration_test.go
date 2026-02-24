package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/cache"
	"github.com/antinvestor/service-files/apps/default/service/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_RateLimiterAndMiddleware(t *testing.T) {
	cfg := middleware.DefaultRateLimiterConfig()
	cfg.RequestsPerSecond = 1
	cfg.BurstSize = 2
	cfg.CleanupInterval = 200 * time.Millisecond
	cfg.EntryTTL = 100 * time.Millisecond

	ipLimiter := middleware.NewIPRateLimiter(cfg)
	mw := middleware.RateLimitMiddleware(ipLimiter)

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestIntegration_SecurityHeadersAndCacheTTL(t *testing.T) {
	h := middleware.SecurityHeadersMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
	assert.NotEmpty(t, rr.Header().Get("Content-Security-Policy"))

	c := cache.NewCache(nil)
	c.SetWithTTL("k", "v", 25*time.Millisecond)
	value, found := c.GetString("k")
	require.True(t, found)
	assert.Equal(t, "v", value)

	time.Sleep(40 * time.Millisecond)
	_, found = c.GetString("k")
	assert.False(t, found)
}
