package ristretto

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/ristretto"

	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// atomicStats holds thread-safe local statistics.
type atomicStats struct {
	expiredKeys atomic.Int64
	evictions   atomic.Int64
}

// ttlEntry stores the expiry time for a key.
type ttlEntry struct {
	expiry time.Time
	ttl    time.Duration
}

// defaultCost is used for all ristretto Set/SetWithTTL calls.
const defaultCost int64 = 1

// Cache wraps *ristretto.Cache and implements cache.LocalCache[K, V].
type Cache[K any, V any] struct {
	inner   *ristretto.Cache
	stats   atomicStats
	counters sync.Map // key string -> *int64 (for Incr/Decr/IncrBy)
	ttlMap   sync.Map // key string -> ttlEntry
	keysMap  sync.Map // key string -> struct{}
}

// New creates a new Ristretto-backed Cache[K, V].
// It applies the given options on top of DefaultConfig and then
// initialises the underlying ristretto cache.
func New[K any, V any](opts ...Option) (*Cache[K, V], error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	inner, err := ristretto.NewCache(&cfg)
	if err != nil {
		return nil, err
	}

	return &Cache[K, V]{
		inner: inner,
	}, nil
}

// hashKey converts a generic key to the uint64 that ristretto expects.
func hashKey[K any](key K) uint64 {
	h, _ := hash.KeyToHash(key)
	return h
}

// keyStr converts a generic key to its string representation for use in
// internal sync.Map lookups (counters, ttlMap, keysMap).
func keyStr[K any](key K) string {
	return hash.ToString(key)
}

// ---------------------------------------------------------------------------
// Core KV
// ---------------------------------------------------------------------------

// Get retrieves a value from the cache.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	h := hashKey(key)
	val, ok := c.inner.Get(h)
	if !ok {
		var zero V
		return zero, false
	}

	typed, ok := val.(V)
	if !ok {
		var zero V
		return zero, false
	}
	return typed, true
}

// Set adds or updates a value without TTL.
func (c *Cache[K, V]) Set(key K, value V) bool {
	h := hashKey(key)
	ks := keyStr(key)

	ok := c.inner.Set(h, value, defaultCost)
	c.inner.Wait()

	if ok {
		c.keysMap.Store(ks, struct{}{})
	}
	return ok
}

// SetWithTTL adds or updates a value with a TTL.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) bool {
	h := hashKey(key)
	ks := keyStr(key)

	ok := c.inner.SetWithTTL(h, value, defaultCost, ttl)
	c.inner.Wait()

	if ok {
		c.keysMap.Store(ks, struct{}{})
		if ttl > 0 {
			c.ttlMap.Store(ks, ttlEntry{
				expiry: time.Now().Add(ttl),
				ttl:    ttl,
			})
		} else {
			// No TTL — make sure any prior TTL tracking is removed.
			c.ttlMap.Delete(ks)
		}
	}
	return ok
}

// Delete removes a value from the cache.
func (c *Cache[K, V]) Delete(key K) {
	h := hashKey(key)
	ks := keyStr(key)
	c.inner.Del(h)
	c.ttlMap.Delete(ks)
	c.keysMap.Delete(ks)
	c.counters.Delete(ks)
}

// Clear removes all items and resets internal tracking maps.
func (c *Cache[K, V]) Clear() {
	c.inner.Clear()
	c.ttlMap = sync.Map{}
	c.keysMap = sync.Map{}
	c.counters = sync.Map{}
	c.stats.expiredKeys.Store(0)
	c.stats.evictions.Store(0)
}

// Close gracefully shuts down the cache.
func (c *Cache[K, V]) Close() {
	c.inner.Close()
}

// ---------------------------------------------------------------------------
// Extended KV
// ---------------------------------------------------------------------------

// SetNX sets the value only if the key does not already exist.
// Returns true if the key was set.
func (c *Cache[K, V]) SetNX(key K, value V, ttl time.Duration) bool {
	h := hashKey(key)
	if _, ok := c.inner.Get(h); ok {
		return false
	}
	return c.SetWithTTL(key, value, ttl)
}

// GetOrSet returns the existing value for key. If the key doesn't exist,
// it calls fn to compute the value, stores it, and returns it.
// The second return value indicates if the key already existed.
func (c *Cache[K, V]) GetOrSet(key K, fn func() (V, time.Duration)) (V, bool) {
	if val, ok := c.Get(key); ok {
		return val, true
	}

	value, ttl := fn()
	c.SetWithTTL(key, value, ttl)
	return value, false
}

// GetDel retrieves a value and then deletes it.
func (c *Cache[K, V]) GetDel(key K) (V, bool) {
	val, ok := c.Get(key)
	if ok {
		c.Delete(key)
	}
	return val, ok
}

// MGet returns values for multiple keys. Missing keys produce zero values.
func (c *Cache[K, V]) MGet(keys ...K) []V {
	results := make([]V, len(keys))
	for i, key := range keys {
		if val, ok := c.Get(key); ok {
			results[i] = val
		}
	}
	return results
}

// MSet sets multiple key-value pairs.
func (c *Cache[K, V]) MSet(keys []K, values []V) {
	n := len(keys)
	if len(values) < n {
		n = len(values)
	}
	for i := 0; i < n; i++ {
		c.Set(keys[i], values[i])
	}
}

// ---------------------------------------------------------------------------
// Counter
// ---------------------------------------------------------------------------

// Incr increments the counter at key by one.
func (c *Cache[K, V]) Incr(key K) (int64, error) {
	return c.IncrBy(key, 1)
}

// Decr decrements the counter at key by one.
func (c *Cache[K, V]) Decr(key K) (int64, error) {
	return c.IncrBy(key, -1)
}

// IncrBy increments the counter at key by n.
// If the key doesn't exist, it is initialised to n.
func (c *Cache[K, V]) IncrBy(key K, n int64) (int64, error) {
	ks := keyStr(key)

	for {
		existing, loaded := c.counters.Load(ks)
		if !loaded {
			// Try to insert a new counter starting at n.
			newVal := n
			actual, loaded := c.counters.LoadOrStore(ks, &newVal)
			if !loaded {
				// We won the race — value is n.
				return n, nil
			}
			// Someone else stored first; fall through to CAS loop.
			existing = actual
		}

		ptr := existing.(*int64)
		old := atomic.LoadInt64(ptr)
		newVal := old + n
		if atomic.CompareAndSwapInt64(ptr, old, newVal) {
			return newVal, nil
		}
		// CAS failed — another goroutine changed it; retry.
	}
}

// ---------------------------------------------------------------------------
// TTL & Keys
// ---------------------------------------------------------------------------

// TTL returns the remaining time to live of a key.
// Returns -1 if key exists but has no TTL, -2 if key doesn't exist.
func (c *Cache[K, V]) TTL(key K) (time.Duration, error) {
	h := hashKey(key)
	ks := keyStr(key)

	// Check existence first.
	if _, ok := c.inner.Get(h); !ok {
		return -2, cache.ErrKeyNotFound
	}

	entry, ok := c.ttlMap.Load(ks)
	if !ok {
		return -1, nil // exists but no TTL
	}

	remaining := time.Until(entry.(ttlEntry).expiry)
	if remaining <= 0 {
		return -2, cache.ErrKeyNotFound
	}
	return remaining, nil
}

// Expire sets a timeout on key. Returns false if the key doesn't exist.
func (c *Cache[K, V]) Expire(key K, ttl time.Duration) (bool, error) {
	h := hashKey(key)
	ks := keyStr(key)

	if _, ok := c.inner.Get(h); !ok {
		return false, cache.ErrKeyNotFound
	}

	if ttl <= 0 {
		c.Delete(key)
		return true, nil
	}

	c.ttlMap.Store(ks, ttlEntry{
		expiry: time.Now().Add(ttl),
		ttl:    ttl,
	})
	return true, nil
}

// Persist removes the TTL on key, making it persistent.
func (c *Cache[K, V]) Persist(key K) (bool, error) {
	h := hashKey(key)
	ks := keyStr(key)

	if _, ok := c.inner.Get(h); !ok {
		return false, cache.ErrKeyNotFound
	}

	_, existed := c.ttlMap.LoadAndDelete(ks)
	return existed, nil
}

// Exists returns true if the key is present in the cache.
func (c *Cache[K, V]) Exists(key K) bool {
	h := hashKey(key)
	_, ok := c.inner.Get(h)
	return ok
}

// Keys returns all tracked keys.
func (c *Cache[K, V]) Keys() []K {
	var results []K
	c.keysMap.Range(func(k, _ any) bool {
		if typed, ok := k.(K); ok {
			results = append(results, typed)
		}
		return true
	})
	return results
}

// DBSize returns the total number of tracked keys.
func (c *Cache[K, V]) DBSize() int {
	if m := c.inner.Metrics; m != nil {
		return int(m.KeysAdded() - m.KeysEvicted())
	}

	// Fallback: count keysMap entries.
	count := 0
	c.keysMap.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// Type returns the type name of the value stored at key.
func (c *Cache[K, V]) Type(key K) (string, error) {
	if !c.Exists(key) {
		return "none", cache.ErrKeyNotFound
	}
	return "string", nil
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

// Stats returns a snapshot of cache statistics.
func (c *Cache[K, V]) Stats() cache.Stats {
	s := cache.Stats{
		ExpiredKeys: c.stats.expiredKeys.Load(),
		Evictions:   c.stats.evictions.Load(),
		KeyCount:    int64(c.DBSize()),
	}

	if m := c.inner.Metrics; m != nil {
		s.Hits = int64(m.Hits())
		s.Misses = int64(m.Misses())
		s.Evictions += int64(m.KeysEvicted())
		s.CostUsed = int64(m.CostAdded() - m.CostEvicted())
	}

	return s
}
