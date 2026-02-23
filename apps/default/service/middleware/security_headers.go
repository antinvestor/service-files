package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeadersConfig defines the configuration for security headers
type SecurityHeadersConfig struct {
	// ContentSecurityPolicy defines the CSP header value
	// Default: "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' https:; frame-ancestors 'none';"
	ContentSecurityPolicy string

	// XFrameOptions defines the X-Frame-Options header value
	// Default: "DENY"
	XFrameOptions string

	// XContentTypeOptions defines the X-Content-Type-Options header value
	// Default: "nosniff"
	XContentTypeOptions string

	// XSSProtection defines the X-XSS-Protection header value
	// Default: "1; mode=block"
	XSSProtection string

	// ReferrerPolicy defines the Referrer-Policy header value
	// Default: "strict-origin-when-cross-origin"
	ReferrerPolicy string

	// PermissionsPolicy defines the Permissions-Policy header value
	// Default: "camera=(), microphone=(), geolocation=(), payment=(), usb=()"
	PermissionsPolicy string

	// StrictTransportSecurity defines the Strict-Transport-Security header value
	// Default: "max-age=31536000; includeSubDomains; preload"
	StrictTransportSecurity string

	// CrossOriginEmbedderPolicy defines the Cross-Origin-Embedder-Policy header value
	// Default: "require-corp"
	CrossOriginEmbedderPolicy string

	// CrossOriginOpenerPolicy defines the Cross-Origin-Opener-Policy header value
	// Default: "same-origin"
	CrossOriginOpenerPolicy string

	// CrossOriginResourcePolicy defines the Cross-Origin-Resource-Policy header value
	// Default: "same-origin"
	CrossOriginResourcePolicy string
}

// DefaultSecurityHeadersConfig returns sensible defaults for security headers
func DefaultSecurityHeadersConfig() *SecurityHeadersConfig {
	return &SecurityHeadersConfig{
		ContentSecurityPolicy:     "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' https:; frame-ancestors 'none';",
		XFrameOptions:             "DENY",
		XContentTypeOptions:       "nosniff",
		XSSProtection:             "1; mode=block",
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		PermissionsPolicy:         "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()",
		StrictTransportSecurity:   "max-age=31536000; includeSubDomains; preload",
		CrossOriginEmbedderPolicy: "require-corp",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
	}
}

// SecurityHeadersMiddleware returns a middleware that adds security headers to all responses
func SecurityHeadersMiddleware(config *SecurityHeadersConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = DefaultSecurityHeadersConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Apply security headers to all responses
			if config.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
			}

			if config.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", config.XFrameOptions)
			}

			if config.XContentTypeOptions != "" {
				w.Header().Set("X-Content-Type-Options", config.XContentTypeOptions)
			}

			if config.XSSProtection != "" {
				w.Header().Set("X-XSS-Protection", config.XSSProtection)
			}

			if config.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", config.ReferrerPolicy)
			}

			if config.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", config.PermissionsPolicy)
			}

			if config.StrictTransportSecurity != "" && isHTTPS(r) {
				w.Header().Set("Strict-Transport-Security", config.StrictTransportSecurity)
			}

			if config.CrossOriginEmbedderPolicy != "" {
				w.Header().Set("Cross-Origin-Embedder-Policy", config.CrossOriginEmbedderPolicy)
			}

			if config.CrossOriginOpenerPolicy != "" {
				w.Header().Set("Cross-Origin-Opener-Policy", config.CrossOriginOpenerPolicy)
			}

			if config.CrossOriginResourcePolicy != "" {
				w.Header().Set("Cross-Origin-Resource-Policy", config.CrossOriginResourcePolicy)
			}

			// Remove server header if present
			w.Header().Set("Server", "")

			next.ServeHTTP(w, r)
		})
	}
}

// isHTTPS checks if the request is using HTTPS
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}

// CORSMiddleware returns a middleware that handles CORS
type CORSConfig struct {
	// AllowedOrigins is the list of allowed origins
	AllowedOrigins []string

	// AllowedMethods is the list of allowed HTTP methods
	AllowedMethods []string

	// AllowedHeaders is the list of allowed headers
	AllowedHeaders []string

	// ExposedHeaders is the list of headers exposed to the browser
	ExposedHeaders []string

	// AllowCredentials indicates whether credentials are allowed
	AllowCredentials bool

	// MaxAge is the maximum age for preflight requests
	MaxAge int
}

// DefaultCORSConfig returns sensible defaults for CORS
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With"},
		ExposedHeaders:   []string{},
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// CORSMiddleware returns a middleware that handles CORS
func CORSMiddleware(config *CORSConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = DefaultCORSConfig()
	}

	// Normalise origins to lowercase
	allowedOrigins := make(map[string]bool)
	for _, origin := range config.AllowedOrigins {
		allowedOrigins[strings.ToLower(origin)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				if origin != "" && (allowedOrigins["*"] || allowedOrigins[strings.ToLower(origin)]) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else if allowedOrigins["*"] {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}

				if len(config.AllowedMethods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
				}

				if len(config.AllowedHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
				}

				if len(config.ExposedHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
				}

				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				if config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", string(rune(config.MaxAge)))
				}

				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Add CORS headers to actual requests
			if origin != "" && (allowedOrigins["*"] || allowedOrigins[strings.ToLower(origin)]) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if allowedOrigins["*"] {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			if len(config.ExposedHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
			}

			if config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware returns a middleware that adds a unique request ID to each request
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}

			w.Header().Set("X-Request-ID", requestID)
			next.ServeHTTP(w, r)
		})
	}
}

// generateRequestID generates a simple unique request ID
func generateRequestID() string {
	// Simple implementation - in production use a proper UUID generator
	return "req_" + randomString(16)
}

// randomString generates a random string of the given length
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		// This is a simplified version - use crypto/rand for production
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}
