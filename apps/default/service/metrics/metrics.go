// Package metrics exposes the file service's product metrics through
// OpenTelemetry. Instruments are created with frame's BusinessMetrics
// factory, so every measurement transparently carries tenant_id and
// partition_id derived from the context's security claims. The historic
// file_service_* metric names are preserved so existing dashboards and
// alerts keep working; metrics are exported through the service's OTel
// pipeline instead of the previous bespoke Prometheus text handler.
package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pitabwire/frame/telemetry"
	"github.com/pitabwire/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "file_service"

// writeIgnoreErr writes to the response writer, ignoring errors.
// For probe output, we continue even if some writes fail.
func writeIgnoreErr(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// Metrics wraps the service's OTel instruments behind the same simple
// recording API the previous in-memory registry offered. A small amount of
// internal state is retained: gauges (active requests, storage totals) need
// current values to record, and the Get* accessors used by tests and
// diagnostics read from it.
type Metrics struct {
	// Request instruments.
	requestsTotalCounter telemetry.Counter
	activeRequestsGauge  telemetry.Gauge

	// File transfer instruments.
	uploadsCounter        telemetry.Counter
	uploadBytesCounter    telemetry.Counter
	uploadErrorsCounter   telemetry.Counter
	downloadsCounter      telemetry.Counter
	downloadBytesCounter  telemetry.Counter
	downloadErrorsCounter telemetry.Counter

	// Storage instruments.
	storageBytesGauge        telemetry.Gauge
	storageFilesGauge        telemetry.Gauge
	storageUsersGauge        telemetry.Gauge
	storagePublicBytesGauge  telemetry.Gauge
	storagePrivateBytesGauge telemetry.Gauge

	// Cache instruments.
	cacheHitsCounter   telemetry.Counter
	cacheMissesCounter telemetry.Counter

	// Internal state backing gauges and the Get* accessors.
	requestsTotal    map[string]int64
	requestsDuration map[string][]time.Duration
	activeRequests   int64

	uploadsTotal   int64
	downloadsTotal int64
	uploadBytes    int64
	downloadBytes  int64
	uploadErrors   int64
	downloadErrors int64

	totalBytesStored   int64
	totalFilesStored   int64
	totalUsers         int64
	publicBytesStored  int64
	privateBytesStored int64

	cacheHits   map[string]int64
	cacheMisses map[string]int64

	mu sync.RWMutex
}

// NewMetrics creates a new metrics instance with all OTel instruments
// registered under their historic file_service_* names.
func NewMetrics() *Metrics {
	bm := telemetry.NewBusinessMetrics(meterName)

	return &Metrics{
		requestsTotalCounter: bm.Counter("file_service_requests_total", "Total requests"),
		activeRequestsGauge:  bm.Gauge("file_service_active_requests", "Current number of active requests"),

		uploadsCounter: bm.Counter("file_service_uploads_total", "Total number of file uploads"),
		uploadBytesCounter: bm.Counter(
			"file_service_upload_bytes_total", "Total bytes uploaded", metric.WithUnit("B"),
		),
		uploadErrorsCounter: bm.Counter("file_service_upload_errors_total", "Total number of upload errors"),
		downloadsCounter:    bm.Counter("file_service_downloads_total", "Total number of file downloads"),
		downloadBytesCounter: bm.Counter(
			"file_service_download_bytes_total", "Total bytes downloaded", metric.WithUnit("B"),
		),
		downloadErrorsCounter: bm.Counter("file_service_download_errors_total", "Total number of download errors"),

		storageBytesGauge: bm.Gauge(
			"file_service_storage_bytes_total", "Total bytes stored", metric.WithUnit("B"),
		),
		storageFilesGauge: bm.Gauge("file_service_storage_files_total", "Total files stored"),
		storageUsersGauge: bm.Gauge("file_service_storage_users_total", "Total users"),
		storagePublicBytesGauge: bm.Gauge(
			"file_service_storage_public_bytes_total", "Public bytes stored", metric.WithUnit("B"),
		),
		storagePrivateBytesGauge: bm.Gauge(
			"file_service_storage_private_bytes_total", "Private bytes stored", metric.WithUnit("B"),
		),

		cacheHitsCounter:   bm.Counter("file_service_cache_hits_total", "Cache hits total"),
		cacheMissesCounter: bm.Counter("file_service_cache_misses_total", "Cache misses total"),

		requestsTotal:    make(map[string]int64),
		requestsDuration: make(map[string][]time.Duration),
		cacheHits:        make(map[string]int64),
		cacheMisses:      make(map[string]int64),
	}
}

// RecordRequest records a completed request for the given endpoint.
func (m *Metrics) RecordRequest(ctx context.Context, method, path string, duration time.Duration, _ int) {
	key := method + ":" + path

	m.mu.Lock()
	m.requestsTotal[key]++
	m.requestsDuration[key] = append(m.requestsDuration[key], duration)

	// Keep only last 1000 durations to avoid memory issues.
	if len(m.requestsDuration[key]) > 1000 {
		m.requestsDuration[key] = m.requestsDuration[key][len(m.requestsDuration[key])-1000:]
	}
	m.mu.Unlock()

	m.requestsTotalCounter.Add(ctx, 1, attribute.String("endpoint", key))
}

// RecordActiveRequest adjusts the active request gauge by delta.
func (m *Metrics) RecordActiveRequest(ctx context.Context, delta int64) {
	m.mu.Lock()
	m.activeRequests += delta
	active := m.activeRequests
	m.mu.Unlock()

	m.activeRequestsGauge.Record(ctx, active)
}

// RecordUpload records a file upload.
func (m *Metrics) RecordUpload(ctx context.Context, size int64, success bool) {
	m.mu.Lock()
	m.uploadsTotal++
	if success {
		m.uploadBytes += size
		m.totalBytesStored += size
		m.totalFilesStored++
	} else {
		m.uploadErrors++
	}
	totalBytes, totalFiles := m.totalBytesStored, m.totalFilesStored
	m.mu.Unlock()

	m.uploadsCounter.Add(ctx, 1)
	if success {
		m.uploadBytesCounter.Add(ctx, size)
		m.storageBytesGauge.Record(ctx, totalBytes)
		m.storageFilesGauge.Record(ctx, totalFiles)
	} else {
		m.uploadErrorsCounter.Add(ctx, 1)
	}
}

// RecordDownload records a file download.
func (m *Metrics) RecordDownload(ctx context.Context, size int64, success bool) {
	m.mu.Lock()
	m.downloadsTotal++
	if success {
		m.downloadBytes += size
	} else {
		m.downloadErrors++
	}
	m.mu.Unlock()

	m.downloadsCounter.Add(ctx, 1)
	if success {
		m.downloadBytesCounter.Add(ctx, size)
	} else {
		m.downloadErrorsCounter.Add(ctx, 1)
	}
}

// RecordStorageStats records storage statistics.
func (m *Metrics) RecordStorageStats(ctx context.Context, totalBytes, totalFiles, totalUsers, publicBytes, privateBytes int64) {
	m.mu.Lock()
	m.totalBytesStored = totalBytes
	m.totalFilesStored = totalFiles
	m.totalUsers = totalUsers
	m.publicBytesStored = publicBytes
	m.privateBytesStored = privateBytes
	m.mu.Unlock()

	m.storageBytesGauge.Record(ctx, totalBytes)
	m.storageFilesGauge.Record(ctx, totalFiles)
	m.storageUsersGauge.Record(ctx, totalUsers)
	m.storagePublicBytesGauge.Record(ctx, publicBytes)
	m.storagePrivateBytesGauge.Record(ctx, privateBytes)
}

// RecordCacheHit records a cache hit.
func (m *Metrics) RecordCacheHit(ctx context.Context, cacheType string) {
	m.mu.Lock()
	m.cacheHits[cacheType]++
	m.mu.Unlock()

	m.cacheHitsCounter.Add(ctx, 1, attribute.String("cache_type", cacheType))
}

// RecordCacheMiss records a cache miss.
func (m *Metrics) RecordCacheMiss(ctx context.Context, cacheType string) {
	m.mu.Lock()
	m.cacheMisses[cacheType]++
	m.mu.Unlock()

	m.cacheMissesCounter.Add(ctx, 1, attribute.String("cache_type", cacheType))
}

// GetRequestMetrics returns request metrics.
func (m *Metrics) GetRequestMetrics() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.requestsTotal
}

// GetActiveRequests returns the number of active requests.
func (m *Metrics) GetActiveRequests() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.activeRequests
}

// GetUploadMetrics returns upload metrics.
func (m *Metrics) GetUploadMetrics() (total, bytes, errors int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.uploadsTotal, m.uploadBytes, m.uploadErrors
}

// GetDownloadMetrics returns download metrics.
func (m *Metrics) GetDownloadMetrics() (total, bytes, errors int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.downloadsTotal, m.downloadBytes, m.downloadErrors
}

// GetStorageMetrics returns storage metrics.
func (m *Metrics) GetStorageMetrics() (totalBytes, totalFiles, totalUsers, publicBytes, privateBytes int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.totalBytesStored, m.totalFilesStored, m.totalUsers,
		m.publicBytesStored, m.privateBytesStored
}

// GetCacheMetrics returns cache metrics.
func (m *Metrics) GetCacheMetrics() (hits, misses map[string]int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cacheHits, m.cacheMisses
}

// GetAverageDuration returns the average request duration for a given endpoint.
func (m *Metrics) GetAverageDuration(method, path string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := method + ":" + path
	durations := m.requestsDuration[key]
	if len(durations) == 0 {
		return 0
	}

	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

// GetPercentileDuration returns the p-th percentile duration for a given endpoint.
func (m *Metrics) GetPercentileDuration(method, path string, p float64) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := method + ":" + path
	durations := m.requestsDuration[key]
	if len(durations) == 0 {
		return 0
	}

	// Simple implementation - sort and find percentile
	// For production, use a proper histogram
	// This is simplified for now
	return durations[len(durations)*int(p)/100]
}

// Middleware returns middleware that records metrics for each request.
func (m *Metrics) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			start := time.Now()
			m.RecordActiveRequest(ctx, 1)
			defer m.RecordActiveRequest(ctx, -1)

			// Wrap response writer to capture status code
			wrapped := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     200,
			}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			m.RecordRequest(ctx, r.Method, r.URL.Path, duration, wrapped.statusCode)

			if duration > 5*time.Second {
				util.Log(ctx).WithFields(map[string]any{
					"duration": duration,
					"path":     r.URL.Path,
				}).Warn("slow request detected")
			}
		})
	}
}

// responseWriterWrapper wraps http.ResponseWriter to capture status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// HealthCheckHandler returns a health check handler
func (m *Metrics) HealthCheckHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeIgnoreErr(w, `{"status":"healthy","service":"file-service"}`)
	})
}

// ReadyCheckHandler returns a readiness check handler
func (m *Metrics) ReadyCheckHandler(isReady func() bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if isReady != nil && !isReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			writeIgnoreErr(w, `{"status":"not_ready","service":"file-service"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		writeIgnoreErr(w, `{"status":"ready","service":"file-service"}`)
	})
}
