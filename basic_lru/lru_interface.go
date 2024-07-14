package basic_lru

// LRUCache is the interface for basic LRU cache.
type LRUCache[K comparable, V any] interface {
	// Add adds an entry to the cache, returns true if an eviction occurred and
	// updates the recency of usage of the key.
	Add(key K, value V) (evicted bool)

	// Get returns key's value from the cache and updates the recency of usage of the key.
	// ok specifies if the key was found or not.
	Get(key K) (value V, ok bool)

	// Contains checks if a key exists in the cache without updating the recency of usage.
	Contains(key K) (ok bool)

	// Peek returns key's value without updating the recency of usage of the key.
	// ok specifies if the key was found or not.
	Peek(key K) (value V, ok bool)

	// Remove removes an entry from the cache with the key specified.
	// ok specifies if the key was found or not.
	Remove(key K) (ok bool)

	// RemoveOldest removes the oldest entry from the cache.
	RemoveOldest() (key K, value V, ok bool)

	// GetOldest returns the oldest entry from the cache.
	GetOldest() (key K, value V, ok bool)

	// Keys returns a slice of the keys in the cache, from oldest to newest.
	Keys() []K

	// Values returns a slice of the values in the cache, from oldest to newest.
	Values() []V

	// Len returns the number of entries in the cache.
	Len() int

	// Cap returns the capacity of the cache.
	Cap() int

	// Purge clears all the cache entries.
	Purge()

	// Resize changes the cache size, returning number of evicted entries.
	Resize(size int) (evicted int)
}
