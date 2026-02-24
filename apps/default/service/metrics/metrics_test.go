package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	require.NotNil(t, m)

	// Verify metrics are properly initialized
	assert.Equal(t, int64(0), m.GetActiveRequests())
}

func TestRecordRequest(t *testing.T) {
	m := NewMetrics()

	m.RecordRequest("GET", "/api/test", 100*time.Millisecond, 200)
	m.RecordRequest("GET", "/api/test", 150*time.Millisecond, 200)

	reqMetrics := m.GetRequestMetrics()
	assert.Equal(t, int64(2), reqMetrics["GET:/api/test"])

	// Test duration tracking
	avg := m.GetAverageDuration("GET", "/api/test")
	assert.Equal(t, 125*time.Millisecond, avg)
}

func TestRecordActiveRequest(t *testing.T) {
	m := NewMetrics()

	assert.Equal(t, int64(0), m.GetActiveRequests())

	m.RecordActiveRequest(1)
	assert.Equal(t, int64(1), m.GetActiveRequests())

	m.RecordActiveRequest(1)
	assert.Equal(t, int64(2), m.GetActiveRequests())

	m.RecordActiveRequest(-1)
	assert.Equal(t, int64(1), m.GetActiveRequests())
}

func TestRecordUpload(t *testing.T) {
	m := NewMetrics()

	m.RecordUpload(1024, true)
	m.RecordUpload(2048, true)
	m.RecordUpload(512, false)

	total, bytes, errors := m.GetUploadMetrics()
	assert.Equal(t, int64(3), total)
	assert.Equal(t, int64(3072), bytes) // Only successful uploads
	assert.Equal(t, int64(1), errors)
}

func TestRecordDownload(t *testing.T) {
	m := NewMetrics()

	m.RecordDownload(1024, true)
	m.RecordDownload(2048, false)

	total, bytes, errors := m.GetDownloadMetrics()
	assert.Equal(t, int64(2), total)
	assert.Equal(t, int64(1024), bytes) // Only successful downloads
	assert.Equal(t, int64(1), errors)
}

func TestRecordStorageStats(t *testing.T) {
	m := NewMetrics()

	m.RecordStorageStats(1000000, 100, 10, 500000, 500000)

	totalBytes, totalFiles, totalUsers, publicBytes, privateBytes := m.GetStorageMetrics()
	assert.Equal(t, int64(1000000), totalBytes)
	assert.Equal(t, int64(100), totalFiles)
	assert.Equal(t, int64(10), totalUsers)
	assert.Equal(t, int64(500000), publicBytes)
	assert.Equal(t, int64(500000), privateBytes)
}

func TestRecordCacheHit(t *testing.T) {
	m := NewMetrics()

	m.RecordCacheHit("thumbnail")
	m.RecordCacheHit("thumbnail")
	m.RecordCacheHit("metadata")

	hits, _ := m.GetCacheMetrics()
	assert.Equal(t, int64(2), hits["thumbnail"])
	assert.Equal(t, int64(1), hits["metadata"])
}

func TestRecordCacheMiss(t *testing.T) {
	m := NewMetrics()

	m.RecordCacheMiss("thumbnail")
	m.RecordCacheMiss("metadata")

	_, misses := m.GetCacheMetrics()
	assert.Equal(t, int64(1), misses["thumbnail"])
	assert.Equal(t, int64(1), misses["metadata"])
}

func TestGetAverageDuration(t *testing.T) {
	m := NewMetrics()

	// Test with no requests
	avg := m.GetAverageDuration("GET", "/test")
	assert.Equal(t, time.Duration(0), avg)

	// Test with requests
	m.RecordRequest("GET", "/test", 100*time.Millisecond, 200)
	m.RecordRequest("GET", "/test", 200*time.Millisecond, 200)
	m.RecordRequest("GET", "/test", 300*time.Millisecond, 200)

	avg = m.GetAverageDuration("GET", "/test")
	assert.Equal(t, 200*time.Millisecond, avg)
}

func TestGetPercentileDuration(t *testing.T) {
	m := NewMetrics()

	// Test with no requests
	pct := m.GetPercentileDuration("GET", "/test", 50)
	assert.Equal(t, time.Duration(0), pct)

	// Test with requests
	// The implementation uses zero-based indexing, so for 10 items:
	// 50th percentile = index 5 (6th item) = 600ms
	// 90th percentile = index 9 (10th item) = 1000ms
	for i := 0; i < 10; i++ {
		m.RecordRequest("GET", "/test", time.Duration(i+1)*100*time.Millisecond, 200)
	}

	pct = m.GetPercentileDuration("GET", "/test", 50)
	assert.Equal(t, 600*time.Millisecond, pct)

	pct = m.GetPercentileDuration("GET", "/test", 90)
	assert.Equal(t, 1000*time.Millisecond, pct)
}

func TestRequestDurationLimit(t *testing.T) {
	m := NewMetrics()

	// Add many requests to test the 1000 limit
	for i := 0; i < 1500; i++ {
		m.RecordRequest("GET", "/test", time.Duration(i)*time.Millisecond, 200)
	}

	reqMetrics := m.GetRequestMetrics()
	assert.Equal(t, int64(1500), reqMetrics["GET:/test"])
}

func TestHandler(t *testing.T) {
	m := NewMetrics()

	// Record some metrics
	m.RecordRequest("GET", "/test", 100*time.Millisecond, 200)
	m.RecordUpload(1024, true)
	m.RecordCacheHit("thumbnail")

	handler := m.Handler()
	require.NotNil(t, handler)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, "text/plain; version=0.0.4", w.Header().Get("Content-Type"))
	body := w.Body.String()
	assert.Contains(t, body, "file_service_active_requests")
	assert.Contains(t, body, "file_service_uploads_total")
	assert.Contains(t, body, "file_service_cache_hits_total")
	assert.Contains(t, body, "file_service_up")
}

func TestMiddleware(t *testing.T) {
	m := NewMetrics()

	mw := m.Middleware()
	require.NotNil(t, mw)

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mw(testHandler).ServeHTTP(w, req)

	assert.True(t, handlerCalled, "handler should be called")

	// Check that request was recorded
	reqMetrics := m.GetRequestMetrics()
	assert.Equal(t, int64(1), reqMetrics["GET:/test"])

	// Check active requests was decremented
	assert.Equal(t, int64(0), m.GetActiveRequests())
}

func TestMiddlewareWithStatusCode(t *testing.T) {
	m := NewMetrics()

	mw := m.Middleware()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	req := httptest.NewRequest("GET", "/notfound", nil)
	w := httptest.NewRecorder()

	mw(testHandler).ServeHTTP(w, req)

	// Check that request with 404 was recorded
	reqMetrics := m.GetRequestMetrics()
	assert.Equal(t, int64(1), reqMetrics["GET:/notfound"])
}

func TestHealthCheckHandler(t *testing.T) {
	m := NewMetrics()

	handler := m.HealthCheckHandler()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	body := w.Body.String()
	assert.Contains(t, body, `"status":"healthy"`)
	assert.Contains(t, body, `"service":"file-service"`)
}

func TestReadyCheckHandler(t *testing.T) {
	m := NewMetrics()

	// Test with ready function
	handler := m.ReadyCheckHandler(func() bool { return true })

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ready"`)

	// Test with not ready function
	handler = m.ReadyCheckHandler(func() bool { return false })

	req = httptest.NewRequest("GET", "/ready", nil)
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"not_ready"`)
}

func TestReadyCheckHandlerWithNilFunction(t *testing.T) {
	m := NewMetrics()

	handler := m.ReadyCheckHandler(nil)

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ready"`)
}

func TestMetricsFormat(t *testing.T) {
	m := NewMetrics()

	m.RecordRequest("GET", "/api/test", 100*time.Millisecond, 200)
	m.RecordUpload(1024, true)
	m.RecordCacheHit("thumbnail")

	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Check HELP comments
	assert.Contains(t, body, "# HELP file_service_active_requests")
	assert.Contains(t, body, "# HELP file_service_uploads_total")
	assert.Contains(t, body, "# HELP file_service_downloads_total")
	assert.Contains(t, body, "# HELP file_service_storage_bytes_total")
	assert.Contains(t, body, "# HELP file_service_cache_hits_total")
	assert.Contains(t, body, "# HELP file_service_requests_total")
	assert.Contains(t, body, "# HELP file_service_up")

	// Check TYPE comments
	assert.Contains(t, body, "# TYPE file_service_active_requests gauge")
	assert.Contains(t, body, "# TYPE file_service_uploads_total counter")
	assert.Contains(t, body, "# TYPE file_service_downloads_total counter")
	assert.Contains(t, body, "# TYPE file_service_storage_bytes_total gauge")
	assert.Contains(t, body, "# TYPE file_service_cache_hits_total counter")
	assert.Contains(t, body, "# TYPE file_service_requests_total counter")
	assert.Contains(t, body, "# TYPE file_service_up gauge")
}

func TestResponseWriterWrapper(t *testing.T) {
	w := httptest.NewRecorder()
	wrapped := &responseWriterWrapper{
		ResponseWriter: w,
		statusCode:     200,
	}

	// Test WriteHeader updates statusCode
	wrapped.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, wrapped.statusCode)

	// Test default status code is 0
	wrapped2 := &responseWriterWrapper{ResponseWriter: w}
	assert.Equal(t, 0, wrapped2.statusCode)
}

func TestWriteIgnoreErr(t *testing.T) {
	var buf strings.Builder
	writeIgnoreErr(&buf, "test %s\n", "value")
	assert.Equal(t, "test value\n", buf.String())

	// Test with a writer that always fails
	failWriter := &failWriter{}
	writeIgnoreErr(failWriter, "this should not panic")
}

type failWriter struct{}

func (w *failWriter) Write(p []byte) (n int, err error) {
	return 0, io.ErrClosedPipe
}
