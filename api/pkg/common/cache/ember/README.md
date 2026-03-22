# Ember

A Redis-like, generic in-memory cache for Go with sharded storage, multiple eviction policies, TTL support, and 13+ data types including probabilistic data structures.

## Features

- **Generic API** — `Cache[K, V]` with type-safe operations
- **Eviction policies** — LRU, LFU (logarithmic counter with decay), Random, NoEviction
- **Sharded storage** — 256 shards by default for reduced lock contention
- **TTL support** — per-key expiration with passive + active cleanup
- **13+ data types** — String, Hash, List, Set, Sorted Set, Bloom Filter, Cuckoo Filter, Count-Min Sketch, HyperLogLog, Morris Counter, Top-K, Geo, Bitmap
- **Eviction pool** — Redis-inspired candidate pool for better eviction accuracy
- **CachedTimer** — ~1ns reads vs ~25ns for `time.Now()` (±1ms precision)
- **Atomic statistics** — hits, misses, evictions, expired keys
- **Evict callback** — optional hook on key eviction/expiration

## Quick Start

```go
import "github.com/huynhanx03/judgify/pkg/common/cache/ember"

// Create cache with defaults (100K entries, LRU, 256 shards)
c := ember.New[string, any]()

// With custom config
c := ember.New[string, any](
    ember.WithMaxEntries(50000),
    ember.WithEvictPolicy(ember.EvictLFU),
    ember.WithCleanupInterval(200 * time.Millisecond),
    ember.WithOnEvict(func(key string, val any) {
        log.Printf("evicted: %s", key)
    }),
)

// Basic operations
c.Set("user:1", user, 5*time.Minute)  // set with TTL
val, ok := c.Get("user:1")
c.Delete("user:1")
c.Exists("user:1")

// TTL management
c.Expire("key", 10*time.Second)
ttl := c.TTL("key")
c.Persist("key")  // remove TTL

// Hash
c.HSet("user:1", "name", "Alice")
name, _ := c.HGet("user:1", "name")
all := c.HGetAll("user:1")

// List
c.LPush("queue", "task1", "task2")
val, _ := c.RPop("queue")

// Set
c.SAdd("tags", "go", "cache")
members := c.SMembers("tags")

// Sorted Set
c.ZAdd("leaderboard", "player1", 100.0)
top := c.ZRange("leaderboard", 0, 9)

// Bloom Filter
c.BFReserve("emails", 0.01, 10000)
c.BFAdd("emails", "test@example.com")
exists := c.BFExists("emails", "test@example.com")

// HyperLogLog
c.PFAdd("visitors", "user1", "user2")
count := c.PFCount("visitors")

// Geo
c.GeoAdd("stores", 13.361389, 38.115556, "Palermo")
nearby := c.GeoRadius("stores", 15.0, 37.0, 200, "km")

// Incr/Decr (atomic)
c.Incr("counter")
c.IncrByFloat("balance", 9.99)
```

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| `MaxEntries` | 100,000 | Max keys (0 = unlimited) |
| `EvictPolicy` | `lru` | `lru`, `lfu`, `random`, `noeviction` |
| `CleanupInterval` | 100ms | Background active expiry frequency |
| `NumShards` | 256 | Internal partitions (power of 2) |
| `OnEvict` | nil | Callback on eviction/expiration |
| `Timer` | CachedTimer | Custom timer implementation |

## Eviction Policies

### LRU (Least Recently Used)
Tracks last access timestamp. Evicts the key with the oldest access time from a random sample of 5 keys.

### LFU (Least Frequently Used)
8-bit logarithmic counter (0–255) with time-based decay:
- Initial value: 5 (prevents immediate eviction)
- Increment probability: `1 / ((counter - 5) * 10 + 1)`
- Decay: counter decremented every 60s of inactivity

### Random
Uniform random selection from sampled keys.

### NoEviction
Rejects new writes when `MaxEntries` is reached.

## Supported Data Types

| Type | Operations |
|------|------------|
| **String** | `Incr`, `Decr`, `IncrBy`, `IncrByFloat` |
| **Hash** | `HSet`, `HGet`, `HDel`, `HGetAll`, `HKeys`, `HVals`, `HLen`, `HExists` |
| **List** | `LPush`, `RPush`, `LPop`, `RPop`, `LRange`, `LLen` |
| **Set** | `SAdd`, `SRem`, `SMember`, `SMembers`, `SCard`, `SInter` |
| **Sorted Set** | `ZAdd`, `ZRem`, `ZRange`, `ZRank`, `ZScore`, `ZCard` |
| **Bloom Filter** | `BFReserve`, `BFAdd`, `BFExists` |
| **Cuckoo Filter** | `CFReserve`, `CFAdd`, `CFExists` |
| **Count-Min Sketch** | `CMSInitByDim`, `CMSIncrBy`, `CMSQuery` |
| **HyperLogLog** | `PFAdd`, `PFCount`, `PFMerge` |
| **Morris Counter** | Approximate counting |
| **Top-K** | `TopKAdd`, `TopKIncrBy`, `TopKQuery` |
| **Geo** | `GeoAdd`, `GeoRadius` |
| **Bitmap** | `SetBit`, `GetBit`, `BitCount` |

## Active Expiration

Background goroutine runs every `CleanupInterval` (default 100ms):
1. Samples 20 keys with TTLs
2. Deletes expired keys
3. If >25% were expired, runs another cycle
4. Each cycle capped at 25ms to avoid blocking

## Testing

```bash
go test ./pkg/common/cache/ember/...          # unit tests
go test ./pkg/common/cache/ember/... -bench .  # benchmarks
```
