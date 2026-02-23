package middleware

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pitabwire/util"
	"golang.org/x/time/rate"
)

// writeIgnoreErr writes to the response writer, ignoring errors
func writeIgnoreErr(w io.Writer, data string) {
	_, _ = fmt.Fprint(w, data)
}

// RateLimiterConfig defines the configuration for rate limiting
type RateLimiterConfig struct {
	// RequestsPerSecond is the maximum number of requests allowed per second
	RequestsPerSecond int
	// BurstSize is the maximum number of requests allowed in a burst
	BurstSize int
	// CleanupInterval is how often to cleanup old entries from the limiter map
	CleanupInterval time.Duration
	// EntryTTL is how long a limiter entry should be kept without activity
	EntryTTL time.Duration
}

// DefaultRateLimiterConfig returns sensible defaults for rate limiting
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		RequestsPerSecond: 100,
		BurstSize:         200,
		CleanupInterval:   5 * time.Minute,
		EntryTTL:          10 * time.Minute,
	}
}

// IPRateLimiter implements rate limiting per IP address
type IPRateLimiter struct {
	limiterMap map[string]*ipLimiterEntry
	mu         sync.RWMutex
	config     *RateLimiterConfig
}

type ipLimiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// NewIPRateLimiter creates a new IP-based rate limiter
func NewIPRateLimiter(config *RateLimiterConfig) *IPRateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}

	rl := &IPRateLimiter{
		limiterMap: make(map[string]*ipLimiterEntry),
		config:     config,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given IP is allowed
func (rl *IPRateLimiter) Allow(ip string) bool {
	rl.mu.RLock()
	entry, exists := rl.limiterMap[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		if entry, exists = rl.limiterMap[ip]; !exists {
			limiter := rate.NewLimiter(rate.Limit(float64(rl.config.RequestsPerSecond)), rl.config.BurstSize)
			entry = &ipLimiterEntry{
				limiter:    limiter,
				lastAccess: time.Now(),
			}
			rl.limiterMap[ip] = entry
		}
		rl.mu.Unlock()
	}

	// Update last access time
	entry.lastAccess = time.Now()

	return entry.limiter.Allow()
}

// cleanup removes old entries from the limiter map
func (rl *IPRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, entry := range rl.limiterMap {
			if now.Sub(entry.lastAccess) > rl.config.EntryTTL {
				delete(rl.limiterMap, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// GetIP extracts the IP address from an HTTP request
func GetIP(r *http.Request) string {
	// Check X-Forwarded-For header for proxies
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP from the comma-separated list
		for i, c := range xff {
			if c == ' ' {
				continue
			}
			if c == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	if r.RemoteAddr != "" {
		// Remove port if present
		for i := len(r.RemoteAddr) - 1; i >= 0; i-- {
			if r.RemoteAddr[i] == ':' {
				return r.RemoteAddr[:i]
			}
		}
		return r.RemoteAddr
	}

	return "unknown"
}

// RateLimitMiddleware returns a middleware that applies rate limiting
func RateLimitMiddleware(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := util.Log(ctx)

			ip := GetIP(r)
			allowed := limiter.Allow(ip)

			if !allowed {
				log.WithField("ip", ip).Warn("rate limit exceeded")
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limiter.config.RequestsPerSecond))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				writeIgnoreErr(w, `{"error": "rate limit exceeded", "code": "rate_limit_exceeded"}`)
				return
			}

			// Add rate limit headers for successful requests
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limiter.config.RequestsPerSecond))
			next.ServeHTTP(w, r)
		})
	}
}

// UserRateLimiter implements rate limiting per user ID (for authenticated users)
type UserRateLimiter struct {
	limiterMap map[string]*userLimiterEntry
	mu         sync.RWMutex
	config     *RateLimiterConfig
}

type userLimiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// NewUserRateLimiter creates a new user-based rate limiter
func NewUserRateLimiter(config *RateLimiterConfig) *UserRateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}

	rl := &UserRateLimiter{
		limiterMap: make(map[string]*userLimiterEntry),
		config:     config,
	}

	go rl.cleanup()
	return rl
}

// Allow checks if a request from the given user is allowed
func (rl *UserRateLimiter) Allow(userID string) bool {
	rl.mu.RLock()
	entry, exists := rl.limiterMap[userID]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		if entry, exists = rl.limiterMap[userID]; !exists {
			limiter := rate.NewLimiter(rate.Limit(float64(rl.config.RequestsPerSecond)), rl.config.BurstSize)
			entry = &userLimiterEntry{
				limiter:    limiter,
				lastAccess: time.Now(),
			}
			rl.limiterMap[userID] = entry
		}
		rl.mu.Unlock()
	}

	entry.lastAccess = time.Now()
	return entry.limiter.Allow()
}

// cleanup removes old entries from the user limiter map
func (rl *UserRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for userID, entry := range rl.limiterMap {
			if now.Sub(entry.lastAccess) > rl.config.EntryTTL {
				delete(rl.limiterMap, userID)
			}
		}
		rl.mu.Unlock()
	}
}

// GetUserID extracts the user ID from the request context
func GetUserID(ctx context.Context) string {
	// Check for user ID in context (set by authentication middleware)
	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		return userID
	}
	return ""
}

// UserRateLimitMiddleware returns a middleware that applies rate limiting per authenticated user
// Falls back to IP-based limiting for unauthenticated requests
func UserRateLimitMiddleware(userLimiter *UserRateLimiter, ipLimiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := util.Log(ctx)

			userID := GetUserID(ctx)
			var allowed bool
			var rateLimitType string

			if userID != "" {
				// Use user-based rate limiting for authenticated users
				allowed = userLimiter.Allow(userID)
				rateLimitType = "user"
				w.Header().Set("X-RateLimit-Scope", "user")
			} else {
				// Fall back to IP-based rate limiting
				ip := GetIP(r)
				allowed = ipLimiter.Allow(ip)
				rateLimitType = "ip"
				w.Header().Set("X-RateLimit-Scope", "ip")
			}

			if !allowed {
				log.WithField("rate_limit_type", rateLimitType).Warn("rate limit exceeded")
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", userLimiter.config.RequestsPerSecond))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				writeIgnoreErr(w, `{"error": "rate limit exceeded", "code": "rate_limit_exceeded"}`)
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", userLimiter.config.RequestsPerSecond))
			next.ServeHTTP(w, r)
		})
	}
}
