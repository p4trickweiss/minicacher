package cache

import (
	"maps"
	"sync"
)

// Cache defines the interface for key-value storage operations
type Cache interface {
	Get(key string) string
	Set(key string, value string)
	Delete(key string)
	Snapshot() map[string]string
	Restore(data map[string]string)
}

// Options configures cache behavior (extensibility for future features)
type Options struct {
	// Future: MaxSize for LRU eviction
	// Future: DefaultTTL for expiration
}

// InMemoryCache is a thread-safe in-memory implementation of Cache
type InMemoryCache struct {
	mu sync.RWMutex
	kv map[string]string
}

// New creates a new in-memory cache
func New(opts Options) *InMemoryCache {
	return &InMemoryCache{
		kv: make(map[string]string),
	}
}

// Get retrieves a value by key
func (c *InMemoryCache) Get(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.kv[key]
}

// Set stores a key-value pair
func (c *InMemoryCache) Set(key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.kv[key] = value
}

// Delete removes a key
func (c *InMemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.kv, key)
}

// Snapshot creates a point-in-time copy for Raft snapshots
func (c *InMemoryCache) Snapshot() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return maps.Clone(c.kv)
}

// Restore replaces the entire cache contents (used during Raft restore)
func (c *InMemoryCache) Restore(data map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.kv = data
}
