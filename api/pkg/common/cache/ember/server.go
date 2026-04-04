package ember

import (
	"time"

	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// Exists returns true if the key exists and has not expired.
func (c *Cache[K, V]) Exists(key K) bool {
	keyStr := hash.ToString(key)
	_, found := c.store.get(keyStr)
	return found
}

// TTL returns the remaining time to live of a key.
// Returns -1 if key exists but has no TTL, -2 if key doesn't exist.
func (c *Cache[K, V]) TTL(key K) (time.Duration, error) {
	keyStr := hash.ToString(key)

	if exp, ok := c.store.expires.Get(keyStr); ok {
		now := c.store.timer.Now()
		if now >= exp {
			c.store.del(keyStr)
			return -2, cache.ErrKeyNotFound
		}
		return time.Duration(exp - now), nil
	}

	// Check if key exists without TTL
	if _, ok := c.store.data.Get(keyStr); ok {
		return -1, nil
	}

	return -2, cache.ErrKeyNotFound
}

// Expire sets a timeout on key.
func (c *Cache[K, V]) Expire(key K, ttl time.Duration) (bool, error) {
	keyStr := hash.ToString(key)

	if _, ok := c.store.data.Get(keyStr); !ok {
		return false, cache.ErrKeyNotFound
	}

	if ttl <= 0 {
		c.store.del(keyStr)
		return true, nil
	}

	c.store.expires.Set(keyStr, c.store.timer.Now()+int64(ttl))
	return true, nil
}

// Persist removes the timeout on key, making it persistent.
func (c *Cache[K, V]) Persist(key K) (bool, error) {
	keyStr := hash.ToString(key)

	if _, ok := c.store.data.Get(keyStr); !ok {
		return false, cache.ErrKeyNotFound
	}

	if _, ok := c.store.expires.Get(keyStr); !ok {
		return false, nil // Key exists but already has no TTL
	}

	c.store.expires.Del(keyStr)
	return true, nil
}

// Keys returns all keys in the cache.
func (c *Cache[K, V]) Keys() []K {
	var results []K
	c.store.data.Do(func(key string, _ *entry) {
		if k, ok := any(key).(K); ok {
			results = append(results, k)
		}
	})
	return results
}

// DBSize returns the total number of keys.
func (c *Cache[K, V]) DBSize() int {
	return c.store.len()
}

// Type returns the type name of the value stored at key.
func (c *Cache[K, V]) Type(key K) (string, error) {
	keyStr := hash.ToString(key)
	obj, ok := c.store.get(keyStr)
	if !ok {
		return "none", cache.ErrKeyNotFound
	}
	return obj.TypeName(), nil
}
