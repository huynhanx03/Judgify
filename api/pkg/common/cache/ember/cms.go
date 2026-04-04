package ember

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/datastructs/sketch"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// CMSInitByDim initializes a Count-Min Sketch.
// Note: depth is ignored here as the underlying sketch uses a fixed depth internally (cmDepth).
func (c *Cache[K, V]) CMSInitByDim(key K, width, depth uint) error {
	keyStr := hash.ToString(key)

	if _, ok := c.store.get(keyStr); ok {
		return cache.ErrNotSupported // Custom error like 'key already exists'
	}

	cms := sketch.New(int64(width))

	obj := NewObject(TypeCountMinSketch, cms, 1, c.now())
	c.evictIfNeeded()
	c.store.set(keyStr, obj, 0)
	return nil
}

// CMSIncrBy increments the count of an item in the sketch by the specified count.
// Returns the estimated count of the item after the increment.
func (c *Cache[K, V]) CMSIncrBy(key K, item string, count uint) (uint, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeCountMinSketch {
		return 0, cache.ErrWrongType
	}

	cms, ok := obj.Value.(*sketch.Sketch)
	if !ok {
		return 0, cache.ErrWrongType
	}

	_, itemHash := hash.KeyToHash(item)
	for i := uint(0); i < count; i++ {
		cms.Increment(itemHash)
	}

	obj.Touch(c.now())

	// Return estimated count after increment
	est := cms.Estimate(itemHash)
	return uint(est), nil
}

// CMSQuery returns the estimated count of an item in the sketch.
func (c *Cache[K, V]) CMSQuery(key K, item string) (uint, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeCountMinSketch {
		return 0, cache.ErrWrongType
	}

	cms, ok := obj.Value.(*sketch.Sketch)
	if !ok {
		return 0, cache.ErrWrongType
	}

	_, itemHash := hash.KeyToHash(item)
	est := cms.Estimate(itemHash)

	obj.Touch(c.now())
	return uint(est), nil
}
