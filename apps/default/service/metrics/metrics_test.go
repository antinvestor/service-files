package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pitabwire/frame/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	require.NotNil(t, m)

	// Verify metrics are properly initialised
	assert.Equal(t, int64(0), m.GetActiveRequests())
}

func TestRecordRequest(t *testing.T) {
	m := NewMetrics()
	ctx := context.Background()

	m.RecordRequest(ctx, "GET", "/api/test", 100*time.Millisecond, 200)
	m.RecordRequest(ctx, "GET", "/api/test", 150*time.Millisecond, 200)

	reqMetrics := m.GetRequestMetrics()
	assert.Equal(t, int64(2), reqMetrics["GET:/api/test"])

	// Test duration tracking
	avg := m.GetAverageDuration("GET", "/api/test")
	assert.Equal(t, 125*time.Millisecond, avg)
}

func TestRecordActiveRequest(t *testing.T) {
	m := NewMetrics()
	ctx := context.Background()

	assert.Equal(t, int64(0), m.GetActiveRequests())

	m.RecordActiveRequest(ctx, 1)
	assert.Equal(t, int64(1), m.GetActiveRequests())

	m.RecordActiveRequest(ctx, 1)
	assert.Equal(t, int64(2), m.GetActiveRequests())

	m.RecordActiveRequest(ctx, -1)
	assert.Equal(t, int64(1), m.GetActiveRequests())
}

func TestRecordUpload(t *testing.T) {
	m := NewMetrics()
	ctx := context.Background()

	m.RecordUpload(ctx, 1024, true)
	m.RecordUpload(ctx, 2048, true)
	m.RecordUpload(ctx, 512, false)

	total, bytes, errors := m.GetUploadMetrics()
	assert.Equal(t, int64(3), total)
	assert.Equal(t, int64(3072), bytes) // Only successful uploads
	assert.Equal(t, int64(1), errors)
}

func TestRecordDownload(t *testing.T) {
	m := NewMetrics()
	ctx := context.Background()

	m.RecordDownload(ctx, 1024, true)
	m.RecordDownload(ctx, 2048, false)

	total, bytes, errors := m.GetDownloadMetrics()
	assert.Equal(t, int64(2), total)
	assert.Equal(t, int64(1024), bytes) // Only successful downloads
	assert.Equal(t, int64(1), errors)
}

func TestRecordStorageStats(t *testing.T) {
	m := NewMetrics()

	m.RecordStorageStats(context.Background(), 1000000, 100, 10, 500000, 500000)

	totalBytes, totalFiles, totalUsers, publicBytes, privateBytes := m.GetStorageMetrics()
	assert.Equal(t, int64(1000000), totalBytes)
	assert.Equal(t, int64(100), totalFiles)
	assert.Equal(t, int64(10), totalUsers)
	assert.Equal(t, int64(500000), publicBytes)
	assert.Equal(t, int64(500000), privateBytes)
}

func TestRecordCacheHit(t *testing.T) {
	m := NewMetrics()
	ctx := context.Background()

	m.RecordCacheHit(ctx, "thumbnail")
	m.RecordCacheHit(ctx, "thumbnail")
	m.RecordCacheHit(ctx, "metadata")

	hits, _ := m.GetCacheMetrics()
	assert.Equal(t, int64(2), hits["thumbnail"])
	assert.Equal(t, int64(1), hits["metadata"])
}

func TestRecordCacheMiss(t *testing.T) {
	m := NewMetrics()
	ctx := context.Background()

	m.RecordCacheMiss(ctx, "thumbnail")
	m.RecordCacheMiss(ctx, "metadata")

	_, misses := m.GetCacheMetrics()
	assert.Equal(t, int64(1), misses["thumbnail"])
	assert.Equal(t, int64(1), misses["metadata"])
}

func TestGetAverageDuration(t *testing.T) {
	m := NewMetrics()
	ctx := context.Background()

	// Test with no requests
	avg := m.GetAverageDuration("GET", "/test")
	assert.Equal(t, time.Duration(0), avg)

	// Test with requests
	m.RecordRequest(ctx, "GET", "/test", 100*time.Millisecond, 200)
	m.RecordRequest(ctx, "GET", "/test", 200*time.Millisecond, 200)
	m.RecordRequest(ctx, "GET", "/test", 300*time.Millisecond, 200)

	avg = m.GetAverageDuration("GET", "/test")
	assert.Equal(t, 200*time.Millisecond, avg)
}

func TestGetPercentileDuration(t *testing.T) {
	m := NewMetrics()
	ctx := context.Background()

	// Test with no requests
	pct := m.GetPercentileDuration("GET", "/test", 50)
	assert.Equal(t, time.Duration(0), pct)

	// Test with requests
	// The implementation uses zero-based indexing, so for 10 items:
	// 50th percentile = index 5 (6th item) = 600ms
	// 90th percentile = index 9 (10th item) = 1000ms
	for i := 0; i < 10; i++ {
		m.RecordRequest(ctx, "GET", "/test", time.Duration(i+1)*100*time.Millisecond, 200)
	}

	pct = m.GetPercentileDuration("GET", "/test", 50)
	assert.Equal(t, 600*time.Millisecond, pct)

	pct = m.GetPercentileDuration("GET", "/test", 90)
	assert.Equal(t, 1000*time.Millisecond, pct)
}

func TestRequestDurationLimit(t *testing.T) {
	m := NewMetrics()
	ctx := context.Background()

	// Add many requests to test the 1000 limit
	for i := 0; i < 1500; i++ {
		m.RecordRequest(ctx, "GET", "/test", time.Duration(i)*time.Millisecond, 200)
	}

	reqMetrics := m.GetRequestMetrics()
	assert.Equal(t, int64(1500), reqMetrics["GET:/test"])
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

// findMetric returns the metric with the given name, or nil when absent.
func findMetric(rm metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i]
			}
		}
	}
	return nil
}

// TestMetricsTenantAttribution proves, via a ManualReader, that the ported
// instruments keep their exact file_service_* names and automatically attach
// tenant_id/partition_id from the context claims: one counter (cache hits,
// with its cache_type attribute) and one gauge (active requests).
func TestMetricsTenantAttribution(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))

	m := NewMetrics()

	claims := &security.AuthenticationClaims{
		TenantID:    "tenant-files-attr",
		PartitionID: "partition-files-attr",
	}
	claims.Subject = "subject-files-attr"
	ctx := claims.ClaimsToContext(context.Background())

	m.RecordCacheHit(ctx, "thumbnail")
	m.RecordActiveRequest(ctx, 1)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	// Counter: exact name, cache_type plus tenant attributes.
	hits := findMetric(rm, "file_service_cache_hits_total")
	require.NotNil(t, hits, "cache hits counter must keep its metric name")
	sum, ok := hits.Data.(metricdata.Sum[int64])
	require.True(t, ok, "cache hits must be an int64 sum")

	var counterMatched bool
	for _, dp := range sum.DataPoints {
		tenant, hasTenant := dp.Attributes.Value("tenant_id")
		if !hasTenant || tenant.AsString() != "tenant-files-attr" {
			continue
		}
		counterMatched = true
		partition, hasPartition := dp.Attributes.Value("partition_id")
		require.True(t, hasPartition, "partition_id must accompany tenant_id")
		require.Equal(t, "partition-files-attr", partition.AsString())
		cacheType, hasCacheType := dp.Attributes.Value("cache_type")
		require.True(t, hasCacheType, "cache_type attribute must be preserved")
		require.Equal(t, "thumbnail", cacheType.AsString())
		require.Equal(t, int64(1), dp.Value)
	}
	require.True(t, counterMatched, "expected a cache hit datapoint attributed to tenant-files-attr")

	// Gauge: exact name with tenant attribution and the recorded value.
	active := findMetric(rm, "file_service_active_requests")
	require.NotNil(t, active, "active requests gauge must keep its metric name")
	gauge, ok := active.Data.(metricdata.Gauge[int64])
	require.True(t, ok, "active requests must be an int64 gauge")

	var gaugeMatched bool
	for _, dp := range gauge.DataPoints {
		tenant, hasTenant := dp.Attributes.Value("tenant_id")
		if !hasTenant || tenant.AsString() != "tenant-files-attr" {
			continue
		}
		gaugeMatched = true
		partition, hasPartition := dp.Attributes.Value("partition_id")
		require.True(t, hasPartition, "partition_id must accompany tenant_id")
		require.Equal(t, "partition-files-attr", partition.AsString())
		require.Equal(t, int64(1), dp.Value)
	}
	require.True(t, gaugeMatched, "expected an active requests datapoint attributed to tenant-files-attr")
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
