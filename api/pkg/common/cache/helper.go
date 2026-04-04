package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
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

// ── Local cache helpers (LocalCache) ──
// All helpers use cost=0 (Ember doesn't rely on variable costs).

// LocalGet retrieves a typed value from local cache.
func LocalGet[T any](c LocalCache[string, any], key string) (T, bool) {
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

// LocalSet stores a value in local cache.
func LocalSet[T any](c LocalCache[string, any], key string, value T) bool {
	return c.Set(key, any(value), 0)
}

// LocalSetWithTTL stores a value in local cache with TTL.
func LocalSetWithTTL[T any](c LocalCache[string, any], key string, value T, ttl time.Duration) bool {
	return c.SetWithTTL(key, any(value), 0, ttl)
}

// LocalDel removes a key from local cache.
func LocalDel(c LocalCache[string, any], key string) {
	c.Delete(key)
}

// LocalUpdate overwrites a key only if it already exists.
func LocalUpdate[T any](c LocalCache[string, any], key string, value T) {
	if _, found := LocalGet[T](c, key); found {
		LocalSet(c, key, value)
	}
}

// LocalFetch returns cached value on hit; on miss calls fn, stores result, and returns it.
func LocalFetch[T any](c LocalCache[string, any], key string, fn func() (T, error)) (T, error) {
	if val, ok := LocalGet[T](c, key); ok {
		return val, nil
	}
	val, err := fn()
	if err != nil {
		var zero T
		return zero, err
	}
	LocalSet(c, key, val)
	return val, nil
}

// LocalFetchWithTTL is like LocalFetch but stores with a TTL.
func LocalFetchWithTTL[T any](c LocalCache[string, any], key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	if val, ok := LocalGet[T](c, key); ok {
		return val, nil
	}
	val, err := fn()
	if err != nil {
		var zero T
		return zero, err
	}
	LocalSetWithTTL(c, key, val, ttl)
	return val, nil
}
