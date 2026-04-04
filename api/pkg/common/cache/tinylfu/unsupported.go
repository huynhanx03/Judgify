package tinylfu

import (
	"time"

	"github.com/huynhanx03/judgify/pkg/common/cache"
)

// unsupportedOps provides stub implementations for rich data type methods
// that TinyLFU does not support. All methods return ErrNotSupported.
type unsupportedOps[K any, V any] struct{}

func (u unsupportedOps[K, V]) Incr(key K) (int64, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) Decr(key K) (int64, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) IncrBy(key K, n int64) (int64, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) LCS(key1, key2 K) (string, error) { return "", cache.ErrNotSupported }

func (u unsupportedOps[K, V]) TTL(key K) (time.Duration, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) Expire(key K, ttl time.Duration) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) Persist(key K) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) Exists(key K) bool { return false }
func (u unsupportedOps[K, V]) Keys() []K { return nil }
func (u unsupportedOps[K, V]) DBSize() int { return 0 }
func (u unsupportedOps[K, V]) Type(key K) (string, error) { return "", cache.ErrNotSupported }

func (u unsupportedOps[K, V]) HSet(key K, field string, value V) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) HGet(key K, field string) (V, error) { var z V; return z, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) HDel(key K, fields ...string) (int, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) HGetAll(key K) (map[string]V, error) { return nil, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) HLen(key K) (int, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) HExists(key K, field string) (bool, error) { return false, cache.ErrNotSupported }

func (u unsupportedOps[K, V]) LPush(key K, values ...V) (int, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) RPush(key K, values ...V) (int, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) LPop(key K) (V, error) { var z V; return z, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) RPop(key K) (V, error) { var z V; return z, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) LRange(key K, start, stop int) ([]V, error) { return nil, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) LLen(key K) (int, error) { return 0, cache.ErrNotSupported }

func (u unsupportedOps[K, V]) SAdd(key K, members ...V) (int, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) SRem(key K, members ...V) (int, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) SMembers(key K) ([]V, error) { return nil, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) SIsMember(key K, member V) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) SMIsMember(key K, members ...V) ([]bool, error) { return nil, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) SRandMember(key K, count int) ([]V, error) { return nil, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) SPop(key K, count int) ([]V, error) { return nil, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) SCard(key K) (int, error) { return 0, cache.ErrNotSupported }

func (u unsupportedOps[K, V]) ZAdd(key K, score float64, member string) (int, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) ZRank(key K, member string) (int64, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) ZScore(key K, member string) (float64, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) ZRem(key K, members ...string) (int, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) ZRange(key K, start, stop int) ([]string, error) { return nil, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) ZCard(key K) (int, error) { return 0, cache.ErrNotSupported }

func (u unsupportedOps[K, V]) BFReserve(key K, errorRate float64, capacity uint) error { return cache.ErrNotSupported }
func (u unsupportedOps[K, V]) BFAdd(key K, item string) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) BFMAdd(key K, items ...string) ([]bool, error) { return nil, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) BFExists(key K, item string) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) BFMExists(key K, items ...string) ([]bool, error) { return nil, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) BFInfo(key K) (map[string]any, error) { return nil, cache.ErrNotSupported }

func (u unsupportedOps[K, V]) CMSInitByDim(key K, width, depth uint) error { return cache.ErrNotSupported }
func (u unsupportedOps[K, V]) CMSIncrBy(key K, item string, count uint) (uint, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) CMSQuery(key K, item string) (uint, error) { return 0, cache.ErrNotSupported }

func (u unsupportedOps[K, V]) PFAdd(key K, items ...string) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) PFCount(keys ...K) (int64, error) { return 0, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) PFMerge(dest K, sources ...K) error { return cache.ErrNotSupported }

func (u unsupportedOps[K, V]) MorrisIncr(key K) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) MorrisCount(key K) (uint64, error) { return 0, cache.ErrNotSupported }

func (u unsupportedOps[K, V]) CFReserve(key K, capacity uint) error { return cache.ErrNotSupported }
func (u unsupportedOps[K, V]) CFAdd(key K, item string) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) CFExists(key K, item string) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) CFDel(key K, item string) (bool, error) { return false, cache.ErrNotSupported }
func (u unsupportedOps[K, V]) CFCount(key K) (uint, error) { return 0, cache.ErrNotSupported }

func (u unsupportedOps[K, V]) SetBit(key K, offset uint64, value int) (int, error) {
	return 0, cache.ErrNotSupported
}
func (u unsupportedOps[K, V]) GetBit(key K, offset uint64) (int, error) {
	return 0, cache.ErrNotSupported
}
func (u unsupportedOps[K, V]) BitCount(key K, start, end int) (int, error) {
	return 0, cache.ErrNotSupported
}

func (u unsupportedOps[K, V]) GeoAdd(key K, lon, lat float64, member string) (bool, error) {
	return false, cache.ErrNotSupported
}
func (u unsupportedOps[K, V]) GeoPos(key K, members ...string) ([]*cache.Point, error) {
	return nil, cache.ErrNotSupported
}
func (u unsupportedOps[K, V]) GeoDist(key K, m1, m2 string, unit string) (float64, error) {
	return 0, cache.ErrNotSupported
}
func (u unsupportedOps[K, V]) GeoSearch(key K, lon, lat, radius float64, unit string) ([]cache.GeoSearchResult, error) {
	return nil, cache.ErrNotSupported
}

func (u unsupportedOps[K, V]) TopKReserve(key K, k uint, width, depth uint, decay float64) error {
	return cache.ErrNotSupported
}
func (u unsupportedOps[K, V]) TopKAdd(key K, items ...string) ([]string, error) {
	return nil, cache.ErrNotSupported
}
func (u unsupportedOps[K, V]) TopKQuery(key K, items ...string) ([]bool, error) {
	return nil, cache.ErrNotSupported
}
func (u unsupportedOps[K, V]) TopKList(key K) ([]string, error) {
	return nil, cache.ErrNotSupported
}
