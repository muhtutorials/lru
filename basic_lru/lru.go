package basic_lru

import (
	"fmt"
	"lru/internal"
)

// EvictCallback is used to get a callback when a cache entry is evicted
type EvictCallback[K comparable, V any] func(key K, value V)

// LRU implements a non-thread safe fixed size LRU cache
type LRU[K comparable, V any] struct {
	size      int
	evictList *internal.LRUList[K, V]
	entries   map[K]*internal.Entry[K, V]
	onEvict   EvictCallback[K, V]
}

// NewLRU constructs an LRU of the given size
func NewLRU[K comparable, V any](size int, onEvict EvictCallback[K, V]) (*LRU[K, V], error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid cache size (%d), must be bigger than zero", size)
	}

	l := &LRU[K, V]{
		size:      size,
		evictList: internal.NewList[K, V](),
		entries:   make(map[K]*internal.Entry[K, V]),
		onEvict:   onEvict,
	}

	return l, nil
}

// Add adds an entry to the cache, returns true if an eviction occurred and
// updates the recency of usage of the key.
func (l *LRU[K, V]) Add(key K, value V) (evicted bool) {
	// check for existing entry
	if entry, ok := l.entries[key]; ok {
		l.evictList.MoveToFront(entry)
		entry.Value = value
		return false
	}

	// add new entry
	entry := l.evictList.PushToFront(key, value)
	l.entries[key] = entry

	evict := l.evictList.Len() > l.size
	if evict {
		l.removeOldest()
	}
	return evict
}

// Get returns key's value from the cache and updates the recency of usage of the key.
// ok specifies if the key was found or not.
func (l *LRU[K, V]) Get(key K) (value V, ok bool) {
	if entry, ok := l.entries[key]; ok {
		l.evictList.MoveToFront(entry)
		return entry.Value, true
	}
	return value, false
}

// Contains checks if a key exists in the cache without updating the recency of usage.
func (l *LRU[K, V]) Contains(key K) (ok bool) {
	_, ok = l.entries[key]
	return ok
}

// Peek returns key's value without updating the recency of usage of the key.
// ok specifies if the key was found or not.
func (l *LRU[K, V]) Peek(key K) (value V, ok bool) {
	if entry, ok := l.entries[key]; ok {
		return entry.Value, ok
	}
	return value, ok
}

// Remove removes an entry from the cache with the key specified.
// ok specifies if the key was found or not.
func (l *LRU[K, V]) Remove(key K) (ok bool) {
	if entry, ok := l.entries[key]; ok {
		l.removeEntry(entry)
		return true
	}
	return false
}

// RemoveOldest removes the oldest entry from the cache.
func (l *LRU[K, V]) RemoveOldest() (key K, value V, ok bool) {
	if entry := l.evictList.Back(); entry != nil {
		l.removeEntry(entry)
		return entry.Key, entry.Value, true
	}
	return key, value, false
}

// GetOldest returns the oldest entry from the cache.
func (l *LRU[K, V]) GetOldest() (key K, value V, ok bool) {
	if entry := l.evictList.Back(); entry != nil {
		return entry.Key, entry.Value, true
	}
	return key, value, false
}

// Keys returns a slice of the keys in the cache, from oldest to newest.
func (l *LRU[K, V]) Keys() []K {
	keys := make([]K, l.evictList.Len())
	i := 0
	for entry := l.evictList.Back(); entry != nil; entry = entry.PrevEntry() {
		keys[i] = entry.Key
		i++
	}
	return keys
}

// Values returns a slice of the values in the cache, from oldest to newest.
func (l *LRU[K, V]) Values() []V {
	values := make([]V, l.evictList.Len())
	i := 0
	for entry := l.evictList.Back(); entry != nil; entry = entry.PrevEntry() {
		values[i] = entry.Value
		i++
	}
	return values
}

// Len returns the number of entries in the cache.
func (l *LRU[K, V]) Len() int {
	return l.evictList.Len()
}

// Cap returns the capacity of the cache.
func (l *LRU[K, V]) Cap() int {
	return l.size
}

// Purge clears all the cache entries.
func (l *LRU[K, V]) Purge() {
	for k, v := range l.entries {
		if l.onEvict != nil {
			l.onEvict(k, v.Value)
		}
		delete(l.entries, k)
	}
	l.evictList.Init()
}

// Resize changes the cache size, returning number of evicted entries.
func (l *LRU[K, V]) Resize(size int) (evicted int) {
	diff := l.Len() - size
	if diff < 0 {
		diff = 0
	}
	for i := 0; i < diff; i++ {
		l.removeOldest()
	}
	l.size = size
	return diff
}

// removeOldest removes the oldest entry from the cache.
func (l *LRU[K, V]) removeOldest() {
	if entry := l.evictList.Back(); entry != nil {
		l.removeEntry(entry)
	}
}

// removeEntry is used to remove a given list entry from the cache
func (l *LRU[K, V]) removeEntry(entry *internal.Entry[K, V]) {
	l.evictList.Remove(entry)
	delete(l.entries, entry.Key)
	if l.onEvict != nil {
		l.onEvict(entry.Key, entry.Value)
	}
}
