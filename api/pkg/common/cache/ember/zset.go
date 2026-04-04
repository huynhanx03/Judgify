package ember

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/datastructs/skiplist"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// zsetData represents the underlying storage for a Sorted Set.
// It bundles a skip list (for fast range/rank queries)
// and a dictionary (for fast score lookups).
// Inspired by Redis's zset implementation.
type zsetData struct {
	sl   *skiplist.SkipList
	dict map[string]float64
}

// ZAdd adds all the specified members with the specified scores to the sorted set stored at key.
// Returns the number of elements added to the sorted set (not including score updates).
func (c *Cache[K, V]) ZAdd(key K, score float64, member string) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		// Create new sorted set
		zset := &zsetData{
			sl:   skiplist.New(),
			dict: make(map[string]float64),
		}
		
		zset.sl.Insert(score, member)
		zset.dict[member] = score

		newObj := NewObject(TypeSortedSet, zset, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return 1, nil
	}

	if obj.Type != TypeSortedSet {
		return 0, cache.ErrWrongType
	}

	zset, ok := obj.Value.(*zsetData)
	if !ok {
		return 0, cache.ErrWrongType
	}

	added := 0
	oldScore, exists := zset.dict[member]
	
	if exists {
		if oldScore != score {
			zset.sl.UpdateScore(oldScore, member, score)
			zset.dict[member] = score
		}
	} else {
		zset.sl.Insert(score, member)
		zset.dict[member] = score
		added = 1
	}

	if added > 0 || exists {
		obj.Touch(c.now())
	}

	return added, nil
}

// ZRank returns the rank of member in the sorted set stored at key, with the scores ordered from low to high.
func (c *Cache[K, V]) ZRank(key K, member string) (int64, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSortedSet {
		return 0, cache.ErrWrongType
	}

	zset, ok := obj.Value.(*zsetData)
	if !ok {
		return 0, cache.ErrWrongType
	}

	score, exists := zset.dict[member]
	if !exists {
		return 0, cache.ErrKeyNotFound
	}

	// skiplist.GetRank returns a 1-based rank, Redis returns a 0-based rank.
	rank := zset.sl.GetRank(score, member)
	if rank == 0 {
		return 0, cache.ErrKeyNotFound
	}
	
	obj.Touch(c.now())
	return int64(rank - 1), nil
}

// ZScore returns the score of member in the sorted set at key.
func (c *Cache[K, V]) ZScore(key K, member string) (float64, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSortedSet {
		return 0, cache.ErrWrongType
	}

	zset, ok := obj.Value.(*zsetData)
	if !ok {
		return 0, cache.ErrWrongType
	}

	score, exists := zset.dict[member]
	if !exists {
		return 0, cache.ErrKeyNotFound
	}

	obj.Touch(c.now())
	return score, nil
}

// ZRem removes the specified members from the sorted set stored at key.
// Returns the number of members removed.
func (c *Cache[K, V]) ZRem(key K, members ...string) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSortedSet {
		return 0, cache.ErrWrongType
	}

	zset, ok := obj.Value.(*zsetData)
	if !ok {
		return 0, cache.ErrWrongType
	}

	removed := 0
	for _, m := range members {
		if score, exists := zset.dict[m]; exists {
			zset.sl.Delete(score, m)
			delete(zset.dict, m)
			removed++
		}
	}

	// Delete key entirely if sorted set becomes empty
	if zset.sl.Len() == 0 {
		c.store.del(keyStr)
	} else if removed > 0 {
		obj.Touch(c.now())
	}

	return removed, nil
}

// ZRange returns the specified range of elements in the sorted set stored at key.
func (c *Cache[K, V]) ZRange(key K, start, stop int) ([]string, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSortedSet {
		return nil, cache.ErrWrongType
	}

	zset, ok := obj.Value.(*zsetData)
	if !ok {
		return nil, cache.ErrWrongType
	}

	nodes := zset.sl.Range(start, stop)
	if nodes == nil {
		return []string{}, nil
	}

	result := make([]string, len(nodes))
	for i, node := range nodes {
		result[i] = node.Key
	}

	obj.Touch(c.now())
	return result, nil
}

// ZCard returns the sorted set cardinality (number of elements) of the sorted set stored at key.
func (c *Cache[K, V]) ZCard(key K) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSortedSet {
		return 0, cache.ErrWrongType
	}

	zset, ok := obj.Value.(*zsetData)
	if !ok {
		return 0, cache.ErrWrongType
	}

	obj.Touch(c.now())
	return zset.sl.Len(), nil
}
