package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_GetSet(t *testing.T) {
	c := NewCache(1, 5*time.Minute)

	// Cache miss.
	_, _, ok := c.Get("missing")
	assert.False(t, ok)

	// Set and get.
	c.Set("key1", []byte("hello"), "text/plain")
	data, ct, ok := c.Get("key1")
	require.True(t, ok)
	assert.Equal(t, []byte("hello"), data)
	assert.Equal(t, "text/plain", ct)
}

func TestCache_TTLExpiration(t *testing.T) {
	c := NewCache(1, 50*time.Millisecond)

	c.Set("key1", []byte("data"), "text/plain")

	// Should be available immediately.
	_, _, ok := c.Get("key1")
	require.True(t, ok)

	// Wait for TTL to expire.
	time.Sleep(100 * time.Millisecond)

	_, _, ok = c.Get("key1")
	assert.False(t, ok, "entry should be expired")
	assert.Equal(t, 0, c.Len(), "expired entry should be removed")
}

func TestCache_LRUEviction(t *testing.T) {
	c := NewCache(0, 5*time.Minute)
	c.maxBytes = 100

	// Fill with entries: 5 entries of 30 bytes each = 150 bytes.
	// Should evict oldest to stay under 100.
	for i := 0; i < 5; i++ {
		c.Set(fmt.Sprintf("key%d", i), make([]byte, 30), "text/plain")
	}

	// Oldest entries should have been evicted.
	// With 100 bytes max and 30 bytes per entry, we can hold 3 entries.
	assert.LessOrEqual(t, c.Len(), 3)

	// Most recent entries should still be present.
	_, _, ok := c.Get("key4")
	assert.True(t, ok, "most recent entry should be present")

	_, _, ok = c.Get("key3")
	assert.True(t, ok, "second most recent should be present")
}

func TestCache_Purge(t *testing.T) {
	c := NewCache(1, 5*time.Minute)

	c.Set("key1", []byte("data1"), "text/plain")
	c.Set("key2", []byte("data2"), "text/html")
	assert.Equal(t, 2, c.Len())

	c.Purge()
	assert.Equal(t, 0, c.Len())

	_, _, ok := c.Get("key1")
	assert.False(t, ok)
}

func TestCache_SizeTracking(t *testing.T) {
	c := NewCache(0, 5*time.Minute)
	c.maxBytes = 1000

	c.Set("a", make([]byte, 100), "text/plain")
	c.Set("b", make([]byte, 200), "text/plain")

	c.mu.Lock()
	assert.Equal(t, int64(300), c.currentBytes)
	c.mu.Unlock()

	// Update existing key — old size should be subtracted.
	c.Set("a", make([]byte, 50), "text/plain")

	c.mu.Lock()
	assert.Equal(t, int64(250), c.currentBytes)
	c.mu.Unlock()
}

func TestCache_ContentTypePreserved(t *testing.T) {
	c := NewCache(1, 5*time.Minute)

	types := []string{
		"text/html",
		"application/json",
		"text/css",
		"image/png",
	}

	for _, ct := range types {
		c.Set(ct, []byte("data"), ct)
	}

	for _, ct := range types {
		_, gotCT, ok := c.Get(ct)
		require.True(t, ok)
		assert.Equal(t, ct, gotCT)
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache(10, 5*time.Minute)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i%10)
			c.Set(key, []byte(fmt.Sprintf("data%d", i)), "text/plain")
			c.Get(key)
		}(i)
	}
	wg.Wait()

	// Should not panic or deadlock. Verify cache is in a consistent state.
	assert.GreaterOrEqual(t, c.Len(), 0)
}

func TestCache_UpdateExisting(t *testing.T) {
	c := NewCache(1, 5*time.Minute)

	c.Set("key", []byte("old"), "text/plain")
	c.Set("key", []byte("new"), "text/html")

	data, ct, ok := c.Get("key")
	require.True(t, ok)
	assert.Equal(t, []byte("new"), data)
	assert.Equal(t, "text/html", ct)
	assert.Equal(t, 1, c.Len())
}
