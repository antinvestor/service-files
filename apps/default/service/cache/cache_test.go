package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheBasics(t *testing.T) {
	cache := NewCache(nil)

	// Test basic operations
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	val1, found1 := cache.Get("key1")
	require.True(t, found1)
	require.Equal(t, "value1", val1)

	val2, found2 := cache.Get("key2")
	require.True(t, found2)
	require.Equal(t, "value2", val2)

	// Test with TTL
	cache.SetWithTTL("key3", "value3", 50*time.Millisecond)
	val3, found3 := cache.Get("key3")
	require.True(t, found3)
	require.Equal(t, "value3", val3)

	// Test expiration
	time.Sleep(75 * time.Millisecond)
	_, found3after := cache.Get("key3")
	require.False(t, found3after)

	// Test delete
	cache.Delete("key1")
	_, found1after := cache.Get("key1")
	require.False(t, found1after)

	require.Equal(t, 1, cache.Size())

	// Clear
	cache.Set("key4", "value4")
	cache.Clear()
	require.Equal(t, 0, cache.Size())
}

func TestCacheMaxSize(t *testing.T) {
	config := DefaultCacheConfig()
	config.MaxSize = 3

	cache := NewCache(config)

	// Fill to max
	for i := 0; i < 3; i++ {
		cache.Set(string(rune('a'+i)), i)
	}

	require.Equal(t, 3, cache.Size())

	// Adding another should evict one
	cache.Set("evicted", "value")
	require.Equal(t, 3, cache.Size())
}

func TestCacheEviction(t *testing.T) {
	config := DefaultCacheConfig()
	config.MaxSize = 2

	cache := NewCache(config)

	// Fill to max
	cache.Set("a", 1)
	cache.Set("b", 2)
	require.Equal(t, 2, cache.Size())

	// Add third should evict first
	cache.Set("c", 3)

	_, foundA := cache.Get("a")
	_, foundB := cache.Get("b")

	// One should be evicted, one should remain
	foundCount := 0
	if foundA {
		foundCount++
	}
	if foundB {
		foundCount++
	}

	require.Equal(t, 1, foundCount, "exactly one should be evicted")
	require.Equal(t, 2, cache.Size())
}

func TestCacheDefaultConfig(t *testing.T) {
	cfg := DefaultCacheConfig()

	require.Equal(t, 5*time.Minute, cfg.DefaultTTL)
	require.Equal(t, 1*time.Minute, cfg.CleanupInterval)
	require.Equal(t, 10000, cfg.MaxSize)
	require.Nil(t, cfg.OnEviction)
}

func TestCacheGetBytes(t *testing.T) {
	cache := NewCache(nil)

	// Store []byte value
	testBytes := []byte{1, 2, 3, 4, 5}
	cache.Set("bytes-key", testBytes)

	// Get as bytes
	result, found := cache.GetBytes("bytes-key")
	require.True(t, found)
	require.Equal(t, testBytes, result)

	// Get as bytes for non-existent key
	_, found = cache.GetBytes("non-existent")
	require.False(t, found)

	// Get as bytes for wrong type
	cache.Set("string-key", "string value")
	_, found = cache.GetBytes("string-key")
	require.False(t, found)
}

func TestCacheGetString(t *testing.T) {
	cache := NewCache(nil)

	// Store string value
	cache.Set("string-key", "string value")

	// Get as string
	result, found := cache.GetString("string-key")
	require.True(t, found)
	require.Equal(t, "string value", result)

	// Get as string for non-existent key
	result, found = cache.GetString("non-existent")
	require.False(t, found)
	require.Empty(t, result)

	// Get as string for wrong type
	cache.Set("bytes-key", []byte{1, 2, 3})
	result, found = cache.GetString("bytes-key")
	require.False(t, found)
	require.Empty(t, result)
}

func TestCacheGetInt(t *testing.T) {
	cache := NewCache(nil)

	// Store int value
	cache.Set("int-key", 42)

	// Get as int
	result, found := cache.GetInt("int-key")
	require.True(t, found)
	require.Equal(t, 42, result)

	// Get as int for non-existent key
	result, found = cache.GetInt("non-existent")
	require.False(t, found)
	require.Equal(t, 0, result)

	// Get as int for wrong type
	cache.Set("string-key", "not an int")
	result, found = cache.GetInt("string-key")
	require.False(t, found)
	require.Equal(t, 0, result)
}

func TestCacheGetInt64(t *testing.T) {
	cache := NewCache(nil)

	// Store int64 value
	cache.Set("int64-key", int64(9223372036854775807))

	// Get as int64
	result, found := cache.GetInt64("int64-key")
	require.True(t, found)
	require.Equal(t, int64(9223372036854775807), result)

	// Get as int64 for non-existent key
	result, found = cache.GetInt64("non-existent")
	require.False(t, found)
	require.Equal(t, int64(0), result)

	// Get as int64 for wrong type
	cache.Set("string-key", "not an int64")
	result, found = cache.GetInt64("string-key")
	require.False(t, found)
	require.Equal(t, int64(0), result)
}

func TestCacheGetOrSet(t *testing.T) {
	cache := NewCache(nil)

	// Test GetOrSet - value not in cache
	result, found := cache.GetOrSet("new-key", func() any {
		return "computed-value"
	})
	require.False(t, found, "should not have been found in cache")
	require.Equal(t, "computed-value", result)

	// Verify value was stored
	result, found = cache.Get("new-key")
	require.True(t, found)
	require.Equal(t, "computed-value", result)

	// Test GetOrSet - value already in cache
	cache.Set("existing-key", "cached-value")
	callCount := 0
	result, found = cache.GetOrSet("existing-key", func() any {
		callCount++
		return "should-not-be-called"
	})
	require.True(t, found, "should have been found in cache")
	require.Equal(t, "cached-value", result)
	require.Equal(t, 0, callCount, "compute function should not have been called")
}

func TestCacheOnEviction(t *testing.T) {
	evictedKeys := make([]string, 0)

	config := DefaultCacheConfig()
	config.MaxSize = 2
	config.OnEviction = func(key string, value any) {
		evictedKeys = append(evictedKeys, key)
	}

	cache := NewCache(config)

	// Fill to max
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// Add third to trigger eviction
	cache.Set("key3", "value3")

	// Verify eviction callback was called
	require.Equal(t, 1, len(evictedKeys))
	require.NotEmpty(t, evictedKeys[0])

	// Test eviction on Clear - should trigger eviction for 2 remaining items
	evictedKeys = evictedKeys[:0]
	cache.Clear()
	require.Equal(t, 2, len(evictedKeys), "clear should trigger eviction for remaining items")
}

func TestCacheSetWithZeroTTL(t *testing.T) {
	cache := NewCache(nil)

	// Set with zero TTL (no expiration)
	cache.SetWithTTL("permanent-key", "permanent-value", 0)

	// Should still be accessible
	time.Sleep(10 * time.Millisecond)
	result, found := cache.Get("permanent-key")
	require.True(t, found)
	require.Equal(t, "permanent-value", result)
}

func TestCacheCleanupExpired(t *testing.T) {
	config := DefaultCacheConfig()
	config.CleanupInterval = 50 * time.Millisecond
	config.DefaultTTL = 100 * time.Millisecond

	cache := NewCache(config)

	// Add entries with short TTL
	cache.SetWithTTL("temp1", "value1", 50*time.Millisecond)
	cache.SetWithTTL("temp2", "value2", 50*time.Millisecond)
	cache.Set("permanent", "value-permanent")

	// Wait for expiration but not cleanup
	time.Sleep(60 * time.Millisecond)

	// Check temporary entries still return something (not yet cleaned up)
	_, found1 := cache.Get("temp1")
	_, found2 := cache.Get("temp2")
	_, foundPerm := cache.Get("permanent")

	// After accessing expired entries, they should be deleted
	require.False(t, found1, "temp1 should be deleted on access")
	require.False(t, found2, "temp2 should be deleted on access")
	require.True(t, foundPerm, "permanent should still exist")
}

func TestCacheWithNilConfig(t *testing.T) {
	// Should use default config when nil is passed
	cache := NewCache(nil)

	cache.Set("key", "value")
	result, found := cache.Get("key")

	require.True(t, found)
	require.Equal(t, "value", result)
}
