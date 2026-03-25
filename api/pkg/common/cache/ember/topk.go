package ember

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/datastructs/topk"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// TopKReserve initializes a Top-K structure with specified parameters.
func (c *Cache[K, V]) TopKReserve(key K, k uint, width, depth uint, decay float64) error {
	keyStr := hash.ToString(key)

	if width == 0 {
		width = 1024 // Default width
	}
	if depth == 0 {
		depth = 7 // Default depth
	}
	if decay == 0 {
		decay = 0.9 // Default decay
	}

	hk := topk.New(uint32(k), uint32(width), uint32(depth), decay)
	obj := NewObject(TypeTopK, hk, 1, c.now())

	c.evictIfNeeded()
	c.store.set(keyStr, obj, 0)
	return nil
}

// TopKAdd adds items to the Top-K structure. 
// Returns a list of items that were dropped from the Top-K if they were previously there (simplified Redis behavior).
// In this implementation, we usually just return nil or the current list if needed, 
// but we'll stick to a simple signature.
func (c *Cache[K, V]) TopKAdd(key K, items ...string) ([]string, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		// Auto-reserve with defaults if not exists
		c.TopKReserve(key, 10, 1024, 7, 0.9)
		obj, _ = c.store.get(keyStr)
	}

	if obj.Type != TypeTopK {
		return nil, cache.ErrWrongType
	}

	hk, ok := obj.Value.(*topk.HeavyKeepers)
	if !ok {
		return nil, cache.ErrWrongType
	}

	for _, item := range items {
		hk.Add(item)
	}

	obj.Touch(c.now())
	return nil, nil // Redis returns items kicked out, we return nil for simplicity
}

// TopKQuery checks if items are in the Top-K list.
func (c *Cache[K, V]) TopKQuery(key K, items ...string) ([]bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeTopK {
		return nil, cache.ErrWrongType
	}

	hk, ok := obj.Value.(*topk.HeavyKeepers)
	if !ok {
		return nil, cache.ErrWrongType
	}

	res := make([]bool, len(items))
	for i, item := range items {
		res[i] = hk.Query(item)
	}

	obj.Touch(c.now())
	return res, nil
}

// TopKList returns all items currently in the Top-K.
func (c *Cache[K, V]) TopKList(key K) ([]string, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeTopK {
		return nil, cache.ErrWrongType
	}

	hk, ok := obj.Value.(*topk.HeavyKeepers)
	if !ok {
		return nil, cache.ErrWrongType
	}

	obj.Touch(c.now())
	return hk.List(), nil
}
