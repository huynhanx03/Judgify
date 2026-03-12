package di

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"

	"github.com/huynhanx03/judgify/internal/core/service"
	"github.com/huynhanx03/judgify/internal/ports"
)

// CacheContainer holds cache-related dependencies.
type CacheContainer struct {
	Service ports.CacheService
}

// InitCacheDependencies initializes cache dependencies.
func InitCacheDependencies(
	cache cache.LocalCache[string, any],
) CacheContainer {
	svc := service.NewCacheService(cache)

	return CacheContainer{
		Service: svc,
	}
}
