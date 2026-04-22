package ember

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/hash"
)

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
func (c *Cache[K, V]) Set(key K, value V) bool {
	return c.SetWithTTL(key, value, 0)
}

// SetWithTTL adds or updates a value with the specified TTL.
// Returns false if EvictPolicy is "noeviction" and cache is full.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) bool {
	keyStr := hash.ToString(key)

	if !c.evictIfNeeded() {
		return false
	}

	obj := NewObject(TypeString, value, 1, c.now())
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
func (c *Cache[K, V]) SetNX(key K, value V, ttl time.Duration) bool {
	keyStr := hash.ToString(key)
	if _, ok := c.store.getNoTouch(keyStr); ok {
		return false
	}

	if !c.evictIfNeeded() {
		return false
	}

	obj := NewObject(TypeString, value, 1, c.now())
	c.store.set(keyStr, obj, ttl)
	return true
}

// GetOrSet returns the existing value for key. If key doesn't exist,
// calls fn to compute the value, stores it, and returns it.
// The second return value indicates if the key already existed.
func (c *Cache[K, V]) GetOrSet(key K, fn func() (V, time.Duration)) (V, bool) {
	if val, ok := c.Get(key); ok {
		return val, true
	}

	value, ttl := fn()
	c.SetWithTTL(key, value, ttl)
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
func (c *Cache[K, V]) MSet(keys []K, values []V) {
	n := len(keys)
	if len(values) < n {
		n = len(values)
	}
	for i := 0; i < n; i++ {
		c.Set(keys[i], values[i])
	}
}

// -- Counter Operations --

// Incr increments the number stored at key by one.
func (c *Cache[K, V]) Incr(key K) (int64, error) {
	return c.IncrBy(key, 1)
}

// Decr decrements the number stored at key by one.
func (c *Cache[K, V]) Decr(key K) (int64, error) {
	return c.IncrBy(key, -1)
}

// IncrBy increments the number stored at key by n.
func (c *Cache[K, V]) IncrBy(key K, n int64) (int64, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		newObj := NewObject(TypeString, n, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return n, nil
	}

	currentVal, err := toInt64(obj.Value)
	if err != nil {
		return 0, cache.ErrWrongType
	}

	newVal := currentVal + n
	obj.Value = newVal
	obj.Touch(c.now())

	return newVal, nil
}

// toInt64 converts an arbitrary value to int64.
func toInt64(v any) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int64:
		return val, nil
	case int32:
		return int64(val), nil
	case float64:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	default:
		return 0, cache.ErrWrongType
	}
}

// -- TTL & Keys --

// Exists returns true if the key exists and has not expired.
func (c *Cache[K, V]) Exists(key K) bool {
	keyStr := hash.ToString(key)
	_, found := c.store.get(keyStr)
	return found
}

// TTL returns the remaining time to live of a key.
// Returns -1 if key exists but has no TTL, -2 if key doesn't exist.
func (c *Cache[K, V]) TTL(key K) (time.Duration, error) {
	keyStr := hash.ToString(key)

	if exp, ok := c.store.expires.Get(keyStr); ok {
		now := c.store.timer.Now()
		if now >= exp {
			c.store.del(keyStr)
			return -2, cache.ErrKeyNotFound
		}
		return time.Duration(exp - now), nil
	}

	if _, ok := c.store.data.Get(keyStr); ok {
		return -1, nil
	}

	return -2, cache.ErrKeyNotFound
}

// Expire sets a timeout on key.
func (c *Cache[K, V]) Expire(key K, ttl time.Duration) (bool, error) {
	keyStr := hash.ToString(key)

	if _, ok := c.store.data.Get(keyStr); !ok {
		return false, cache.ErrKeyNotFound
	}

	if ttl <= 0 {
		c.store.del(keyStr)
		return true, nil
	}

	c.store.expires.Set(keyStr, c.store.timer.Now()+int64(ttl))
	return true, nil
}

// Persist removes the timeout on key, making it persistent.
func (c *Cache[K, V]) Persist(key K) (bool, error) {
	keyStr := hash.ToString(key)

	if _, ok := c.store.data.Get(keyStr); !ok {
		return false, cache.ErrKeyNotFound
	}

	if _, ok := c.store.expires.Get(keyStr); !ok {
		return false, nil
	}

	c.store.expires.Del(keyStr)
	return true, nil
}

// Keys returns all keys in the cache.
func (c *Cache[K, V]) Keys() []K {
	var results []K
	c.store.data.Do(func(key string, _ *entry) {
		if k, ok := any(key).(K); ok {
			results = append(results, k)
		}
	})
	return results
}

// DBSize returns the total number of keys.
func (c *Cache[K, V]) DBSize() int {
	return c.store.len()
}

// Type returns the type name of the value stored at key.
func (c *Cache[K, V]) Type(key K) (string, error) {
	keyStr := hash.ToString(key)
	obj, ok := c.store.get(keyStr)
	if !ok {
		return "none", cache.ErrKeyNotFound
	}
	return obj.TypeName(), nil
}
