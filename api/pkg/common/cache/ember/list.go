package ember

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// LPush inserts all the specified values at the head of the list stored at key.
func (c *Cache[K, V]) LPush(key K, values ...V) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		list := make([]V, len(values))
		// LPush reverses the order of arguments if pushed one by one,
		// e.g. LPush(key, a, b, c) results in [c, b, a].
		for i, v := range values {
			list[len(values)-1-i] = v
		}

		newObj := NewObject(TypeList, list, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return len(list), nil
	}

	if obj.Type != TypeList {
		return 0, cache.ErrWrongType
	}

	list, ok := obj.Value.([]V)
	if !ok {
		return 0, cache.ErrWrongType
	}

	// Prepend elements (reversed order like Redis)
	newList := make([]V, len(values)+len(list))
	for i, v := range values {
		newList[len(values)-1-i] = v
	}
	copy(newList[len(values):], list)
	
	obj.Value = newList
	obj.Touch(c.now())
	
	return len(newList), nil
}

// RPush inserts all the specified values at the tail of the list stored at key.
func (c *Cache[K, V]) RPush(key K, values ...V) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		// RPush appends in order
		list := make([]V, len(values))
		copy(list, values)

		newObj := NewObject(TypeList, list, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return len(list), nil
	}

	if obj.Type != TypeList {
		return 0, cache.ErrWrongType
	}

	list, ok := obj.Value.([]V)
	if !ok {
		return 0, cache.ErrWrongType
	}

	list = append(list, values...)
	obj.Value = list
	obj.Touch(c.now())
	
	return len(list), nil
}

// LPop removes and returns the first element of the list stored at key.
func (c *Cache[K, V]) LPop(key K) (V, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		var zero V
		return zero, cache.ErrKeyNotFound
	}

	if obj.Type != TypeList {
		var zero V
		return zero, cache.ErrWrongType
	}

	list, ok := obj.Value.([]V)
	if !ok || len(list) == 0 {
		var zero V
		return zero, cache.ErrKeyNotFound
	}

	val := list[0]
	list = list[1:]
	
	if len(list) == 0 {
		c.store.del(keyStr)
	} else {
		obj.Value = list
		obj.Touch(c.now())
	}

	return val, nil
}

// RPop removes and returns the last element of the list stored at key.
func (c *Cache[K, V]) RPop(key K) (V, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		var zero V
		return zero, cache.ErrKeyNotFound
	}

	if obj.Type != TypeList {
		var zero V
		return zero, cache.ErrWrongType
	}

	list, ok := obj.Value.([]V)
	if !ok || len(list) == 0 {
		var zero V
		return zero, cache.ErrKeyNotFound
	}

	lastIdx := len(list) - 1
	val := list[lastIdx]
	list = list[:lastIdx]
	
	if len(list) == 0 {
		c.store.del(keyStr)
	} else {
		obj.Value = list
		obj.Touch(c.now())
	}

	return val, nil
}

// LRange returns the specified elements of the list stored at key.
func (c *Cache[K, V]) LRange(key K, start, stop int) ([]V, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeList {
		return nil, cache.ErrWrongType
	}

	list, ok := obj.Value.([]V)
	if !ok {
		return nil, cache.ErrWrongType
	}
	
	length := len(list)
	if length == 0 {
		return []V{}, nil
	}

	// Normalize negative indices
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}

	// Bounds checking
	if start < 0 {
		start = 0
	}
	if start >= length || start > stop {
		return []V{}, nil
	}

	if stop >= length {
		stop = length - 1
	}

	obj.Touch(c.now())
	
	// Return a copy to prevent external mutation affecting the cache
	count := stop - start + 1
	result := make([]V, count)
	copy(result, list[start:stop+1])
	
	return result, nil
}

// LLen returns the length of the list stored at key.
func (c *Cache[K, V]) LLen(key K) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeList {
		return 0, cache.ErrWrongType
	}

	list, ok := obj.Value.([]V)
	if !ok {
		return 0, cache.ErrWrongType
	}

	obj.Touch(c.now())
	return len(list), nil
}
