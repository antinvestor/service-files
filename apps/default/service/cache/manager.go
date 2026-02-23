package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/types"
)

// Manager manages multiple cache instances for different data types
type Manager struct {
	thumbnailCache    *Cache
	metadataCache     *Cache
	urlPreviewCache   *Cache
	storageStatsCache *Cache
}

// ManagerConfig defines the configuration for the cache manager
type ManagerConfig struct {
	ThumbnailCache    *CacheConfig
	MetadataCache     *CacheConfig
	URLPreviewCache   *CacheConfig
	StorageStatsCache *CacheConfig
}

// DefaultManagerConfig returns sensible defaults for the cache manager
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		ThumbnailCache: &CacheConfig{
			DefaultTTL:      30 * time.Minute, // Thumbnails rarely change
			CleanupInterval: 5 * time.Minute,
			MaxSize:         5000, // Thumbnails are larger
			OnEviction:      nil,
		},
		MetadataCache: &CacheConfig{
			DefaultTTL:      10 * time.Minute, // Metadata changes occasionally
			CleanupInterval: 5 * time.Minute,
			MaxSize:         10000,
			OnEviction:      nil,
		},
		URLPreviewCache: &CacheConfig{
			DefaultTTL:      1 * time.Hour, // URL previews are stable
			CleanupInterval: 10 * time.Minute,
			MaxSize:         1000,
			OnEviction:      nil,
		},
		StorageStatsCache: &CacheConfig{
			DefaultTTL:      5 * time.Minute, // Stats are frequently computed
			CleanupInterval: 1 * time.Minute,
			MaxSize:         100,
			OnEviction:      nil,
		},
	}
}

// NewManager creates a new cache manager
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	return &Manager{
		thumbnailCache:    NewCache(config.ThumbnailCache),
		metadataCache:     NewCache(config.MetadataCache),
		urlPreviewCache:   NewCache(config.URLPreviewCache),
		storageStatsCache: NewCache(config.StorageStatsCache),
	}
}

// ThumbnailCache methods

// GetThumbnail retrieves cached thumbnail data
func (m *Manager) GetThumbnail(mediaID string, width, height int, method string) ([]byte, bool) {
	key := m.thumbnailKey(mediaID, width, height, method)
	return m.thumbnailCache.GetBytes(key)
}

// SetThumbnail stores thumbnail data in cache
func (m *Manager) SetThumbnail(mediaID string, width, height int, method string, data []byte) {
	key := m.thumbnailKey(mediaID, width, height, method)
	m.thumbnailCache.Set(key, data)
}

// DeleteThumbnail removes a thumbnail from cache
func (m *Manager) DeleteThumbnail(mediaID string, width, height int, method string) {
	key := m.thumbnailKey(mediaID, width, height, method)
	m.thumbnailCache.Delete(key)
}

// thumbnailKey generates a unique cache key for a thumbnail
func (m *Manager) thumbnailKey(mediaID string, width, height int, method string) string {
	return fmt.Sprintf("thumb:%s:%dx%d:%s", mediaID, width, height, method)
}

// DeleteAllThumbnails removes all thumbnails for a media ID
func (m *Manager) DeleteAllThumbnails(mediaID string) {
	// In production, implement prefix-based deletion or use a cache with prefix support
	// For now, we'll skip this as the simple cache doesn't support prefix deletion
}

// MetadataCache methods

// GetMetadata retrieves cached metadata
func (m *Manager) GetMetadata(mediaID types.MediaID) (*types.MediaMetadata, bool) {
	key := fmt.Sprintf("meta:%s", mediaID)
	value, found := m.metadataCache.Get(key)
	if !found {
		return nil, false
	}
	if metadata, ok := value.(*types.MediaMetadata); ok {
		return metadata, true
	}
	return nil, false
}

// SetMetadata stores metadata in cache
func (m *Manager) SetMetadata(mediaID types.MediaID, metadata *types.MediaMetadata) {
	key := fmt.Sprintf("meta:%s", mediaID)
	m.metadataCache.Set(key, metadata)
}

// DeleteMetadata removes metadata from cache
func (m *Manager) DeleteMetadata(mediaID types.MediaID) {
	key := fmt.Sprintf("meta:%s", mediaID)
	m.metadataCache.Delete(key)
}

// URLPreviewCache methods

// URLPreviewData represents cached URL preview data
type URLPreviewData struct {
	Title       string
	Description string
	ImageURL    string
	SiteName    string
}

// GetURLPreview retrieves cached URL preview
func (m *Manager) GetURLPreview(url string) (*URLPreviewData, bool) {
	key := m.urlPreviewKey(url)
	value, found := m.urlPreviewCache.Get(key)
	if !found {
		return nil, false
	}
	if preview, ok := value.(*URLPreviewData); ok {
		return preview, true
	}
	return nil, false
}

// SetURLPreview stores URL preview in cache
func (m *Manager) SetURLPreview(url string, preview *URLPreviewData) {
	key := m.urlPreviewKey(url)
	m.urlPreviewCache.Set(key, preview)
}

// urlPreviewKey generates a cache key for URL preview
func (m *Manager) urlPreviewKey(url string) string {
	hash := sha256.Sum256([]byte(url))
	return fmt.Sprintf("url_preview:%s", hex.EncodeToString(hash[:]))
}

// StorageStatsCache methods

// StorageStatsData represents cached storage stats
type StorageStatsData struct {
	TotalBytes   int64
	FileCount    int
	UserCount    int
	PublicBytes  int64
	PrivateBytes int64
	ComputedAt   time.Time
}

// GetStorageStats retrieves cached storage stats
func (m *Manager) GetStorageStats(ctx context.Context, userID string) (*StorageStatsData, bool) {
	key := fmt.Sprintf("stats:%s", userID)
	value, found := m.storageStatsCache.Get(key)
	if found {
		if data, ok := value.(*StorageStatsData); ok {
			// Check if the stats are still fresh (within 1 minute)
			if time.Since(data.ComputedAt) < time.Minute {
				return data, true
			}
			m.storageStatsCache.Delete(key)
		}
	}
	return nil, false
}

// SetStorageStats stores storage stats in cache
func (m *Manager) SetStorageStats(ctx context.Context, userID string, data *StorageStatsData) {
	data.ComputedAt = time.Now()
	key := fmt.Sprintf("stats:%s", userID)
	m.storageStatsCache.Set(key, data)
}

// InvalidateAll clears all caches
func (m *Manager) InvalidateAll() {
	m.thumbnailCache.Clear()
	m.metadataCache.Clear()
	m.urlPreviewCache.Clear()
	m.storageStatsCache.Clear()
}

// CacheStats provides statistics about the cache
type CacheStats struct {
	ThumbnailSize    int
	MetadataSize     int
	URLPreviewSize   int
	StorageStatsSize int
}

// Stats returns cache statistics
func (m *Manager) Stats() CacheStats {
	return CacheStats{
		ThumbnailSize:    m.thumbnailCache.Size(),
		MetadataSize:     m.metadataCache.Size(),
		URLPreviewSize:   m.urlPreviewCache.Size(),
		StorageStatsSize: m.storageStatsCache.Size(),
	}
}
