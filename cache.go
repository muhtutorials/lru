package main

import (
	"lru/basic_lru"
	"sync"
)

const (
	// DefaultEvictedBufferSize defines the default buffer size to store evicted key/val
	DefaultEvictedBufferSize = 16
)

// Cache is a thread-safe fixed size LRU cache.
type Cache[K comparable, V any] struct {
	lru           *basic_lru.LRU[K, V]
	evictedKeys   []K
	evictedValues []V
	onEvict       func(key K, value V)
	lock          sync.RWMutex
}

// New creates an LRU of the given size.
func New[K comparable, V any](size int) (*Cache[K, V], error) {
	return NewWithOnEvict[K, V](size, nil)
}

func NewWithOnEvict[K comparable, V any](size int, onEvict func(key K, value V)) (c *Cache[K, V], err error) {
	// create a cache with default settings
	c = &Cache[K, V]{onEvict: onEvict}
	if onEvict != nil {
		c.initEvictBuffers()
		onEvict = c.onEvictCB
	}
	c.lru, err = basic_lru.NewLRU(size, onEvict)
	return c, err
}

func (c *Cache[K, V]) initEvictBuffers() {
	c.evictedKeys = make([]K, 0, DefaultEvictedBufferSize)
	c.evictedValues = make([]V, 0, DefaultEvictedBufferSize)
}

func (c *Cache[K, V]) onEvictCB(key K, value V) {
	c.evictedKeys = append(c.evictedKeys, key)
	c.evictedValues = append(c.evictedValues, value)
}

// Add adds an entry to the cache, returns true if an eviction occurred and
// updates the recency of usage of the key.
func (c *Cache[K, V]) Add(key K, value V) (evicted bool) {
	var (
		k K
		v V
	)
	c.lock.Lock()
	evicted = c.lru.Add(key, value)
	if evicted && c.onEvict != nil {
		k, v = c.evictedKeys[0], c.evictedValues[0]
		c.evictedKeys, c.evictedValues = c.evictedKeys[:0], c.evictedValues[:0]
	}
	c.lock.Unlock()
	if evicted && c.onEvict != nil {
		c.onEvict(k, v)
	}
	return evicted
}

// Get returns key's value from the cache and updates the recency of usage of the key.
// ok specifies if the key was found or not.
func (c *Cache[K, V]) Get(key K) (value V, ok bool) {
	c.lock.Lock()
	value, ok = c.lru.Get(key)
	c.lock.Unlock()
	return value, ok
}

// Contains checks if a key exists in the cache without updating the recency of usage.
func (c *Cache[K, V]) Contains(key K) (ok bool) {
	c.lock.RLock()
	ok = c.lru.Contains(key)
	c.lock.RUnlock()
	return ok
}

// Peek returns key's value without updating the recency of usage of the key.
// ok specifies if the key was found or not.
func (c *Cache[K, V]) Peek(key K) (value V, ok bool) {
	c.lock.RLock()
	value, ok = c.lru.Peek(key)
	c.lock.RUnlock()
	return value, ok
}

// ContainsOrAdd checks if a key is in the cache without updating the
// recency of usage or deleting it for being stale, and if not, adds the value.
// Returns whether it was found and whether an eviction occurred.
func (c *Cache[K, V]) ContainsOrAdd(key K, value V) (ok, evicted bool) {
	var (
		k K
		v V
	)
	c.lock.Lock()
	if c.lru.Contains(key) {
		c.lock.Unlock()
		return true, false
	}
	evicted = c.lru.Add(key, value)
	if evicted && c.onEvict != nil {
		k, v = c.evictedKeys[0], c.evictedValues[0]
		c.evictedKeys, c.evictedValues = c.evictedKeys[:0], c.evictedValues[:0]
	}
	c.lock.Unlock()
	if evicted && c.onEvict != nil {
		c.onEvict(k, v)
	}
	return false, evicted
}

// PeekOrAdd checks if a key is in the cache without updating the
// recency of usage or deleting it for being stale, and if not, adds the value.
// Returns key's previous value if it was found, whether found and whether an eviction occurred.
func (c *Cache[K, V]) PeekOrAdd(key K, value V) (prev V, ok, evicted bool) {
	var (
		k K
		v V
	)
	c.lock.Lock()
	prev, ok = c.lru.Peek(key)
	if ok {
		c.lock.Unlock()
		return prev, ok, false
	}
	evicted = c.lru.Add(key, value)
	if evicted && c.onEvict != nil {
		k, v = c.evictedKeys[0], c.evictedValues[0]
		c.evictedKeys, c.evictedValues = c.evictedKeys[:0], c.evictedValues[:0]
	}
	c.lock.Unlock()
	if evicted && c.onEvict != nil {
		c.onEvict(k, v)
	}
	return prev, ok, evicted
}

// Remove removes an entry from the cache with the key specified.
// ok specifies if the key was found or not.
func (c *Cache[K, V]) Remove(key K) (ok bool) {
	var (
		k K
		v V
	)
	c.lock.Lock()
	ok = c.lru.Remove(key)
	if ok && c.onEvict != nil {
		k, v = c.evictedKeys[0], c.evictedValues[0]
		c.evictedKeys, c.evictedValues = c.evictedKeys[:0], c.evictedValues[:0]
	}
	c.lock.Unlock()
	if ok && c.onEvict != nil {
		c.onEvict(k, v)
	}
	return ok
}

// RemoveOldest removes the oldest entry from the cache.
func (c *Cache[K, V]) RemoveOldest() (key K, value V, ok bool) {
	var (
		k K
		v V
	)
	c.lock.Lock()
	key, value, ok = c.lru.RemoveOldest()
	if ok && c.onEvict != nil {
		k, v = c.evictedKeys[0], c.evictedValues[0]
		c.evictedKeys, c.evictedValues = c.evictedKeys[:0], c.evictedValues[:0]
	}
	c.lock.Unlock()
	if ok && c.onEvict != nil {
		c.onEvict(k, v)
	}
	return key, value, ok
}

// GetOldest returns the oldest entry from the cache.
func (c *Cache[K, V]) GetOldest() (key K, value V, ok bool) {
	c.lock.RLock()
	key, value, ok = c.lru.GetOldest()
	c.lock.RUnlock()
	return key, value, ok
}

// Keys returns a slice of the keys in the cache, from oldest to newest.
func (c *Cache[K, V]) Keys() []K {
	c.lock.RLock()
	keys := c.lru.Keys()
	c.lock.RUnlock()
	return keys
}

// Values returns a slice of the values in the cache, from oldest to newest.
func (c *Cache[K, V]) Values() []V {
	c.lock.RLock()
	values := c.lru.Values()
	c.lock.RUnlock()
	return values
}

// Len returns the number of entries in the cache.
func (c *Cache[K, V]) Len() int {
	c.lock.RLock()
	length := c.lru.Len()
	c.lock.RUnlock()
	return length
}

// Cap returns the capacity of the cache.
func (c *Cache[K, V]) Cap() int {
	return c.lru.Cap()
}

// Purge clears all the cache entries.
func (c *Cache[K, V]) Purge() {
	var (
		keys   []K
		values []V
	)
	c.lock.Lock()
	c.lru.Purge()
	if c.onEvict != nil && len(c.evictedKeys) > 0 {
		keys, values = c.evictedKeys, c.evictedValues
		c.initEvictBuffers()
	}
	c.lock.Unlock()
	if c.onEvict != nil {
		for i := 0; i < len(keys); i++ {
			c.onEvict(keys[i], values[i])
		}
	}
}

// Resize changes the cache size, returning number of evicted entries.
func (c *Cache[K, V]) Resize(size int) (evicted int) {
	var (
		keys   []K
		values []V
	)
	c.lock.Lock()
	evicted = c.lru.Resize(size)
	if evicted > 0 && c.onEvict != nil {
		keys, values = c.evictedKeys, c.evictedValues
		c.initEvictBuffers()
	}
	c.lock.Unlock()
	if evicted > 0 && c.onEvict != nil {
		for i := 0; i < len(keys); i++ {
			c.onEvict(keys[i], values[i])
		}
	}
	return evicted
}
