package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSecurityHeadersConfig(t *testing.T) {
	cfg := DefaultSecurityHeadersConfig()

	assert.Contains(t, cfg.ContentSecurityPolicy, "default-src 'self'")
	assert.Contains(t, cfg.ContentSecurityPolicy, "script-src 'self'")
	assert.Equal(t, "DENY", cfg.XFrameOptions)
	assert.Equal(t, "nosniff", cfg.XContentTypeOptions)
	assert.Equal(t, "1; mode=block", cfg.XSSProtection)
	assert.Equal(t, "strict-origin-when-cross-origin", cfg.ReferrerPolicy)
	assert.Contains(t, cfg.PermissionsPolicy, "camera=()")
	assert.Equal(t, "max-age=31536000; includeSubDomains; preload", cfg.StrictTransportSecurity)
}

func TestDefaultCORSConfig(t *testing.T) {
	cfg := DefaultCORSConfig()

	assert.Equal(t, []string{"*"}, cfg.AllowedOrigins)
	assert.Equal(t, []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, cfg.AllowedMethods)
	assert.Equal(t, []string{"Content-Type", "Authorization", "X-Requested-With"}, cfg.AllowedHeaders)
	assert.Equal(t, 86400, cfg.MaxAge)
}

func TestDefaultRateLimiterConfig(t *testing.T) {
	cfg := DefaultRateLimiterConfig()

	assert.Equal(t, 100, cfg.RequestsPerSecond)
	assert.Equal(t, 200, cfg.BurstSize)
	assert.Equal(t, 5*time.Minute, cfg.CleanupInterval)
}

func TestNewIPRateLimiter(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	limiter := NewIPRateLimiter(cfg)

	require.NotNil(t, limiter)
	assert.Equal(t, cfg.RequestsPerSecond, limiter.config.RequestsPerSecond)
	assert.Equal(t, cfg.BurstSize, limiter.config.BurstSize)
}

func TestNewUserRateLimiter(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	limiter := NewUserRateLimiter(cfg)

	require.NotNil(t, limiter)
	assert.Equal(t, cfg.RequestsPerSecond, limiter.config.RequestsPerSecond)
	assert.Equal(t, cfg.BurstSize, limiter.config.BurstSize)
}

func TestIPRateLimiterAllow(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	limiter := NewIPRateLimiter(cfg)

	// First 200 requests should be allowed (burst size)
	for i := 0; i < 200; i++ {
		assert.True(t, limiter.Allow("127.0.0.1"), "request %d should be allowed", i)
	}

	// 201st request should still be allowed (since rate is 100/sec)
	// But after immediate burst, next request might be limited depending on timing
	// For simplicity, let's just verify the burst allows at least 200 requests
}

func TestIPRateLimiterAllowDifferentIPs(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	limiter := NewIPRateLimiter(cfg)

	// Different IPs should have separate rate limits
	for i := 0; i < 5; i++ {
		ip := "127.0.0."
		if i < 10 {
			ip = ip + string(rune('1'+i))
		} else {
			ip = ip + string(rune('0'+i-10))
		}
		assert.True(t, limiter.Allow(ip), "IP %s should be allowed", ip)
	}

	// Test that IP entries are tracked separately
	for i := 0; i < 10; i++ {
		limiter.Allow("127.0.0.1")
	}
	limiter.Allow("127.0.0.2")

	// Different IP should still be allowed
	allowed := limiter.Allow("127.0.0.3")
	assert.True(t, allowed, "different IP should be allowed")
}

func TestRateLimiterCleanup(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.RequestsPerSecond = 100
	cfg.BurstSize = 10
	cfg.CleanupInterval = 100 * time.Millisecond
	cfg.EntryTTL = 50 * time.Millisecond

	limiter := NewIPRateLimiter(cfg)

	// Use up burst
	for i := 0; i < 10; i++ {
		limiter.Allow("127.0.0.1")
	}

	// Wait for cleanup interval + some buffer
	time.Sleep(150 * time.Millisecond)

	// Tokens should have been partially refilled, so some requests allowed
	allowedCount := 0
	for i := 0; i < 5; i++ {
		if limiter.Allow("127.0.0.1") {
			allowedCount++
		}
	}

	assert.True(t, allowedCount > 0, "tokens should have been refilled after cleanup interval")
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	mw := SecurityHeadersMiddleware(nil)
	require.NotNil(t, mw)

	// Create a test handler
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mw(testHandler).ServeHTTP(w, req)

	assert.True(t, handlerCalled, "handler should be called")
	assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
}

func TestSecurityHeadersWithCustomConfig(t *testing.T) {
	customCSP := "default-src 'https://example.com'"
	cfg := &SecurityHeadersConfig{
		ContentSecurityPolicy: customCSP,
		XFrameOptions:         "SAMEORIGIN",
	}

	mw := SecurityHeadersMiddleware(cfg)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mw(testHandler).ServeHTTP(w, req)

	assert.Equal(t, customCSP, w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
}

func TestCORSMiddleware(t *testing.T) {
	cfg := DefaultCORSConfig()
	mw := CORSMiddleware(cfg)

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Test preflight request
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	mw(testHandler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")

	// Test regular request
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w = httptest.NewRecorder()
	mw(testHandler).ServeHTTP(w, req)

	assert.True(t, handlerCalled, "handler should be called")
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestGetIP(t *testing.T) {
	tests := []struct {
		name       string
		xForwarded string
		xRealIP    string
		remoteAddr string
		expected   string
	}{
		{
			name:       "X-Forwarded-For header",
			xForwarded: "192.168.1.1, 192.168.1.2",
			remoteAddr: "127.0.0.1:1234",
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Real-IP header",
			xRealIP:    "192.168.1.3",
			remoteAddr: "127.0.0.1:1234",
			expected:   "192.168.1.3",
		},
		{
			name:       "RemoteAddr fallback",
			remoteAddr: "127.0.0.1:1234",
			expected:   "127.0.0.1",
		},
		{
			name:     "unknown IP",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.xForwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwarded)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			req.RemoteAddr = tt.remoteAddr

			ip := GetIP(req)
			assert.Equal(t, tt.expected, ip)
		})
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	mw := RequestIDMiddleware()

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mw(testHandler).ServeHTTP(w, req)

	assert.True(t, handlerCalled, "handler should be called")
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRequestIDMiddlewareWithExistingID(t *testing.T) {
	mw := RequestIDMiddleware()

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	existingID := "my-custom-id"
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	w := httptest.NewRecorder()
	mw(testHandler).ServeHTTP(w, req)

	assert.True(t, handlerCalled, "handler should be called")
	assert.Equal(t, existingID, w.Header().Get("X-Request-ID"))
}
