package cache

import (
	"context"
	"time"
)

// LocalCache defines the interface for in-memory local cache operations.
type LocalCache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V, cost int64) bool
	SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool
	Delete(key K)
	Clear()
	Close()
}

// CacheEngine defines the standard interface for remote caching operations.
type CacheEngine interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	InvalidatePrefix(ctx context.Context, prefix string) error
	BatchSet(ctx context.Context, values map[string]any, ttl time.Duration) error
	DeleteBulk(ctx context.Context, keys []string) error
	Incr(ctx context.Context, key string) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	GeoAdd(ctx context.Context, key string, locations ...*GeoLocation) error
	GeoRemove(ctx context.Context, key string, members ...string) error
	GeoRadius(ctx context.Context, key string, longitude, latitude, radius float64, unit string) ([]*GeoLocation, error)
	ZAdd(ctx context.Context, key string, members ...*ZMember) error
	ZRemRangeByScore(ctx context.Context, key string, min, max string) error
	ZCount(ctx context.Context, key string, min, max string) (int64, error)
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	Keys(ctx context.Context, pattern string) ([]string, error)
	Close()
}

// ZMember represents a member in a sorted set.
type ZMember struct {
	Score  float64
	Member any
}
