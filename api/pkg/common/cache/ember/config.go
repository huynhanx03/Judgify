package ember

import (
	"time"

	"github.com/huynhanx03/judgify/pkg/timer"
)

// Option represents an optional configuration function.
type Option func(opts *Config)

// loadOptions applies all the provided options to the default config.
func loadOptions(options ...Option) Config {
	opts := defaultConfig()
	for _, opt := range options {
		opt(&opts)
	}
	return opts
}

// Config holds the configuration for the Ember cache instance.
type Config struct {
	// MaxEntries is the maximum number of keys allowed in the cache.
	// When exceeded, the EvictPolicy will be triggered. 0 means unlimited.
	MaxEntries int

	// EvictPolicy sets the algorithm for eviction when MaxEntries is reached.
	// Valid values: "lru", "random", "noeviction".
	EvictPolicy string

	// CleanupInterval is the interval at which the background active
	// expiration routine runs to clear expired keys.
	CleanupInterval time.Duration

	// NumShards is the number of internal partitions. Higher values reduce
	// lock contention. Must be a power of 2 for optimal performance (e.g. 256).
	NumShards int

	// OnEvict is an optional callback executed when a key is evicted or expired.
	OnEvict func(key string, value any)

	// Timer overrides the default CachedTimer. Useful for testing or
	// when nanosecond precision is required.
	Timer timer.Timer
}

// defaultConfig returns the recommended default configuration.
func defaultConfig() Config {
	return Config{
		MaxEntries:      DefaultMaxEntries,
		EvictPolicy:     DefaultEvictPolicy,
		CleanupInterval: DefaultCleanupInterval,
		NumShards:       DefaultShards,
		OnEvict:         nil,
	}
}

// WithMaxEntries sets the maximum number of items the cache can hold.
func WithMaxEntries(maxSettings int) Option {
	return func(opts *Config) {
		opts.MaxEntries = maxSettings
	}
}

// WithEvictPolicy sets the eviction policy ("lru", "random", "noeviction").
func WithEvictPolicy(policy string) Option {
	return func(opts *Config) {
		opts.EvictPolicy = policy
	}
}

// WithCleanupInterval sets the frequency of the background active expiry loop.
func WithCleanupInterval(interval time.Duration) Option {
	return func(opts *Config) {
		opts.CleanupInterval = interval
	}
}

// WithNumShards sets the number of internal shards to reduce lock contention.
func WithNumShards(shards int) Option {
	return func(opts *Config) {
		opts.NumShards = shards
	}
}

// WithOnEvict sets the callback function to execute upon item eviction.
func WithOnEvict(fn func(key string, value any)) Option {
	return func(opts *Config) {
		opts.OnEvict = fn
	}
}

// WithTimer overrides the default CachedTimer with a custom timer implementation.
func WithTimer(t timer.Timer) Option {
	return func(opts *Config) {
		opts.Timer = t
	}
}
