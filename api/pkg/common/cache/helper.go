package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

// ── Remote cache helpers (CacheEngine) ──

// HandleHitCache handles cache hit
func HandleHitCache(ctx context.Context, model any, c CacheEngine, key string) error {
	byteData, exists, err := c.Get(ctx, key)
	if exists && err == nil {
		err = json.Unmarshal(byteData, model)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshal cache")
		}
		return nil
	}
	return errors.Wrap(err, "miss cache")
}

// HandleSetCache handles cache set
func HandleSetCache(ctx context.Context, model any, c CacheEngine, key string, ttl time.Duration) error {
	return c.Set(ctx, key, model, ttl)
}

// HandleUpdateCache handles cache update
func HandleUpdateCache(ctx context.Context, model any, c CacheEngine, key string, ttl time.Duration) {
	if _, exists, err := c.Get(ctx, key); err == nil && exists {
		_ = HandleSetCache(ctx, model, c, key, ttl)
	}
}

// HandleDeleteCache handles cache delete
func HandleDeleteCache(ctx context.Context, c CacheEngine, key string) error {
	return c.Delete(ctx, key)
}

// ── Local cache helpers ──
// All helpers use cost=0 (Ember doesn't rely on variable costs).

// Get retrieves a typed value from local cache.
func Get[T any](c LocalCache[string, any], key string) (T, bool) {
	var zero T
	val, found := c.Get(key)
	if !found {
		return zero, false
	}
	if typed, ok := val.(T); ok {
		return typed, true
	}
	return zero, false
}

// Set stores a value in local cache without TTL.
func Set[T any](c LocalCache[string, any], key string, value T) bool {
	return c.Set(key, any(value))
}

// SetWithTTL stores a value with TTL.
func SetWithTTL[T any](c LocalCache[string, any], key string, value T, ttl time.Duration) bool {
	return c.SetWithTTL(key, any(value), ttl)
}

// Del removes a key from local cache.
func Del(c LocalCache[string, any], key string) {
	c.Delete(key)
}

// Fetch retrieves a cached value. On miss, calls fn, caches the result with TTL, and returns it.
// Uses singleflight to deduplicate concurrent requests for the same key.
// Cache hit → return immediately, 0 DB queries.
// Cache miss → singleflight ensures only 1 goroutine calls fn, others wait.
func Fetch[T any](
	c   LocalCache[string, any],
	sf  *singleflight.Group,
	key string,
	ttl time.Duration,
	fn  func() (T, error),
) (T, error) {
	v, err, _ := sf.Do(key, func() (any, error) {
		// Cache hit
		if val, ok := Get[T](c, key); ok {
			return val, nil
		}
		// Cache miss → query
		result, err := fn()
		if err != nil {
			var zero T
			return zero, err
		}
		SetWithTTL(c, key, result, ttl)
		return result, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return v.(T), nil
}
