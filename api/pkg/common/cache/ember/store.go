package ember

import (
	"context"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/huynhanx03/judgify/pkg/datastructs/shardedmap"
	"github.com/huynhanx03/judgify/pkg/hash"
	"github.com/huynhanx03/judgify/pkg/timer"
)

// entry wraps the object with its string key.
type entry struct {
	key string
	obj *Object
}

// store is the core key-value storage engine for Ember.
// Uses string keys in sharded maps to eliminate hash collision bugs.
type store struct {
	data    *shardedmap.Map[string, *entry]
	expires *shardedmap.Map[string, int64] // key -> UnixNano expiration
	count   atomic.Int64
	timer   timer.Timer
}

// DefaultTimerStep is the default interval for the cached timer (1ms).
// Provides ~1ns reads vs ~25ns for time.Now(), with ±1ms precision.
const DefaultTimerStep = time.Millisecond

// newStore creates a new storage engine.
// Uses CachedTimer by default for high-throughput performance.
func newStore(shards int, tmr timer.Timer) *store {
	if tmr == nil {
		tmr = timer.NewCachedTimer(DefaultTimerStep)
	}
	hashFn := func(k string) uint64 { return hash.Sum64(k) }
	return &store{
		data:    shardedmap.New[string, *entry](shards, hashFn),
		expires: shardedmap.New[string, int64](shards, hashFn),
		timer:   tmr,
	}
}

// get retrieves an object, performing passive expiration check.
func (s *store) get(key string) (*Object, bool) {
	// Passive expiry check
	if exp, ok := s.expires.Get(key); ok {
		if s.timer.Now() >= exp {
			s.del(key)
			return nil, false
		}
	}

	e, ok := s.data.Get(key)
	if !ok {
		return nil, false
	}

	e.obj.Touch(s.timer.Now())
	return e.obj, true
}

// getNoTouch retrieves an object without updating access metadata.
// Used for internal operations that shouldn't affect eviction stats.
func (s *store) getNoTouch(key string) (*Object, bool) {
	if exp, ok := s.expires.Get(key); ok {
		if s.timer.Now() >= exp {
			s.del(key)
			return nil, false
		}
	}

	e, ok := s.data.Get(key)
	if !ok {
		return nil, false
	}
	return e.obj, true
}

// set inserts or updates an object.
func (s *store) set(key string, obj *Object, ttl time.Duration) {
	_, existing := s.data.Get(key)

	s.data.Set(key, &entry{key: key, obj: obj})

	if ttl > 0 {
		s.expires.Set(key, s.timer.Now()+int64(ttl))
	} else {
		s.expires.Del(key) // Clear any previous TTL
	}

	if !existing {
		s.count.Add(1)
	}
}

// del removes an object.
func (s *store) del(key string) bool {
	_, ok := s.data.Get(key)
	if !ok {
		return false
	}

	s.data.Del(key)
	s.expires.Del(key)
	s.count.Add(-1)
	return true
}

// len returns the number of keys.
func (s *store) len() int {
	return int(s.count.Load())
}

// clear removes all items.
func (s *store) clear() {
	s.data.Clear()
	s.expires.Clear()
	s.count.Store(0)
}


// -- Background Active Expiration --

// startActiveExpiry launches a goroutine to actively clean up expired keys.
// Inspired by Redis's activeExpireCycle.
func (s *store) startActiveExpiry(ctx context.Context, interval time.Duration, onExpired func(key string)) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.activeExpireCycle(onExpired)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// activeExpireCycle randomly samples keys with TTLs and deletes expired ones.
// Loops until the expired ratio drops below threshold or time limit is hit.
func (s *store) activeExpireCycle(onExpired func(key string)) {
	start := s.timer.Now()

	for {
		expiredCount := 0
		sampledCount := 0

		// Collect keys with expiration
		var sample []string
		s.expires.Do(func(k string, _ int64) {
			if len(sample) < ActiveExpireSampleSize*2 {
				sample = append(sample, k)
			}
		})

		if len(sample) == 0 {
			break
		}

		// Shuffle for randomness
		if len(sample) > ActiveExpireSampleSize {
			rand.Shuffle(len(sample), func(i, j int) { sample[i], sample[j] = sample[j], sample[i] })
			sample = sample[:ActiveExpireSampleSize]
		}

		nowNano := s.timer.Now()

		for _, key := range sample {
			sampledCount++
			if exp, ok := s.expires.Get(key); ok && nowNano >= exp {
				s.del(key)
				expiredCount++
				if onExpired != nil {
					onExpired(key)
				}
			}
		}

		if sampledCount == 0 || (float64(expiredCount)/float64(sampledCount)) < ActiveExpireThreshold {
			break
		}

		if s.timer.Now()-start > int64(ActiveExpireMaxDuration) {
			break
		}
	}
}
