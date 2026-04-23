package cache

import (
	"context"
	"time"
)

// Stats holds hit/miss and other cache metrics.
type Stats struct {
	Hits        int64
	Misses      int64
	Evictions   int64
	ExpiredKeys int64
	KeyCount    int64
	CostUsed    int64
}

// LocalCache defines the interface for in-memory local cache operations.
type LocalCache[K any, V any] interface {
	// Core KV
	Get(key K) (V, bool)
	Set(key K, value V) bool
	SetWithTTL(key K, value V, ttl time.Duration) bool
	Delete(key K)
	Clear()
	Close()

	// Extended KV
	SetNX(key K, value V, ttl time.Duration) bool
	GetOrSet(key K, fn func() (V, time.Duration)) (V, bool)
	GetDel(key K) (V, bool)
	MGet(keys ...K) []V
	MSet(keys []K, values []V)

	// Counter
	Incr(key K) (int64, error)
	Decr(key K) (int64, error)
	IncrBy(key K, n int64) (int64, error)

	// TTL & Keys
	TTL(key K) (time.Duration, error)
	Expire(key K, ttl time.Duration) (bool, error)
	Persist(key K) (bool, error)
	Exists(key K) bool
	Keys() []K
	DBSize() int
	Type(key K) (string, error)

	// Stats
	Stats() Stats
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
