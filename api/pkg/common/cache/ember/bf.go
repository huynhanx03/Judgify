package ember

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/datastructs/bloom"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// BFReserve creates a new Bloom Filter.
func (c *Cache[K, V]) BFReserve(key K, errorRate float64, capacity uint) error {
	keyStr := hash.ToString(key)

	if _, ok := c.store.get(keyStr); ok {
		return cache.ErrNotSupported // Or custom error like 'key already exists'
	}

	filter, err := bloom.New(uint64(capacity), errorRate)
	if err != nil {
		return err
	}

	obj := NewObject(TypeBloomFilter, filter, 1, c.now())
	c.evictIfNeeded()
	c.store.set(keyStr, obj, 0)
	return nil
}

// BFAdd adds an item to the Bloom Filter.
// Returns 1 if the item was newly added (i.e. definitely not in the filter before).
// Returns 0 if the item may have already been in the filter.
func (c *Cache[K, V]) BFAdd(key K, item string) (bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return false, cache.ErrKeyNotFound
	}

	if obj.Type != TypeBloomFilter {
		return false, cache.ErrWrongType
	}

	filter, ok := obj.Value.(*bloom.Bloom)
	if !ok {
		return false, cache.ErrWrongType
	}

	_, itemHash := hash.KeyToHash(item)
	wasPresent := filter.AddIfNotHas(itemHash)

	obj.Touch(c.now())

	if wasPresent {
		return false, nil
	}
	return true, nil
}

// BFExists checks if an item is in the Bloom Filter.
func (c *Cache[K, V]) BFExists(key K, item string) (bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return false, cache.ErrKeyNotFound
	}

	if obj.Type != TypeBloomFilter {
		return false, cache.ErrWrongType
	}

	filter, ok := obj.Value.(*bloom.Bloom)
	if !ok {
		return false, cache.ErrWrongType
	}

	_, itemHash := hash.KeyToHash(item)
	exists := filter.Has(itemHash)

	obj.Touch(c.now())
	return exists, nil
}

// BFMAdd adds multiple items to the Bloom Filter.
func (c *Cache[K, V]) BFMAdd(key K, items ...string) ([]bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeBloomFilter {
		return nil, cache.ErrWrongType
	}

	filter, ok := obj.Value.(*bloom.Bloom)
	if !ok {
		return nil, cache.ErrWrongType
	}

	results := make([]bool, len(items))
	for i, item := range items {
		_, itemHash := hash.KeyToHash(item)
		wasPresent := filter.AddIfNotHas(itemHash)
		results[i] = !wasPresent
	}

	obj.Touch(c.now())
	return results, nil
}

// BFMExists checks if multiple items are in the Bloom Filter.
func (c *Cache[K, V]) BFMExists(key K, items ...string) ([]bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeBloomFilter {
		return nil, cache.ErrWrongType
	}

	filter, ok := obj.Value.(*bloom.Bloom)
	if !ok {
		return nil, cache.ErrWrongType
	}

	results := make([]bool, len(items))
	for i, item := range items {
		_, itemHash := hash.KeyToHash(item)
		results[i] = filter.Has(itemHash)
	}

	obj.Touch(c.now())
	return results, nil
}

// BFInfo returns information about the Bloom Filter.
func (c *Cache[K, V]) BFInfo(key K) (map[string]any, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeBloomFilter {
		return nil, cache.ErrWrongType
	}

	filter, ok := obj.Value.(*bloom.Bloom)
	if !ok {
		return nil, cache.ErrWrongType
	}

	obj.Touch(c.now())

	return map[string]any{
		"size":    filter.TotalSize(),
		"filters": filter.K(),
	}, nil
}

