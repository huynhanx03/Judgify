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
// It includes both basic K-V operations and rich data type operations.
type LocalCache[K any, V any] interface {
	// ── Core K-V ──
	Get(key K) (V, bool)
	Set(key K, value V, cost int64) bool
	SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool
	Delete(key K)
	Clear()
	Close()

	// ── String ──
	Incr(key K) (int64, error)
	Decr(key K) (int64, error)
	IncrBy(key K, n int64) (int64, error)
	LCS(key1, key2 K) (string, error)

	// ── TTL & Keys ──
	TTL(key K) (time.Duration, error)
	Expire(key K, ttl time.Duration) (bool, error)
	Persist(key K) (bool, error)
	Exists(key K) bool
	Keys() []K
	DBSize() int
	Type(key K) (string, error)

	// ── Hash ──
	HSet(key K, field string, value V) (bool, error)
	HGet(key K, field string) (V, error)
	HDel(key K, fields ...string) (int, error)
	HGetAll(key K) (map[string]V, error)
	HLen(key K) (int, error)
	HExists(key K, field string) (bool, error)

	// ── List ──
	LPush(key K, values ...V) (int, error)
	RPush(key K, values ...V) (int, error)
	LPop(key K) (V, error)
	RPop(key K) (V, error)
	LRange(key K, start, stop int) ([]V, error)
	LLen(key K) (int, error)

	// ── Set ──
	SAdd(key K, members ...V) (int, error)
	SRem(key K, members ...V) (int, error)
	SMembers(key K) ([]V, error)
	SIsMember(key K, member V) (bool, error)
	SMIsMember(key K, members ...V) ([]bool, error)
	SRandMember(key K, count int) ([]V, error)
	SPop(key K, count int) ([]V, error)
	SCard(key K) (int, error)

	// ── Sorted Set ──
	ZAdd(key K, score float64, member string) (int, error)
	ZRank(key K, member string) (int64, error)
	ZScore(key K, member string) (float64, error)
	ZRem(key K, members ...string) (int, error)
	ZRange(key K, start, stop int) ([]string, error)
	ZCard(key K) (int, error)

	// ── Bloom Filter ──
	BFReserve(key K, errorRate float64, capacity uint) error
	BFAdd(key K, item string) (bool, error)
	BFMAdd(key K, items ...string) ([]bool, error)
	BFExists(key K, item string) (bool, error)
	BFMExists(key K, items ...string) ([]bool, error)
	BFInfo(key K) (map[string]any, error)

	// ── Count-Min Sketch ──
	CMSInitByDim(key K, width, depth uint) error
	CMSIncrBy(key K, item string, count uint) (uint, error)
	CMSQuery(key K, item string) (uint, error)

	// ── HyperLogLog ──
	PFAdd(key K, items ...string) (bool, error)
	PFCount(keys ...K) (int64, error)
	PFMerge(dest K, sources ...K) error

	// ── Morris Counter ──
	MorrisIncr(key K) (bool, error)
	MorrisCount(key K) (uint64, error)

	// ── Cuckoo Filter ──
	CFReserve(key K, capacity uint) error
	CFAdd(key K, item string) (bool, error)
	CFExists(key K, item string) (bool, error)
	CFDel(key K, item string) (bool, error)
	CFCount(key K) (uint, error)

	// ── Bitmap ──
	SetBit(key K, offset uint64, value int) (int, error)
	GetBit(key K, offset uint64) (int, error)
	BitCount(key K, start, end int) (int, error)

	// ── Geo ──
	GeoAdd(key K, lon, lat float64, member string) (bool, error)
	GeoPos(key K, members ...string) ([]*Point, error)
	GeoDist(key K, m1, m2 string, unit string) (float64, error)
	GeoSearch(key K, lon, lat, radius float64, unit string) ([]GeoSearchResult, error)

	// ── Top-K ──
	TopKReserve(key K, k uint, width, depth uint, decay float64) error
	TopKAdd(key K, items ...string) ([]string, error)
	TopKQuery(key K, items ...string) ([]bool, error)
	TopKList(key K) ([]string, error)

	// ── Stats ──
	Stats() Stats
}

// Point represents a geographical location (repeated here for interface or imported).
// Since Geo library already has Point, we can use it or define a simple one if we don't want to export data package.
// Actually, geo package is in pkg/datastructs/geo, let's just use it if possible or define a local alias.
// For now, I'll use pointers to the geo.Point if I can import it, or just define a simple struct here.
// Let's define a simple one for the interface to avoid importing datastructs into common/cache.
type Point struct {
	Longitude float64
	Latitude  float64
}

// GeoSearchResult represents a match from a geospatial search.
type GeoSearchResult struct {
	Member   string
	Distance float64
	Point    *Point
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
