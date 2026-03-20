package ember

import (
	"math/rand"
	"sort"
	"sync"
)

// EvictPoolSize is the number of best eviction candidates to retain across rounds.
// Matches Redis's EVPOOL_SIZE (16).
const EvictPoolSize = 16

// evictIfNeeded checks if the store has exceeded MaxEntries and evicts accordingly.
// Returns true if a Set can proceed, false if NoEviction policy prevents eviction.
func (c *Cache[K, V]) evictIfNeeded() bool {
	if c.cfg.MaxEntries <= 0 {
		return true // Unlimited
	}

	for c.store.len() >= c.cfg.MaxEntries {
		switch c.cfg.EvictPolicy {
		case EvictLRU:
			if !c.evictFromPool(true) {
				return false
			}
		case EvictLFU:
			if !c.evictFromPool(false) {
				return false
			}
		case EvictRandom:
			if !c.evictRandom() {
				return false
			}
		case EvictNoEviction:
			return false
		default:
			if !c.evictFromPool(true) {
				return false
			}
		}
	}
	return true
}

// evictCandidate represents an eviction candidate in the pool.
type evictCandidate struct {
	key        string
	lastAccess int64
	lfuCounter uint8
	value      any
}

// evictPool maintains the best eviction candidates across multiple rounds.
// Inspired by Redis's EvictionPoolLRU (server.h).
type evictPool struct {
	mu         sync.Mutex
	candidates []evictCandidate
}

// poolInstance is a package-level pool shared per cache instance.
// We store it in the Cache struct instead.

// getOrCreatePool returns the eviction pool, creating it lazily.
func (c *Cache[K, V]) getOrCreatePool() *evictPool {
	c.poolOnce.Do(func() {
		c.pool = &evictPool{
			candidates: make([]evictCandidate, 0, EvictPoolSize),
		}
	})
	return c.pool
}

// populatePool samples random keys and inserts the best candidates into the pool.
func (c *Cache[K, V]) populatePool(pool *evictPool, useLRU bool) {
	samples := c.sampleEntries(EvictSampleSize)
	if len(samples) == 0 {
		return
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	for _, s := range samples {
		cand := evictCandidate{
			key:        s.key,
			lastAccess: s.lastAccess,
			lfuCounter: s.lfuCounter,
			value:      s.value,
		}

		// Skip if already in pool
		duplicate := false
		for _, existing := range pool.candidates {
			if existing.key == cand.key {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}

		if len(pool.candidates) < EvictPoolSize {
			pool.candidates = append(pool.candidates, cand)
		} else {
			// Replace the least-evictable candidate if new one is better
			worstIdx := c.findLeastEvictable(pool.candidates, useLRU)
			if c.isBetterVictim(cand, pool.candidates[worstIdx], useLRU) {
				pool.candidates[worstIdx] = cand
			}
		}
	}

	// Sort: best victim (most evictable) at the end for easy pop
	sort.Slice(pool.candidates, func(i, j int) bool {
		if useLRU {
			// Ascending by lastAccess: oldest (smallest) at end? No.
			// We want oldest at the END so we can pop. So sort descending.
			return pool.candidates[i].lastAccess > pool.candidates[j].lastAccess
		}
		// LFU: lowest counter = best victim, put at end. Sort descending.
		return pool.candidates[i].lfuCounter > pool.candidates[j].lfuCounter
	})
}

// evictFromPool evicts the best candidate from the eviction pool.
// If pool is empty, populates it first by sampling random keys.
func (c *Cache[K, V]) evictFromPool(useLRU bool) bool {
	pool := c.getOrCreatePool()

	// Always populate with fresh samples to improve pool quality
	c.populatePool(pool, useLRU)

	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Try candidates from best (end of slice) to worst
	for len(pool.candidates) > 0 {
		// Pop best victim (last element)
		last := len(pool.candidates) - 1
		victim := pool.candidates[last]
		pool.candidates = pool.candidates[:last]

		// Verify key still exists (may have been deleted/expired since pooled)
		if _, ok := c.store.getNoTouch(victim.key); !ok {
			continue // Stale candidate, try next
		}

		c.store.del(victim.key)
		c.stats.evictions.Add(1)

		if c.cfg.OnEvict != nil {
			c.cfg.OnEvict(victim.key, victim.value)
		}
		return true
	}

	return false
}

// isBetterVictim returns true if a is a better eviction candidate than b.
func (c *Cache[K, V]) isBetterVictim(a, b evictCandidate, useLRU bool) bool {
	if useLRU {
		return a.lastAccess < b.lastAccess // Older = better victim
	}
	return a.lfuCounter < b.lfuCounter // Less frequent = better victim
}

// findLeastEvictable finds the index of the candidate that is LEAST worth evicting
// (most recently used for LRU, most frequently used for LFU).
func (c *Cache[K, V]) findLeastEvictable(candidates []evictCandidate, useLRU bool) int {
	best := 0
	for i := 1; i < len(candidates); i++ {
		if useLRU {
			if candidates[i].lastAccess > candidates[best].lastAccess {
				best = i // More recent = less evictable
			}
		} else {
			if candidates[i].lfuCounter > candidates[best].lfuCounter {
				best = i // More frequent = less evictable
			}
		}
	}
	return best
}

// evictSample represents a sampled entry for eviction consideration.
type evictSample struct {
	key        string
	lastAccess int64
	lfuCounter uint8
	value      any
}

// sampleEntries collects a random sample of entries using reservoir sampling.
func (c *Cache[K, V]) sampleEntries(n int) []evictSample {
	var pool []evictSample
	idx := 0

	c.store.data.Do(func(_ string, e *entry) {
		idx++
		s := evictSample{
			key:        e.key,
			lastAccess: e.obj.LastAccess(),
			lfuCounter: e.obj.LFUCounter(),
			value:      e.obj.Value,
		}
		if len(pool) < n {
			pool = append(pool, s)
		} else {
			j := rand.Intn(idx)
			if j < n {
				pool[j] = s
			}
		}
	})

	return pool
}

// evictRandom evicts a random entry using Go's pseudo-random map iteration.
func (c *Cache[K, V]) evictRandom() bool {
	var victimKey string
	var victimVal any
	found := false

	c.store.data.Do(func(_ string, e *entry) {
		if !found {
			victimKey = e.key
			victimVal = e.obj.Value
			found = true
		}
	})

	if !found {
		return false
	}

	c.store.del(victimKey)
	c.stats.evictions.Add(1)

	if c.cfg.OnEvict != nil {
		c.cfg.OnEvict(victimKey, victimVal)
	}
	return true
}
