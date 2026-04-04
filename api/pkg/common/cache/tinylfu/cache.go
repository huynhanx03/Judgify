package tinylfu

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/datastructs/shardedmap"
	"github.com/huynhanx03/judgify/pkg/hash"
	"github.com/huynhanx03/judgify/pkg/mq/batcher"
	"github.com/huynhanx03/judgify/pkg/timer"
)

var _ cache.LocalCache[string, any] = (*Cache[string, any])(nil)

// Config holds cache configuration.
type Config struct {
	MaxCost     int64
	NumCounters int64
	BufferSize  int

	// Timer is the time provider. If nil, time.Now is used (wrapped).
	Timer timer.Timer
}

// Cache is a high-performance, thread-safe LFU cache with TTL support.
type Cache[K any, V any] struct {
	unsupportedOps[K, V]

	store      *shardedmap.Map[uint64, *storeItem[V]]
	controller *Controller[V]
	getBuf     *batcher.StripedBatcher[uint64]
	setBuf     chan *Item[V]
	stop       chan struct{}
	itemPool   *sync.Pool
	timer      timer.Timer
	cost       func(V) int64
	onEvict    func(*Item[V])
	isClosed   atomic.Bool

	hits       atomic.Int64
	misses     atomic.Int64
}

// storeItem uses int64 expiration for minimal memory.
type storeItem[V any] struct {
	conflict   uint64
	value      V
	expiration int64
}

// IsExpired returns true if the item has expired.
func (i *storeItem[V]) IsExpired(now int64) bool {
	return i.expiration > 0 && now >= i.expiration
}

// New creates a new TinyLFU cache.
func New[K any, V any](cfg Config) *Cache[K, V] {
	if cfg.MaxCost <= 0 {
		cfg.MaxCost = 1 << 20
	}
	if cfg.NumCounters <= 0 {
		cfg.NumCounters = cfg.MaxCost / 100
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 64
	}

	c := &Cache[K, V]{
		store:      shardedmap.New[uint64, *storeItem[V]](256, func(k uint64) uint64 { return k }),
		controller: NewController[V](cfg.MaxCost, cfg.NumCounters),
		setBuf:     make(chan *Item[V], 32*1024),
		stop:       make(chan struct{}),
		timer:      cfg.Timer,
		itemPool: &sync.Pool{
			New: func() any {
				return &Item[V]{}
			},
		},
	}

	if c.timer == nil {
		c.timer = timer.SystemTimer{}
	}

	c.getBuf = batcher.New[uint64](c.controller, batcher.Config{StripeSize: cfg.BufferSize})

	go c.processItems()

	return c
}

// Get retrieves a value from the cache.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	if c.isClosed.Load() {
		var zero V
		return zero, false
	}

	keyHash, conflict := hash.KeyToHash(key)
	item, ok := c.store.Get(keyHash)
	if !ok {
		c.misses.Add(1)
		var zero V
		return zero, false
	}

	if conflict != 0 && item.conflict != conflict {
		c.misses.Add(1)
		var zero V
		return zero, false
	}

	// Lazy expiration: convert cached nanos to unix seconds
	if item.IsExpired(c.timer.Now() / int64(time.Second)) {
		c.misses.Add(1)
		var zero V
		return zero, false
	}

	c.hits.Add(1)
	c.getBuf.Push(keyHash)
	return item.value, true
}

// Set adds or updates a value in the cache without TTL.
func (c *Cache[K, V]) Set(key K, value V, cost int64) bool {
	return c.SetWithTTL(key, value, cost, 0)
}

// SetWithTTL adds or updates a value in the cache with TTL.
func (c *Cache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	if c.isClosed.Load() {
		return false
	}

	keyHash, conflict := hash.KeyToHash(key)

	if cost == 0 && c.cost != nil {
		cost = c.cost(value)
	}
	if cost == 0 {
		cost = 1
	}

	var expiration int64
	if ttl > 0 {
		expiration = (c.timer.Now() + int64(ttl)) / int64(time.Second)
	}

	// Get Item from pool
	item := c.itemPool.Get().(*Item[V])
	item.Key = keyHash
	item.Conflict = conflict
	item.Value = value
	item.Cost = cost
	item.Expiration = expiration

	select {
	case c.setBuf <- item:
		return true
	default:
		// Return to pool on failure
		c.itemPool.Put(item)
		return false
	}
}

// Del removes a value from the cache.
func (c *Cache[K, V]) Delete(key K) {
	if c.isClosed.Load() {
		return
	}

	keyHash, _ := hash.KeyToHash(key)
	c.store.Del(keyHash)
	c.controller.Del(keyHash)
}

// Clear removes all items from the cache.
func (c *Cache[K, V]) Clear() {
	c.store.Clear()
	c.controller.Clear()
}

// Close shuts down the cache.
func (c *Cache[K, V]) Close() {
	if c.isClosed.Swap(true) {
		return
	}
	close(c.stop)
	if t, ok := c.timer.(*timer.CachedTimer); ok {
		t.Stop()
	}
}

// Stats returns basic statistics about the cache.
func (c *Cache[K, V]) Stats() cache.Stats {
	return cache.Stats{
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
	}
}


// SetCostFunc sets a function to calculate item cost.
func (c *Cache[K, V]) SetCostFunc(fn func(V) int64) {
	c.cost = fn
}

// SetOnEvict sets a callback for when items are evicted.
func (c *Cache[K, V]) SetOnEvict(fn func(*Item[V])) {
	c.onEvict = fn
}

func (c *Cache[K, V]) processItems() {
	for {
		select {
		case item := <-c.setBuf:
			victims, added := c.controller.Add(item.Key, item.Cost)
			if added {
				c.store.Set(item.Key, &storeItem[V]{
					conflict:   item.Conflict,
					value:      item.Value,
					expiration: item.Expiration,
				})
			}
			// Return item to pool after processing
			c.itemPool.Put(item)

			for _, victim := range victims {
				if evicted, ok := c.store.Get(victim.Key); ok {
					c.store.Del(victim.Key)
					if c.onEvict != nil {
						c.onEvict(&Item[V]{
							Key:        victim.Key,
							Value:      evicted.value,
							Cost:       victim.Cost,
							Expiration: evicted.expiration,
						})
					}
				}
			}
		case <-c.stop:
			return
		}
	}
}
