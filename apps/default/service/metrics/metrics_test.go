package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetricsBasics(t *testing.T) {
	t.Run("new_metrics", func(t *testing.T) {
		metrics := NewMetrics()

		require.NotNil(t, metrics)

		// Test recording requests
		metrics.RecordRequest("GET", "/api/files", 100*1_000_000, 200)
		metrics.RecordRequest("POST", "/api/upload", 50*1_000_000, 200)

		reqMetrics := metrics.GetRequestMetrics()
		require.Len(t, reqMetrics, 3)

		// Test active requests
		active := metrics.GetActiveRequests()
		require.Equal(t, int64(2), active)

		metrics.RecordActiveRequest(-1)
		active = metrics.GetActiveRequests()
		require.Equal(t, int64(1), active)
	})

	t.Run("record_upload", func(t *testing.T) {
		metrics := NewMetrics()

		metrics.RecordUpload(1024, true)
		metrics.RecordUpload(2048, false)
		metrics.RecordUpload(512, true)

		total, bytes, errors := metrics.GetUploadMetrics()
		require.Equal(t, int64(3), total, "should count 3 uploads")
		require.Equal(t, int64(3584), bytes, "should count correct bytes")
		require.Equal(t, int64(1), errors, "should count 1 error")
	})

	t.Run("record_download", func(t *testing.T) {
		metrics := NewMetrics()

		metrics.RecordDownload(2048, true)
		metrics.RecordDownload(512, false)

		total, bytes, errors := metrics.GetDownloadMetrics()
		require.Equal(t, int64(2), total, "should count 2 downloads")
		require.Equal(t, int64(2560), bytes, "should count correct bytes")
		require.Equal(t, int64(0), errors, "should have no errors")
	})

	t.Run("cache_operations", func(t *testing.T) {
		metrics := NewMetrics()

		metrics.RecordCacheHit("thumbnail")
		metrics.RecordCacheHit("metadata")

		hits, misses := metrics.GetCacheMetrics()
		require.Len(t, hits, 2)
		require.Equal(t, int64(1), hits["thumbnail"])
		require.Equal(t, int64(1), hits["metadata"])

		metrics.RecordCacheMiss("url_preview")
		metrics.RecordCacheMiss("storage_stats")

		hits2, misses2 := metrics.GetCacheMetrics()
		require.Len(t, hits2, 2)
		require.Equal(t, int64(1), hits2["url_preview"])
		require.Equal(t, int64(1), misses2["storage_stats"])
	})

	t.Run("health_check", func(t *testing.T) {
		metrics := NewMetrics()

		handler := metrics.HealthCheckHandler()
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, nil)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
		require.Contains(t, recorder.Body.String(), `"status":"healthy"`)
	})

	t.Run("middleware", func(t *testing.T) {
		metrics := NewMetrics()

		// Create a simple handler to test middleware behavior
		nextCalled := false
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			nextCalled = true
		})

		// Wrap with our middleware
		recorder := httptest.NewRecorder()
		middleware := metrics.Middleware()
		middleware.ServeHTTP(recorder, nextHandler)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "ok", recorder.Body.String())

		// Middleware should have been called
		require.True(t, nextCalled, "middleware wrapper should call next handler")
	})
}
