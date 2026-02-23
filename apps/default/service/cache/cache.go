package cache

import (
	"sync"
	"time"
)

// CacheEntry represents a single cache entry
type CacheEntry struct {
	Value      any
	Expiration int64
}

// IsExpired checks if the cache entry is expired
func (e *CacheEntry) IsExpired() bool {
	if e.Expiration == 0 {
		return false // No expiration
	}
	return time.Now().UnixNano() > e.Expiration
}

// CacheConfig defines the configuration for the cache
type CacheConfig struct {
	// DefaultTTL is the default time-to-live for cache entries
	DefaultTTL time.Duration

	// CleanupInterval is how often to cleanup expired entries
	CleanupInterval time.Duration

	// MaxSize is the maximum number of entries in the cache (0 = unlimited)
	MaxSize int

	// OnEviction is called when an entry is evicted
	OnEviction func(key string, value any)
}

// DefaultCacheConfig returns sensible defaults for the cache
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		MaxSize:         10000,
		OnEviction:      nil,
	}
}

// Cache is a thread-safe in-memory cache with TTL support
type Cache struct {
	mu     sync.RWMutex
	items  map[string]*CacheEntry
	config *CacheConfig
}

// NewCache creates a new cache instance
func NewCache(config *CacheConfig) *Cache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	c := &Cache{
		items:  make(map[string]*CacheEntry),
		config: config,
	}

	// Start cleanup goroutine
	go c.cleanup()

	return c
}

// Set stores a value in the cache with the default TTL
func (c *Cache) Set(key string, value any) {
	c.SetWithTTL(key, value, c.config.DefaultTTL)
}

// SetWithTTL stores a value in the cache with a specific TTL
func (c *Cache) SetWithTTL(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict before adding
	if c.config.MaxSize > 0 && len(c.items) >= c.config.MaxSize {
		c.evictOne()
	}

	var expiration int64
	if ttl > 0 {
		expiration = time.Now().Add(ttl).UnixNano()
	}

	c.items[key] = &CacheEntry{
		Value:      value,
		Expiration: expiration,
	}
}

// Get retrieves a value from the cache
// Returns the value and true if found and not expired, nil and false otherwise
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	item, found := c.items[key]
	c.mu.RUnlock()

	if !found || item.IsExpired() {
		return nil, false
	}

	return item.Value, true
}

// GetBytes retrieves a []byte value from the cache
func (c *Cache) GetBytes(key string) ([]byte, bool) {
	value, found := c.Get(key)
	if !found {
		return nil, false
	}
	if bytes, ok := value.([]byte); ok {
		return bytes, true
	}
	return nil, false
}

// GetString retrieves a string value from the cache
func (c *Cache) GetString(key string) (string, bool) {
	value, found := c.Get(key)
	if !found {
		return "", false
	}
	if str, ok := value.(string); ok {
		return str, true
	}
	return "", false
}

// GetInt retrieves an int value from the cache
func (c *Cache) GetInt(key string) (int, bool) {
	value, found := c.Get(key)
	if !found {
		return 0, false
	}
	if i, ok := value.(int); ok {
		return i, true
	}
	return 0, false
}

// GetInt64 retrieves an int64 value from the cache
func (c *Cache) GetInt64(key string) (int64, bool) {
	value, found := c.Get(key)
	if !found {
		return 0, false
	}
	if i, ok := value.(int64); ok {
		return i, true
	}
	return 0, false
}

// Delete removes a value from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, exists := c.items[key]; exists {
		delete(c.items, key)
		if c.config.OnEviction != nil {
			c.config.OnEviction(key, item.Value)
		}
	}
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.config.OnEviction != nil {
		for key, item := range c.items {
			c.config.OnEviction(key, item.Value)
		}
	}

	c.items = make(map[string]*CacheEntry)
}

// Size returns the number of entries in the cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// cleanup removes expired entries from the cache
func (c *Cache) cleanup() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now().UnixNano()
		for key, item := range c.items {
			if item.Expiration > 0 && now > item.Expiration {
				delete(c.items, key)
				if c.config.OnEviction != nil {
					c.config.OnEviction(key, item.Value)
				}
			}
		}
		c.mu.Unlock()
	}
}

// evictOne evicts a single entry from the cache
// Uses a simple FIFO strategy - in production, consider LRU or LFU
func (c *Cache) evictOne() {
	// Simple FIFO eviction - remove first key
	for key, item := range c.items {
		delete(c.items, key)
		if c.config.OnEviction != nil {
			c.config.OnEviction(key, item.Value)
		}
		return
	}
}

// GetOrSet retrieves a value from the cache or sets it if not found
// Returns the value and whether it was in the cache
func (c *Cache) GetOrSet(key string, valueFunc func() any) (any, bool) {
	// Try to get from cache first
	if value, found := c.Get(key); found {
		return value, true
	}

	// Not found, compute the value
	value := valueFunc()

	// Store in cache
	c.Set(key, value)

	return value, false
}
