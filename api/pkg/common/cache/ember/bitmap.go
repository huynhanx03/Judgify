package ember

import (
	"math/bits"

	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// SetBit sets or clears the bit at offset in the string value stored at key.
// Returns the original bit value at offset.
func (c *Cache[K, V]) SetBit(key K, offset uint64, value int) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		// Create new empty bitset
		byteOffset := offset / 8
		bitOffset := offset % 8
		data := make([]byte, byteOffset+1)
		if value != 0 {
			data[byteOffset] |= (1 << (7 - bitOffset))
		}
		newObj := NewObject(TypeString, data, 1, c.now())
		c.evictIfNeeded()
		c.store.set(keyStr, newObj, 0)
		return 0, nil
	}

	if obj.Type != TypeString {
		return 0, cache.ErrWrongType
	}

	var data []byte
	switch v := obj.Value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		// Attempt to convert other types to string and then bytes if possible, 
		// but usually it's better to fail if it's not a byte/string compatible type.
		return 0, cache.ErrWrongType
	}

	byteOffset := offset / 8
	bitOffset := offset % 8

	// Grow data if needed
	if uint64(len(data)) <= byteOffset {
		newData := make([]byte, byteOffset+1)
		copy(newData, data)
		data = newData
		obj.Value = data // Update reference
	}

	byteVal := data[byteOffset]
	oldBit := (byteVal >> (7 - bitOffset)) & 1

	if value != 0 {
		data[byteOffset] |= (1 << (7 - bitOffset))
	} else {
		data[byteOffset] &= ^(1 << (7 - bitOffset))
	}

	obj.Touch(c.now())
	return int(oldBit), nil
}

// GetBit returns the bit value at offset in the string value stored at key.
func (c *Cache[K, V]) GetBit(key K, offset uint64) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, nil
	}

	if obj.Type != TypeString {
		return 0, cache.ErrWrongType
	}

	var data []byte
	switch v := obj.Value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return 0, cache.ErrWrongType
	}

	byteOffset := offset / 8
	bitOffset := offset % 8

	if uint64(len(data)) <= byteOffset {
		return 0, nil
	}

	byteVal := data[byteOffset]
	bit := (byteVal >> (7 - bitOffset)) & 1

	obj.Touch(c.now())
	return int(bit), nil
}

// BitCount counts the number of set bits (population count) in a string.
func (c *Cache[K, V]) BitCount(key K, start, end int) (int, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return 0, nil
	}

	if obj.Type != TypeString {
		return 0, cache.ErrWrongType
	}

	var data []byte
	switch v := obj.Value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return 0, cache.ErrWrongType
	}

	l := len(data)
	if l == 0 {
		return 0, nil
	}

	// Handle Redis-style negative indexing
	if start < 0 {
		start = l + start
	}
	if end < 0 {
		end = l + end
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = -1
	}

	if start > end || start >= l {
		return 0, nil
	}
	if end >= l {
		end = l - 1
	}

	count := 0
	for i := start; i <= end; i++ {
		count += bits.OnesCount8(data[i])
	}

	obj.Touch(c.now())
	return count, nil
}
