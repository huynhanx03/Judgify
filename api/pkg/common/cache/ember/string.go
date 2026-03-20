package ember

import (
	"strconv"

	"github.com/huynhanx03/judgify/pkg/algorithm"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// Incr increments the number stored at key by one.
func (c *Cache[K, V]) Incr(key K) (int64, error) {
	return c.IncrBy(key, 1)
}

// Decr decrements the number stored at key by one.
func (c *Cache[K, V]) Decr(key K) (int64, error) {
	return c.IncrBy(key, -1)
}

// IncrBy increments the number stored at key by n.
func (c *Cache[K, V]) IncrBy(key K, n int64) (int64, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		newObj := NewObject(TypeString, n, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return n, nil
	}

	if obj.Type != TypeString {
		return 0, cache.ErrWrongType
	}

	currentVal, err := toInt64(obj.Value)
	if err != nil {
		return 0, cache.ErrWrongType
	}

	newVal := currentVal + n
	obj.Value = newVal
	obj.Touch(c.now())

	return newVal, nil
}

// IncrByFloat increments the float value stored at key by f.
func (c *Cache[K, V]) IncrByFloat(key K, f float64) (float64, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		newObj := NewObject(TypeString, f, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return f, nil
	}

	if obj.Type != TypeString {
		return 0, cache.ErrWrongType
	}

	currentVal, err := toFloat64(obj.Value)
	if err != nil {
		return 0, cache.ErrWrongType
	}

	newVal := currentVal + f
	obj.Value = newVal
	obj.Touch(c.now())

	return newVal, nil
}

// Append appends a string to the value stored at key.
// If key doesn't exist, creates it. Returns the length after append.
func (c *Cache[K, V]) Append(key K, value string) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		newObj := NewObject(TypeString, value, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return len(value), nil
	}

	if obj.Type != TypeString {
		return 0, cache.ErrWrongType
	}

	current := valueToString(obj.Value)
	newStr := current + value
	obj.Value = newStr
	obj.Touch(c.now())

	return len(newStr), nil
}

// LCS returns the Longest Common Subsequence between values at key1 and key2.
func (c *Cache[K, V]) LCS(key1, key2 K) (string, error) {
	key1Str := hash.ToString(key1)
	key2Str := hash.ToString(key2)

	obj1, ok1 := c.store.get(key1Str)
	obj2, ok2 := c.store.get(key2Str)

	if !ok1 || !ok2 {
		return "", nil
	}

	if obj1.Type != TypeString || obj2.Type != TypeString {
		return "", cache.ErrWrongType
	}

	s1 := valueToString(obj1.Value)
	s2 := valueToString(obj2.Value)

	return algorithm.LCS(s1, s2), nil
}

// -- Helper functions --

func toInt64(v any) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int64:
		return val, nil
	case int32:
		return int64(val), nil
	case float64:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	default:
		return 0, cache.ErrWrongType
	}
}

func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, cache.ErrWrongType
	}
}

func valueToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return ""
	}
}
