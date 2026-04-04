package ember

import "time"

const (
	// -- System Defaults --
	DefaultShards         = 256
	DefaultMaxEntries     = 100000
	DefaultCleanupInterval = 100 * time.Millisecond
	DefaultEvictPolicy    = EvictLRU

	// -- Eviction Policies --
	EvictLRU        = "lru"
	EvictLFU        = "lfu"
	EvictRandom     = "random"
	EvictNoEviction = "noeviction"

	// -- Eviction Constants --
	// EvictSampleSize is the number of keys to sample for approximated eviction algorithms.
	EvictSampleSize = 5
	// ActiveExpireSampleSize is the number of keys with TTLs to sample during background cleanup.
	ActiveExpireSampleSize = 20
	// ActiveExpireMaxDuration is the maximum time a background cleanup cycle can take.
	ActiveExpireMaxDuration = 25 * time.Millisecond
	// ActiveExpireThreshold is the percentage of expired keys (0.25 = 25%) found in a sample 
	// that triggers another cleanup cycle.
	ActiveExpireThreshold = 0.25

	// -- LFU Constants --
	// LFUInitVal is the initial counter value for new objects to prevent immediate eviction.
	LFUInitVal = 5
	// LFULogFactor is the logarithmic factor for incrementing the LFU counter.
	// Higher means counter increases more slowly.
	LFULogFactor = 10
	// LFUDecayInterval is the time interval after which the LFU counter is decremented.
	LFUDecayInterval = 60 * time.Second
)
