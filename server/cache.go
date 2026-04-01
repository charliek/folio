package main

import (
	"container/list"
	"sync"
	"time"
)

// Cache is a thread-safe in-memory LRU cache with TTL-based expiration.
type Cache struct {
	mu           sync.Mutex
	items        map[string]*list.Element
	evictList    *list.List
	maxBytes     int64
	currentBytes int64
	ttl          time.Duration
}

type cacheEntry struct {
	key         string
	data        []byte
	contentType string
	expiresAt   time.Time
	size        int64
}

// NewCache creates a new LRU/TTL cache with the given maximum size and TTL.
func NewCache(maxMB int, ttl time.Duration) *Cache {
	return &Cache{
		items:     make(map[string]*list.Element),
		evictList: list.New(),
		maxBytes:  int64(maxMB) * 1024 * 1024,
		ttl:       ttl,
	}
}

// Get retrieves a cached entry by key. Returns the data, content type, and
// whether the entry was found and still valid.
func (c *Cache) Get(key string) ([]byte, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, "", false
	}

	entry := elem.Value.(*cacheEntry)

	// Check TTL expiration.
	if time.Now().After(entry.expiresAt) {
		c.removeElement(elem)
		return nil, "", false
	}

	// Move to front (most recently used).
	c.evictList.MoveToFront(elem)
	return entry.data, entry.contentType, true
}

// Set adds or updates a cache entry. Evicts LRU entries if the cache exceeds
// its maximum size.
func (c *Cache) Set(key string, data []byte, contentType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entrySize := int64(len(data))

	// If the entry already exists, remove the old one first.
	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}

	// Evict LRU entries until there's room.
	for c.currentBytes+entrySize > c.maxBytes && c.evictList.Len() > 0 {
		c.removeOldest()
	}

	entry := &cacheEntry{
		key:         key,
		data:        data,
		contentType: contentType,
		expiresAt:   time.Now().Add(c.ttl),
		size:        entrySize,
	}

	elem := c.evictList.PushFront(entry)
	c.items[key] = elem
	c.currentBytes += entrySize
}

// Purge removes all entries from the cache.
func (c *Cache) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictList.Init()
	c.currentBytes = 0
}

// Len returns the number of entries in the cache.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictList.Len()
}

func (c *Cache) removeElement(elem *list.Element) {
	entry := elem.Value.(*cacheEntry)
	c.evictList.Remove(elem)
	delete(c.items, entry.key)
	c.currentBytes -= entry.size
}

func (c *Cache) removeOldest() {
	elem := c.evictList.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}
