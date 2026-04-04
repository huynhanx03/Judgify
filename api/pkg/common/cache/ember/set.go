package ember

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// SAdd adds the specified members to the set stored at key.
// Returns the number of elements that were added to the set.
func (c *Cache[K, V]) SAdd(key K, members ...V) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		// Go doesn't have a stable generic map key for 'any' types that aren't comparable
		// But interface types V usually hash fine if they are primitive or pointer.
		// We use map[any]struct{} as set storage since V could be any.
		smap := make(map[any]struct{})
		for _, m := range members {
			smap[m] = struct{}{}
		}

		newObj := NewObject(TypeSet, smap, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return len(smap), nil
	}

	if obj.Type != TypeSet {
		return 0, cache.ErrWrongType
	}

	smap, ok := obj.Value.(map[any]struct{})
	if !ok {
		return 0, cache.ErrWrongType
	}

	added := 0
	for _, m := range members {
		if _, exists := smap[m]; !exists {
			smap[m] = struct{}{}
			added++
		}
	}

	if added > 0 {
		obj.Touch(c.now())
	}

	return added, nil
}

// SRem removes the specified members from the set stored at key.
// Returns the number of members that were removed from the set.
func (c *Cache[K, V]) SRem(key K, members ...V) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSet {
		return 0, cache.ErrWrongType
	}

	smap, ok := obj.Value.(map[any]struct{})
	if !ok {
		return 0, cache.ErrWrongType
	}

	removed := 0
	for _, m := range members {
		if _, exists := smap[m]; exists {
			delete(smap, m)
			removed++
		}
	}

	// Delete key entirely if set becomes empty
	if len(smap) == 0 {
		c.store.del(keyStr)
	} else if removed > 0 {
		obj.Touch(c.now())
	}

	return removed, nil
}

// SMembers returns all the members of the set value stored at key.
func (c *Cache[K, V]) SMembers(key K) ([]V, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSet {
		return nil, cache.ErrWrongType
	}

	smap, ok := obj.Value.(map[any]struct{})
	if !ok {
		return nil, cache.ErrWrongType
	}

	obj.Touch(c.now())
	
	result := make([]V, 0, len(smap))
	for m := range smap {
		if val, ok := m.(V); ok {
			result = append(result, val)
		}
	}

	return result, nil
}

// SIsMember returns if member is a member of the set stored at key.
func (c *Cache[K, V]) SIsMember(key K, member V) (bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return false, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSet {
		return false, cache.ErrWrongType
	}

	smap, ok := obj.Value.(map[any]struct{})
	if !ok {
		return false, cache.ErrWrongType
	}

	_, exists := smap[member]
	obj.Touch(c.now())
	return exists, nil
}

// SCard returns the set cardinality (number of elements) of the set stored at key.
func (c *Cache[K, V]) SCard(key K) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSet {
		return 0, cache.ErrWrongType
	}

	smap, ok := obj.Value.(map[any]struct{})
	if !ok {
		return 0, cache.ErrWrongType
	}

	obj.Touch(c.now())
	return len(smap), nil
}

// SMIsMember returns whether multiple members are members of the set stored at key.
func (c *Cache[K, V]) SMIsMember(key K, members ...V) ([]bool, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		result := make([]bool, len(members))
		return result, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSet {
		return nil, cache.ErrWrongType
	}

	smap, ok := obj.Value.(map[any]struct{})
	if !ok {
		return nil, cache.ErrWrongType
	}

	result := make([]bool, len(members))
	for i, m := range members {
		_, exists := smap[m]
		result[i] = exists
	}

	obj.Touch(c.now())
	return result, nil
}

// SRandMember returns one or multiple random elements from the set value stored at key.
func (c *Cache[K, V]) SRandMember(key K, count int) ([]V, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSet {
		return nil, cache.ErrWrongType
	}

	smap, ok := obj.Value.(map[any]struct{})
	if !ok {
		return nil, cache.ErrWrongType
	}

	if count <= 0 {
		return []V{}, nil
	}

	obj.Touch(c.now())

	result := make([]V, 0, count)
	i := 0
	for m := range smap {
		if val, ok := m.(V); ok {
			result = append(result, val)
			i++
			if i == count {
				break
			}
		}
	}

	return result, nil
}

// SPop removes and returns one or multiple random elements from the set value stored at key.
func (c *Cache[K, V]) SPop(key K, count int) ([]V, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSet {
		return nil, cache.ErrWrongType
	}

	smap, ok := obj.Value.(map[any]struct{})
	if !ok {
		return nil, cache.ErrWrongType
	}

	if count <= 0 {
		return []V{}, nil
	}

	result := make([]V, 0, count)
	i := 0
	for m := range smap {
		if val, ok := m.(V); ok {
			result = append(result, val)
			delete(smap, m)
			i++
			if i == count {
				break
			}
		}
	}

	if len(smap) == 0 {
		c.store.del(keyStr)
	} else if i > 0 {
		obj.Touch(c.now())
	}

	return result, nil
}

