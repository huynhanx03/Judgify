package ember

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// HSet sets the specified fields to their respective values in the hash stored at key.
func (c *Cache[K, V]) HSet(key K, field string, value V) (bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		// Create new hash
		hmap := make(map[string]V)
		hmap[field] = value
		newObj := NewObject(TypeHash, hmap, 1, c.now())

		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return true, nil
	}

	if obj.Type != TypeHash {
		return false, cache.ErrWrongType
	}

	hmap, ok := obj.Value.(map[string]V)
	if !ok {
		return false, cache.ErrWrongType
	}

	_, exists := hmap[field]
	hmap[field] = value
	obj.Touch(c.now())
	return !exists, nil
}

// HGet returns the value associated with field in the hash stored at key.
func (c *Cache[K, V]) HGet(key K, field string) (V, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		var zero V
		return zero, cache.ErrKeyNotFound
	}

	if obj.Type != TypeHash {
		var zero V
		return zero, cache.ErrWrongType
	}

	hmap, ok := obj.Value.(map[string]V)
	if !ok {
		var zero V
		return zero, cache.ErrWrongType
	}

	val, exists := hmap[field]
	if !exists {
		var zero V
		return zero, cache.ErrKeyNotFound
	}

	obj.Touch(c.now())
	return val, nil
}

// HDel removes the specified fields from the hash stored at key.
func (c *Cache[K, V]) HDel(key K, fields ...string) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeHash {
		return 0, cache.ErrWrongType
	}

	hmap, ok := obj.Value.(map[string]V)
	if !ok {
		return 0, cache.ErrWrongType
	}

	deleted := 0
	for _, f := range fields {
		if _, exists := hmap[f]; exists {
			delete(hmap, f)
			deleted++
		}
	}

	// Delete key entirely if hash becomes empty
	if len(hmap) == 0 {
		c.store.del(keyStr)
	} else if deleted > 0 {
		obj.Touch(c.now())
	}

	return deleted, nil
}

// HGetAll returns all fields and values of the hash stored at key.
func (c *Cache[K, V]) HGetAll(key K) (map[string]V, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeHash {
		return nil, cache.ErrWrongType
	}

	hmap, ok := obj.Value.(map[string]V)
	if !ok {
		return nil, cache.ErrWrongType
	}

	obj.Touch(c.now())
	
	// Create a shallow copy to prevent external mutation
	result := make(map[string]V, len(hmap))
	for k, v := range hmap {
		result[k] = v
	}
	
	return result, nil
}

// HLen returns the number of fields contained in the hash stored at key.
func (c *Cache[K, V]) HLen(key K) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeHash {
		return 0, cache.ErrWrongType
	}

	hmap, ok := obj.Value.(map[string]V)
	if !ok {
		return 0, cache.ErrWrongType
	}

	obj.Touch(c.now())
	return len(hmap), nil
}

// HExists returns if field is an existing field in the hash stored at key.
func (c *Cache[K, V]) HExists(key K, field string) (bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return false, cache.ErrKeyNotFound
	}

	if obj.Type != TypeHash {
		return false, cache.ErrWrongType
	}

	hmap, ok := obj.Value.(map[string]V)
	if !ok {
		return false, cache.ErrWrongType
	}

	_, exists := hmap[field]
	obj.Touch(c.now())
	return exists, nil
}
