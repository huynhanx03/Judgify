package ember

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/datastructs/hyperloglog"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// PFAdd adds all the element arguments to the HyperLogLog data structure stored at the specified key.
func (c *Cache[K, V]) PFAdd(key K, items ...string) (bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		hll := hyperloglog.New()
		for _, item := range items {
			_, h := hash.KeyToHash(item)
			hll.Add(h)
		}
		newObj := NewObject(TypeHyperLogLog, hll, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return true, nil
	}

	if obj.Type != TypeHyperLogLog {
		return false, cache.ErrWrongType
	}

	hll, ok := obj.Value.(*hyperloglog.HLL)
	if !ok {
		return false, cache.ErrWrongType
	}

	updated := false
	for _, item := range items {
		_, h := hash.KeyToHash(item)
		if hll.Add(h) {
			updated = true
		}
	}

	if updated {
		obj.Touch(c.now())
		return true, nil
	}

	return false, nil
}

// PFCount returns the approximated cardinality of the set(s) observed by the HyperLogLog at key(s).
func (c *Cache[K, V]) PFCount(keys ...K) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	if len(keys) == 1 {
		keyStr := hash.ToString(keys[0])
		obj, ok := c.store.get(keyStr)
		if !ok {
			return 0, nil
		}
		if obj.Type != TypeHyperLogLog {
			return 0, cache.ErrWrongType
		}
		hll, ok := obj.Value.(*hyperloglog.HLL)
		if !ok {
			return 0, cache.ErrWrongType
		}
		obj.Touch(c.now())
		return hll.Count(), nil
	}

	// Multiple keys: merge them temporarily
	merged := hyperloglog.New()
	for _, key := range keys {
		keyStr := hash.ToString(key)
		obj, ok := c.store.get(keyStr)
		if !ok {
			continue
		}
		if obj.Type != TypeHyperLogLog {
			return 0, cache.ErrWrongType
		}
		hll, ok := obj.Value.(*hyperloglog.HLL)
		if !ok {
			return 0, cache.ErrWrongType
		}
		merged.Merge(hll)
		obj.Touch(c.now())
	}

	return merged.Count(), nil
}

// PFMerge merges multiple HyperLogLog values into an unique value that will approximate the cardinality of the union of the observed Sets of the source structures.
func (c *Cache[K, V]) PFMerge(dest K, sources ...K) error {
	merged := hyperloglog.New()
	
	for _, src := range sources {
		keyStr := hash.ToString(src)
		obj, ok := c.store.get(keyStr)
		if !ok {
			continue
		}
		if obj.Type != TypeHyperLogLog {
			return cache.ErrWrongType
		}
		hll, ok := obj.Value.(*hyperloglog.HLL)
		if !ok {
			return cache.ErrWrongType
		}
		merged.Merge(hll)
		obj.Touch(c.now())
	}

	destStr := hash.ToString(dest)
	obj, ok := c.store.get(destStr)
	if !ok {
		newObj := NewObject(TypeHyperLogLog, merged, 1, c.now())
		c.evictIfNeeded()
		c.store.set(destStr, newObj, 0)
		return nil
	}

	if obj.Type != TypeHyperLogLog {
		return cache.ErrWrongType
	}

	hll, ok := obj.Value.(*hyperloglog.HLL)
	if !ok {
		return cache.ErrWrongType
	}

	hll.Merge(merged)
	obj.Touch(c.now())
	return nil
}
