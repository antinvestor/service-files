package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/cache"
	"github.com/antinvestor/service-files/apps/default/service/handler"
	"github.com/antinvestor/service-files/apps/default/service/metrics"
	"github.com/antinvestor/service-files/apps/default/service/middleware"
	"github.com/antinvestor/service-files/apps/default/service/queue"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/datastore"
)

type IntegrationTestSuite struct {
	suite.Suite
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.InitResourceFunc = func(_ context.Context) []tests.TestResource {
		return tests.CreateTestResources()
	}
	s.BaseTestSuite.SetupSuite()
}

// Test_RateLimitingWithMiddleware tests the rate limiting middleware
func (s *IntegrationTestSuite) Test_RateLimitingWithMiddleware() {
	suite.WithTestDependencies(suite.T(), func(t *testing.T, dep *tests.TestResource) {
		ctx, svc, res := suite.CreateService(t, dep)

		// Create rate limiters
		config := middleware.DefaultRateLimiterConfig()
		config.RequestsPerSecond = 50 // Lower for tests
		config.BurstSize = 100

		ipLimiter := middleware.NewIPRateLimiter(config)
		userLimiter := middleware.NewUserRateLimiter(config)

		t.Run("allows_requests_within_limit", func(t *testing.T) {
			// Should allow 50 requests before limiting
			for i := 0; i < 50; i++ {
				allowed := ipLimiter.Allow("192.168.1.1")
				assert.True(t, allowed)
			}

			// 51st should be rate limited
			allowed := ipLimiter.Allow("192.168.1.1")
			assert.False(t, allowed)
		})

		t.Run("blocks_requests_exceeding_limit", func(t *testing.T) {
			// Should block after burst
			for i := 0; i < 150; i++ {
				_ = ipLimiter.Allow("192.168.1.1")
			}
		})

		t.Run("user_rate_limiting", func(t *testing.T) {
			// User-based limiting should work
			ctxWithUser := context.WithValue(ctx, "user_id", "test-user")

			for i := 0; i < 50; i++ {
				allowed := userLimiter.Allow("test-user")
				assert.True(t, allowed, "user request %d should be allowed", i)
			}

			// Different user should not be limited
			allowed := userLimiter.Allow("other-user")
			assert.True(t, allowed, "other user requests should be allowed")
		})
}

// Test_SecurityHeaders tests the security headers middleware
func (s *IntegrationTestSuite) Test_SecurityHeaders() {
	suite.WithTestDependencies(suite.T(), func(t *testing.T, dep *tests.TestResource) {
		handler := middleware.SecurityHeadersMiddleware(nil)

		req := httptest.NewRequest("GET", "/test", nil)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Check security headers are set
		assert.Equal(t, "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' https:; frame-ancestors 'none'", w.Header().Get("Content-Security-Policy"))
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
		assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
		assert.Equal(t, "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()", w.Header().Get("Permissions-Policy"))
		assert.Equal(t, "", w.Header().Get("Server"))
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	})

// Test_Caching tests the caching layer
func (s *IntegrationTestSuite) Test_Caching() {
	suite.WithTestDependencies(suite.T(), func(t *testing.T, dep *tests.TestResource) {
		cache := cache.NewCache(nil)

		// Test basic set and get
		cache.Set("key1", "value1")
		cache.Set("key2", "value2")

		val, found := cache.Get("key1")
		assert.True(t, found, "should find value in cache")
		assert.Equal(t, "value1", val, "should return correct value")

		// Test TTL expiration
		cache.SetWithTTL("temp", "temp-value", 100) // 100ms TTL
		val, found := cache.Get("temp")
		assert.True(t, found, "temporary value should be found")

		time.Sleep(110 * time.Millisecond) // Wait for expiration
		val, found = cache.Get("temp")
		assert.False(t, found, "temporary value should be expired after TTL")

		// Test cache size limit eviction
		config := cache.DefaultCacheConfig()
		config.MaxSize = 2

		cache2 := cache.NewCache(config)
		for i := 0; i < 5; i++ {
			cache2.Set(string(rune('a'+i)), i)
		}

		assert.Equal(t, 2, cache2.Size(), "should evict to max size")
		assert.Equal(t, 2, cache.Size(), "should evict first entry")
	})

// Test_Metrics tests the metrics collection
func (s *IntegrationTestSuite) Test_Metrics() {
	suite.WithTestDependencies(suite.T(), func(t *testing.T, dep *tests.TestResource) {
		metrics := metrics.NewMetrics()

		// Test request recording
		metrics.RecordRequest("GET", "/api/test", 100*time.Millisecond, 200)
		metrics.RecordRequest("POST", "/api/upload", 50*time.Millisecond, 200)

		reqMetrics := metrics.GetRequestMetrics()
		assert.Equal(t, int64(2), reqMetrics["GET:/api/test"]+reqMetrics["POST:/api/upload"])

		// Test upload/download metrics
		metrics.RecordUpload(1024, true)
		metrics.RecordUpload(2048, true)

		total, bytes, errors := metrics.GetUploadMetrics()
		assert.Equal(t, int64(2), total, "should count 2 uploads")
		assert.Equal(t, int64(3072), bytes, "should sum upload bytes")
		assert.Equal(t, int64(0), errors, "should have no errors")

		// Test cache metrics
		metrics.RecordCacheHit("thumbnail")
		metrics.RecordCacheHit("metadata")

		hits, misses := metrics.GetCacheMetrics()
		assert.Len(t, hits, 2, "should have 2 cache hit types")
		assert.Equal(t, int64(1), hits["thumbnail"], "should have 1 thumbnail hit")
		assert.Equal(t, int64(1), hits["metadata"], "should have 1 metadata hit")

		metrics.RecordCacheMiss("url_preview")

		hits2, misses2 := metrics.GetCacheMetrics()
		assert.Equal(t, int64(2), hits2, "should have same hit count")
		assert.Equal(t, int64(1), misses2["url_preview"], "should have 1 url preview miss",)
	})

// Test_EndToEnd tests the complete request flow
func (s *IntegrationTestSuite) Test_EndToEnd() {
	suite.WithTestDependencies(suite.T(), func(t *testing.T, dep *tests.TestResource) {
		ctx, svc, res := suite.CreateService(t, dep)

		// Test file upload -> download flow
		handler := handler.NewFileServer(svc, res.MediaService, res.AuthzMiddleware, res.MediaDB, res.StorageProvider)

		// Upload a test file
		uploadData := []byte("test file content")
		req := connect.NewRequest(&filesv1.CreateContentRequest{
			Metadata: &filesv1.MediaMetadata{
				ContentType:   "text/plain",
				Filename:      "test.txt",
			},
			Stream: func() io.Reader {
				return &bytesReader{data: uploadData}
			},
		})

		uploadResp, err := handler.UploadContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, uploadResp.Msg.MediaId)

		// Download the file
		downloadReq := connect.NewRequest(&filesv1.GetContentRequest{
			MediaId: uploadResp.Msg.MediaId,
		})

		downloadResp, err := handler.GetContent(ctx, downloadReq)
		require.NoError(t, err)
		require.NotNil(t, downloadResp.Msg.Content)

		// Verify the content
		assert.Equal(t, uploadData, downloadResp.Msg.Content)
	})

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	end := r.pos + n
	if end > len(r.data) {
		end = len(r.data)
	}
	n := end - r.pos
	r.pos = end
	return p[:n], nil
}
