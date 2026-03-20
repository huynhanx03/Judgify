package ember

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/datastructs/morris"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// MorrisIncr increments the Morris Counter at key.
func (c *Cache[K, V]) MorrisIncr(key K) (bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		m := morris.New()
		updated := m.Increment()
		newObj := NewObject(TypeMorrisCounter, m, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return updated, nil
	}

	if obj.Type != TypeMorrisCounter {
		return false, cache.ErrWrongType
	}

	m, ok := obj.Value.(*morris.Morris)
	if !ok {
		return false, cache.ErrWrongType
	}

	updated := m.Increment()
	if updated {
		obj.Touch(c.now())
	}

	return updated, nil
}

// MorrisCount returns the estimated count of the Morris Counter at key.
func (c *Cache[K, V]) MorrisCount(key K) (uint64, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, nil
	}

	if obj.Type != TypeMorrisCounter {
		return 0, cache.ErrWrongType
	}

	m, ok := obj.Value.(*morris.Morris)
	if !ok {
		return 0, cache.ErrWrongType
	}

	obj.Touch(c.now())
	return m.Count(), nil
}
