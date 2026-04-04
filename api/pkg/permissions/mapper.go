package permissions

import (
	"sync"
)

var (
	resourceMap = make(map[string]int)
	mapLock     sync.RWMutex

	resolverFunc func(key string) int
)

// SetResourceMap populates the cache with a known map (e.g. from DB on startup)
func SetResourceMap(mapping map[string]int) {
	mapLock.Lock()
	defer mapLock.Unlock()
	resourceMap = mapping
}

func SetResolver(fn func(key string) int) {
	mapLock.Lock()
	defer mapLock.Unlock()
	resolverFunc = fn
}

func GetResourceID(key string) int {
	mapLock.RLock()
	id, ok := resourceMap[key]
	mapLock.RUnlock()

	if ok {
		return id
	}

	mapLock.Lock()
	defer mapLock.Unlock()

	if id, ok = resourceMap[key]; ok {
		return id
	}

	// Use resolver if available
	if resolverFunc != nil {
		id = resolverFunc(key)
		if id != 0 {
			resourceMap[key] = id
		}
	}
	return id
}
