package expirable_lru

import (
	"lru/internal"
	"sync"
	"time"
)

const (
	// noEvictionTTL is a very long TTL to prevent eviction
	noEvictionTTL = time.Hour * 24 * 365 * 100

	// because of uint8 usage for nextBucket, it should not exceed 256
	// casting it to uint8 explicitly requires type conversions in multiple places
	numBuckets = 100
)

// EvictCallback is used to get a callback when a cache entry is evicted
type EvictCallback[K comparable, V any] func(key K, value V)

// LRU implements a thread-safe LRU with expirable entries.
type LRU[K comparable, V any] struct {
	size      int
	evictList *internal.LRUList[K, V]
	entries   map[K]*internal.Entry[K, V]
	onEvict   EvictCallback[K, V]

	// expirable options
	lock sync.Mutex
	ttl  time.Duration
	done chan struct{}

	// buckets for expiration
	buckets []bucket[K, V]
	// uint8 because it's a number between 0 and numBuckets
	nextBucket uint8
}

// bucket is a container for holding entries to be expired
type bucket[K comparable, V any] struct {
	entries     map[K]*internal.Entry[K, V]
	newestEntry time.Time
}

// NewLRU returns a new thread-safe cache with expirable entries.
//
// Size parameter set to 0 makes cache of unlimited size, e.g. turns LRU mechanism off.
//
// Providing 0 TTL turns expiring off.
//
// Delete expired entries every 1/100th of TTL value. Goroutine which deletes expired entries runs indefinitely.
func NewLRU[K comparable, V any](size int, onEvict EvictCallback[K, V], ttl time.Duration) *LRU[K, V] {
	if size < 0 {
		size = 0
	}
	if ttl <= 0 {
		ttl = noEvictionTTL
	}

	l := &LRU[K, V]{
		size:      size,
		evictList: internal.NewList[K, V](),
		entries:   make(map[K]*internal.Entry[K, V]),
		onEvict:   onEvict,
		ttl:       ttl,
		done:      make(chan struct{}),
	}

	l.buckets = make([]bucket[K, V], numBuckets)
	for i := 0; i < numBuckets; i++ {
		l.buckets[i] = bucket[K, V]{entries: make(map[K]*internal.Entry[K, V])}
	}

	// enable deleteExpired() running in a separate goroutine for cache with non-zero TTL.
	//
	// Important: done channel is never closed, so deleteExpired() goroutine will never exit.
	// This functionality is not implemented yet.
	if l.ttl != noEvictionTTL {
		go func() {
			ticker := time.NewTicker(l.ttl / numBuckets)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					l.deleteExpired()
				case <-l.done:
					return
				}
			}
		}()
	}

	return l
}

// Add adds an entry to the cache, returns true if an eviction occurred and
// updates the recency of usage of the key.
func (l *LRU[K, V]) Add(key K, value V) (evicted bool) {
	l.lock.Lock()
	defer l.lock.Unlock()

	expiresAt := time.Now().Add(l.ttl)

	// check for existing entry
	if entry, ok := l.entries[key]; ok {
		l.evictList.MoveToFront(entry)
		// remove the entry from its current bucket as expiresAt is updated
		l.removeFromBucket(entry)
		entry.Value = value
		entry.ExpiresAt = expiresAt
		l.addToBucket(entry)
		return false
	}

	// add new entry
	entry := l.evictList.PushToFrontExpirable(key, value, expiresAt)
	l.entries[key] = entry
	// adds the entry to the appropriate bucket and sets entry.Bucket
	l.addToBucket(entry)
	evict := l.size > 0 && l.evictList.Len() > l.size
	// verify if size not exceeded
	if evict {
		l.removeOldest()
	}
	return evict
}

// Get returns key's value from the cache and updates the recency of usage of the key.
// ok specifies if the key was found or not.
func (l *LRU[K, V]) Get(key K) (value V, ok bool) {
	l.lock.Lock()
	defer l.lock.Unlock()
	if entry, ok := l.entries[key]; ok {
		// check if entry has expired
		if time.Now().After(entry.ExpiresAt) {
			return value, false
		}
		l.evictList.MoveToFront(entry)
		return entry.Value, true
	}
	return value, ok
}

// Contains checks if a key exists in the cache without updating the recency of usage.
func (l *LRU[K, V]) Contains(key K) (ok bool) {
	l.lock.Lock()
	defer l.lock.Unlock()
	_, ok = l.entries[key]
	return ok
}

// Peek returns key's value without updating the recency of usage of the key.
// ok specifies if the key was found or not.
func (l *LRU[K, V]) Peek(key K) (value V, ok bool) {
	l.lock.Lock()
	defer l.lock.Unlock()
	if entry, ok := l.entries[key]; ok {
		// check if entry has expired
		if time.Now().After(entry.ExpiresAt) {
			return value, false
		}
		return entry.Value, true
	}
	return value, ok
}

// Remove removes an entry from the cache with the key specified.
// ok specifies if the key was found or not.
func (l *LRU[K, V]) Remove(key K) (ok bool) {
	l.lock.Lock()
	defer l.lock.Unlock()
	if entry, ok := l.entries[key]; ok {
		l.removeEntry(entry)
		return true
	}
	return false
}

// RemoveOldest removes the oldest entry from the cache.
func (l *LRU[K, V]) RemoveOldest() (key K, value V, ok bool) {
	l.lock.Lock()
	defer l.lock.Unlock()
	if entry := l.evictList.Back(); entry != nil {
		l.removeEntry(entry)
		return entry.Key, entry.Value, true
	}
	return key, value, false
}

// GetOldest returns the oldest entry from the cache.
func (l *LRU[K, V]) GetOldest() (key K, value V, ok bool) {
	l.lock.Lock()
	defer l.lock.Unlock()
	if entry := l.evictList.Back(); entry != nil {
		return entry.Key, entry.Value, true
	}
	return key, value, false
}

// Keys returns a slice of the keys in the cache, from oldest to newest.
// Expired entries are filtered out.
func (l *LRU[K, V]) Keys() []K {
	l.lock.Lock()
	defer l.lock.Unlock()
	keys := make([]K, 0, l.evictList.Len())
	now := time.Now()
	for entry := l.evictList.Back(); entry != nil; entry = entry.PrevEntry() {
		if now.After(entry.ExpiresAt) {
			continue
		}
		keys = append(keys, entry.Key)
	}
	return keys
}

// Values returns a slice of the values in the cache, from oldest to newest.
// Expired entries are filtered out.
func (l *LRU[K, V]) Values() []V {
	l.lock.Lock()
	defer l.lock.Unlock()
	values := make([]V, 0, l.evictList.Len())
	now := time.Now()
	for entry := l.evictList.Back(); entry != nil; entry = entry.PrevEntry() {
		if now.After(entry.ExpiresAt) {
			continue
		}
		values = append(values, entry.Value)
	}
	return values
}

// Len returns the number of entries in the cache.
func (l *LRU[K, V]) Len() int {
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.evictList.Len()
}

// Cap returns the capacity of the cache.
func (l *LRU[K, V]) Cap() int {
	return l.size
}

// Purge clears all the cache entries.
func (l *LRU[K, V]) Purge() {
	l.lock.Lock()
	defer l.lock.Unlock()
	for k, v := range l.entries {
		if l.onEvict != nil {
			l.onEvict(k, v.Value)
		}
		delete(l.entries, k)
	}
	for _, b := range l.buckets {
		for _, entry := range b.entries {
			delete(b.entries, entry.Key)
		}
	}
	l.evictList.Init()
}

// Resize changes the cache size, returning number of evicted entries.
// Size of 0 means unlimited.
func (l *LRU[K, V]) Resize(size int) (evicted int) {
	l.lock.Lock()
	defer l.lock.Unlock()
	if size <= 0 {
		l.size = 0
		return 0
	}
	diff := l.evictList.Len() - size
	if diff < 0 {
		diff = 0
	}
	for i := 0; i < diff; i++ {
		l.removeOldest()
	}
	l.size = size
	return diff
}

// removeOldest removes the oldest entry from the cache. Has to be called with lock!
func (l *LRU[K, V]) removeOldest() {
	if entry := l.evictList.Back(); entry != nil {
		l.removeEntry(entry)
	}
}

// removeEntry is used to remove a given list entry from the cache. Has to be called with lock!
func (l *LRU[K, V]) removeEntry(entry *internal.Entry[K, V]) {
	l.evictList.Remove(entry)
	delete(l.entries, entry.Key)
	l.removeFromBucket(entry)
	if l.onEvict != nil {
		l.onEvict(entry.Key, entry.Value)
	}
}

// deleteExpired deletes expired entries from the oldest bucket, waiting for the newest entry
// in it to expire first.
func (l *LRU[K, V]) deleteExpired() {
	l.lock.Lock()
	bucketIndex := l.nextBucket
	timeToExpire := time.Until(l.buckets[bucketIndex].newestEntry)
	// wait for newest entry to expire before cleanup without holding lock
	if timeToExpire > 0 {
		l.lock.Unlock()
		time.Sleep(timeToExpire)
		l.lock.Lock()
	}
	for _, entry := range l.buckets[bucketIndex].entries {
		l.removeEntry(entry)
	}
	l.nextBucket = (l.nextBucket + 1) % numBuckets
	l.lock.Unlock()
}

// addToBucket adds entry to expiry bucket so that it will be cleaned up when the time comes.
// Has to be called with a lock!
func (l *LRU[K, V]) addToBucket(entry *internal.Entry[K, V]) {
	bucketIndex := l.nextBucket % numBuckets
	entry.Bucket = bucketIndex
	l.buckets[bucketIndex].entries[entry.Key] = entry
	if l.buckets[bucketIndex].newestEntry.Before(entry.ExpiresAt) {
		l.buckets[bucketIndex].newestEntry = entry.ExpiresAt
	}
}

// removeFromBucket removes the entry from its corresponding bucket.
// Has to be called with a lock!
func (l *LRU[K, V]) removeFromBucket(entry *internal.Entry[K, V]) {
	delete(l.buckets[entry.Bucket].entries, entry.Key)
}
