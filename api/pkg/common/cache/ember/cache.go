package ember

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// Ensure Cache implements the full LocalCache interface.
var _ cache.LocalCache[string, any] = (*Cache[string, any])(nil)

// atomicStats holds thread-safe cache statistics.
type atomicStats struct {
	hits        atomic.Int64
	misses      atomic.Int64
	evictions   atomic.Int64
	expiredKeys atomic.Int64
}

// Cache is the main Ember local cache instance.
type Cache[K any, V any] struct {
	store      *store
	cfg        Config
	stats      atomicStats
	cancelFunc context.CancelFunc
	pool       *evictPool
	poolOnce   sync.Once
}

// New creates a new Ember Cache instance.
func New[K any, V any](opts ...Option) *Cache[K, V] {
	cfg := loadOptions(opts...)
	ctx, cancel := context.WithCancel(context.Background())
	c := &Cache[K, V]{
		store:      newStore(cfg.NumShards, cfg.Timer),
		cfg:        cfg,
		cancelFunc: cancel,
	}

	c.store.startActiveExpiry(ctx, cfg.CleanupInterval, func(key string) {
		c.stats.expiredKeys.Add(1)
		if c.cfg.OnEvict != nil {
			c.cfg.OnEvict(key, nil)
		}
	})

	return c
}

// now returns the current time in unix nanoseconds from the store's timer.
func (c *Cache[K, V]) now() int64 {
	return c.store.timer.Now()
}

// Get retrieves a value from the cache.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	keyStr := hash.ToString(key)
	obj, ok := c.store.get(keyStr)
	if !ok {
		c.stats.misses.Add(1)
		var zero V
		return zero, false
	}

	c.stats.hits.Add(1)

	if val, ok := obj.Value.(V); ok {
		return val, true
	}

	var zero V
	return zero, false
}

// Set adds or updates a value without TTL.
func (c *Cache[K, V]) Set(key K, value V, cost int64) bool {
	return c.SetWithTTL(key, value, cost, 0)
}

// SetWithTTL adds or updates a value with the specified TTL.
// Returns false if EvictPolicy is "noeviction" and cache is full.
func (c *Cache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	keyStr := hash.ToString(key)

	if !c.evictIfNeeded() {
		return false
	}

	obj := NewObject(TypeString, value, cost, c.now())
	c.store.set(keyStr, obj, ttl)
	return true
}

// Delete removes a value from the cache.
func (c *Cache[K, V]) Delete(key K) {
	keyStr := hash.ToString(key)
	c.store.del(keyStr)
}

// Clear removes all items.
func (c *Cache[K, V]) Clear() {
	c.store.clear()
}

// Close gracefully shuts down the cache, stopping background goroutines.
func (c *Cache[K, V]) Close() {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	if c.store.timer != nil {
		c.store.timer.Stop()
	}
	c.Clear()
}

// Stats returns a snapshot of cache statistics.
func (c *Cache[K, V]) Stats() cache.Stats {
	return cache.Stats{
		Hits:        c.stats.hits.Load(),
		Misses:      c.stats.misses.Load(),
		Evictions:   c.stats.evictions.Load(),
		ExpiredKeys: c.stats.expiredKeys.Load(),
		KeyCount:    int64(c.store.len()),
		CostUsed:    0,
	}
}

// -- Extended Operations --

// SetNX sets the value only if the key does not already exist.
// Returns true if the key was set.
func (c *Cache[K, V]) SetNX(key K, value V, cost int64, ttl time.Duration) bool {
	keyStr := hash.ToString(key)
	if _, ok := c.store.getNoTouch(keyStr); ok {
		return false
	}

	if !c.evictIfNeeded() {
		return false
	}

	obj := NewObject(TypeString, value, cost, c.now())
	c.store.set(keyStr, obj, ttl)
	return true
}

// GetOrSet returns the existing value for key. If key doesn't exist,
// calls fn to compute the value, stores it, and returns it.
// The second return value indicates if the key already existed.
func (c *Cache[K, V]) GetOrSet(key K, fn func() (V, int64, time.Duration)) (V, bool) {
	if val, ok := c.Get(key); ok {
		return val, true
	}

	value, cost, ttl := fn()
	c.SetWithTTL(key, value, cost, ttl)
	return value, false
}

// GetDel gets the value of a key and deletes it.
func (c *Cache[K, V]) GetDel(key K) (V, bool) {
	keyStr := hash.ToString(key)
	obj, ok := c.store.getNoTouch(keyStr)
	if !ok {
		c.stats.misses.Add(1)
		var zero V
		return zero, false
	}

	c.stats.hits.Add(1)
	c.store.del(keyStr)

	if val, ok := obj.Value.(V); ok {
		return val, true
	}

	var zero V
	return zero, false
}

// MGet returns values for multiple keys. Missing keys have zero values.
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
func (c *Cache[K, V]) MSet(keys []K, values []V, cost int64) {
	n := len(keys)
	if len(values) < n {
		n = len(values)
	}
	for i := 0; i < n; i++ {
		c.Set(keys[i], values[i], cost)
	}
}
