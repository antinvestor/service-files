package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pitabwire/util"
)

// writeIgnoreErr writes to the response writer, ignoring errors
// For metrics output, we continue even if some writes fail
func writeIgnoreErr(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// Metrics represents the application metrics
type Metrics struct {
	// Request metrics
	requestsTotal    map[string]int64
	requestsDuration map[string][]time.Duration
	activeRequests   int64

	// File metrics
	uploadsTotal   int64
	downloadsTotal int64
	uploadBytes    int64
	downloadBytes  int64
	uploadErrors   int64
	downloadErrors int64

	// Storage metrics
	totalBytesStored   int64
	totalFilesStored   int64
	totalUsers         int64
	publicBytesStored  int64
	privateBytesStored int64

	// Cache metrics
	cacheHits   map[string]int64
	cacheMisses map[string]int64

	mu sync.RWMutex
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		requestsTotal:    make(map[string]int64),
		requestsDuration: make(map[string][]time.Duration),
		cacheHits:        make(map[string]int64),
		cacheMisses:      make(map[string]int64),
	}
}

// RecordRequest records a request
func (m *Metrics) RecordRequest(method, path string, duration time.Duration, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := method + ":" + path
	m.requestsTotal[key]++
	m.requestsDuration[key] = append(m.requestsDuration[key], duration)

	// Keep only last 1000 durations to avoid memory issues
	if len(m.requestsDuration[key]) > 1000 {
		m.requestsDuration[key] = m.requestsDuration[key][len(m.requestsDuration[key])-1000:]
	}
}

// RecordActiveRequest records an active request
func (m *Metrics) RecordActiveRequest(delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeRequests += delta
}

// RecordUpload records a file upload
func (m *Metrics) RecordUpload(size int64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.uploadsTotal++
	if success {
		m.uploadBytes += size
		m.totalBytesStored += size
		m.totalFilesStored++
	} else {
		m.uploadErrors++
	}
}

// RecordDownload records a file download
func (m *Metrics) RecordDownload(size int64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.downloadsTotal++
	if success {
		m.downloadBytes += size
	} else {
		m.downloadErrors++
	}
}

// RecordStorageStats records storage statistics
func (m *Metrics) RecordStorageStats(totalBytes, totalFiles, totalUsers, publicBytes, privateBytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalBytesStored = totalBytes
	m.totalFilesStored = totalFiles
	m.totalUsers = totalUsers
	m.publicBytesStored = publicBytes
	m.privateBytesStored = privateBytes
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit(cacheType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cacheHits[cacheType]++
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss(cacheType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cacheMisses[cacheType]++
}

// GetRequestMetrics returns request metrics
func (m *Metrics) GetRequestMetrics() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.requestsTotal
}

// GetActiveRequests returns the number of active requests
func (m *Metrics) GetActiveRequests() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.activeRequests
}

// GetUploadMetrics returns upload metrics
func (m *Metrics) GetUploadMetrics() (total, bytes, errors int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.uploadsTotal, m.uploadBytes, m.uploadErrors
}

// GetDownloadMetrics returns download metrics
func (m *Metrics) GetDownloadMetrics() (total, bytes, errors int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.downloadsTotal, m.downloadBytes, m.downloadErrors
}

// GetStorageMetrics returns storage metrics
func (m *Metrics) GetStorageMetrics() (totalBytes, totalFiles, totalUsers, publicBytes, privateBytes int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.totalBytesStored, m.totalFilesStored, m.totalUsers,
		m.publicBytesStored, m.privateBytesStored
}

// GetCacheMetrics returns cache metrics
func (m *Metrics) GetCacheMetrics() (hits, misses map[string]int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cacheHits, m.cacheMisses
}

// GetAverageDuration returns the average request duration for a given endpoint
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

// GetPercentileDuration returns the p-th percentile duration for a given endpoint
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

// Handler returns an HTTP handler that exposes metrics in Prometheus format
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		// Active requests
		active := m.GetActiveRequests()
		writeIgnoreErr(w, "# HELP file_service_active_requests Current number of active requests")
		writeIgnoreErr(w, "# TYPE file_service_active_requests gauge")
		writeIgnoreErr(w, "file_service_active_requests %d\n", active)

		// Upload metrics
		uploads, uploadBytes, uploadErrors := m.GetUploadMetrics()
		writeIgnoreErr(w, "\n# HELP file_service_uploads_total Total number of file uploads")
		writeIgnoreErr(w, "# TYPE file_service_uploads_total counter")
		writeIgnoreErr(w, "file_service_uploads_total %d\n", uploads)

		writeIgnoreErr(w, "\n# HELP file_service_upload_bytes_total Total bytes uploaded\n")
		writeIgnoreErr(w, "# TYPE file_service_upload_bytes_total counter\n")
		writeIgnoreErr(w, "file_service_upload_bytes_total %d\n", uploadBytes)

		writeIgnoreErr(w, "\n# HELP file_service_upload_errors_total Total number of upload errors\n")
		writeIgnoreErr(w, "# TYPE file_service_upload_errors_total counter\n")
		writeIgnoreErr(w, "file_service_upload_errors_total %d\n", uploadErrors)

		// Download metrics
		downloads, downloadBytes, downloadErrors := m.GetDownloadMetrics()
		writeIgnoreErr(w, "\n# HELP file_service_downloads_total Total number of file downloads\n")
		writeIgnoreErr(w, "# TYPE file_service_downloads_total counter\n")
		writeIgnoreErr(w, "file_service_downloads_total %d\n", downloads)

		writeIgnoreErr(w, "\n# HELP file_service_download_bytes_total Total bytes downloaded\n")
		writeIgnoreErr(w, "# TYPE file_service_download_bytes_total counter\n")
		writeIgnoreErr(w, "file_service_download_bytes_total %d\n", downloadBytes)

		writeIgnoreErr(w, "\n# HELP file_service_download_errors_total Total number of download errors\n")
		writeIgnoreErr(w, "# TYPE file_service_download_errors_total counter\n")
		writeIgnoreErr(w, "file_service_download_errors_total %d\n", downloadErrors)

		// Storage metrics
		totalBytes, totalFiles, totalUsers, publicBytes, privateBytes := m.GetStorageMetrics()
		writeIgnoreErr(w, "\n# HELP file_service_storage_bytes_total Total bytes stored\n")
		writeIgnoreErr(w, "# TYPE file_service_storage_bytes_total gauge\n")
		writeIgnoreErr(w, "file_service_storage_bytes_total %d\n", totalBytes)

		writeIgnoreErr(w, "\n# HELP file_service_storage_files_total Total files stored\n")
		writeIgnoreErr(w, "# TYPE file_service_storage_files_total gauge\n")
		writeIgnoreErr(w, "file_service_storage_files_total %d\n", totalFiles)

		writeIgnoreErr(w, "\n# HELP file_service_storage_users_total Total users\n")
		writeIgnoreErr(w, "# TYPE file_service_storage_users_total gauge\n")
		writeIgnoreErr(w, "file_service_storage_users_total %d\n", totalUsers)

		writeIgnoreErr(w, "\n# HELP file_service_storage_public_bytes_total Public bytes stored\n")
		writeIgnoreErr(w, "# TYPE file_service_storage_public_bytes_total gauge\n")
		writeIgnoreErr(w, "file_service_storage_public_bytes_total %d\n", publicBytes)

		writeIgnoreErr(w, "\n# HELP file_service_storage_private_bytes_total Private bytes stored\n")
		writeIgnoreErr(w, "# TYPE file_service_storage_private_bytes_total gauge\n")
		writeIgnoreErr(w, "file_service_storage_private_bytes_total %d\n", privateBytes)

		// Cache metrics
		hits, misses := m.GetCacheMetrics()
		writeIgnoreErr(w, "\n# HELP file_service_cache_hits_total Cache hits total\n")
		writeIgnoreErr(w, "# TYPE file_service_cache_hits_total counter\n")
		for cacheType, count := range hits {
			writeIgnoreErr(w, "file_service_cache_hits_total{cache_type=\"%s\"} %d\n", cacheType, count)
		}

		writeIgnoreErr(w, "\n# HELP file_service_cache_misses_total Cache misses total\n")
		writeIgnoreErr(w, "# TYPE file_service_cache_misses_total counter\n")
		for cacheType, count := range misses {
			writeIgnoreErr(w, "file_service_cache_misses_total{cache_type=\"%s\"} %d\n", cacheType, count)
		}

		// Request count metrics
		requests := m.GetRequestMetrics()
		writeIgnoreErr(w, "\n# HELP file_service_requests_total Total requests\n")
		writeIgnoreErr(w, "# TYPE file_service_requests_total counter\n")
		for key, count := range requests {
			writeIgnoreErr(w, "file_service_requests_total{endpoint=\"%s\"} %d\n", key, count)
		}

		writeIgnoreErr(w, "\n# HELP file_service_up Service health status\n")
		writeIgnoreErr(w, "# TYPE file_service_up gauge\n")
		writeIgnoreErr(w, "file_service_up 1\n")
	})
}

// Middleware returns middleware that records metrics for each request
func (m *Metrics) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			m.RecordActiveRequest(1)
			defer m.RecordActiveRequest(-1)

			// Wrap response writer to capture status code
			wrapped := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     200,
			}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			m.RecordRequest(r.Method, r.URL.Path, duration, wrapped.statusCode)

			if duration > 5*time.Second {
				util.Log(r.Context()).WithField("duration", duration).
					WithField("path", r.URL.Path).
					Warn("slow request detected")
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
