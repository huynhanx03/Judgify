package ember

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/hash"
	"github.com/huynhanx03/judgify/pkg/datastructs/cuckoo"
)

// CFReserve creates a new Cuckoo Filter at key.
func (c *Cache[K, V]) CFReserve(key K, capacity uint) error {
	keyStr := hash.ToString(key)

	if _, ok := c.store.get(keyStr); ok {
		return cache.ErrNotSupported // Should be key already exists
	}

	filter := cuckoo.New(capacity)
	obj := NewObject(TypeCuckooFilter, filter, 1, c.now())
	c.evictIfNeeded()
	c.store.set(keyStr, obj, 0)
	return nil
}

// CFAdd adds an item to the Cuckoo Filter at key.
func (c *Cache[K, V]) CFAdd(key K, item string) (bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return false, cache.ErrKeyNotFound
	}

	if obj.Type != TypeCuckooFilter {
		return false, cache.ErrWrongType
	}

	filter, ok := obj.Value.(*cuckoo.Filter)
	if !ok {
		return false, cache.ErrWrongType
	}

	err := filter.Add(item)
	if err == nil {
		obj.Touch(c.now())
		return true, nil
	}
	return false, err
}

// CFExists checks if an item is in the Cuckoo Filter at key.
func (c *Cache[K, V]) CFExists(key K, item string) (bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return false, cache.ErrKeyNotFound
	}

	if obj.Type != TypeCuckooFilter {
		return false, cache.ErrWrongType
	}

	filter, ok := obj.Value.(*cuckoo.Filter)
	if !ok {
		return false, cache.ErrWrongType
	}

	exists := filter.Contains(item)
	obj.Touch(c.now())
	return exists, nil
}

// CFDel deletes an item from the Cuckoo Filter at key.
func (c *Cache[K, V]) CFDel(key K, item string) (bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return false, cache.ErrKeyNotFound
	}

	if obj.Type != TypeCuckooFilter {
		return false, cache.ErrWrongType
	}

	filter, ok := obj.Value.(*cuckoo.Filter)
	if !ok {
		return false, cache.ErrWrongType
	}

	deleted := filter.Delete(item)
	if deleted {
		obj.Touch(c.now())
	}
	return deleted, nil
}

// CFCount returns the number of items in the Cuckoo Filter at key.
func (c *Cache[K, V]) CFCount(key K) (uint, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeCuckooFilter {
		return 0, cache.ErrWrongType
	}

	filter, ok := obj.Value.(*cuckoo.Filter)
	if !ok {
		return 0, cache.ErrWrongType
	}

	obj.Touch(c.now())
	return filter.Count(), nil
}
