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
