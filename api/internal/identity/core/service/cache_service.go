package service

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/cache"

	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

const (
	defaultPermissionConfigVersion = 1
)

type cacheService struct {
	cache cache.LocalCache[string, any]
}

// NewCacheService creates a new CacheService instance.
func NewCacheService(cache cache.LocalCache[string, any]) ports.CacheService {
	return &cacheService{cache: cache}
}

// InvalidatePermissionConfig increments the permission config version.
func (s *cacheService) InvalidatePermissionConfig(ctx context.Context) error {
	version, _ := s.GetPermissionConfigVersion(ctx)
	newVersion := version + 1
	cache.Set(s.cache, constant.CacheKeyPermissionConfigVersion, newVersion)
	return nil
}

// GetPermissionConfigVersion retrieves the current permission config version.
func (s *cacheService) GetPermissionConfigVersion(ctx context.Context) (int64, error) {
	if v, found := cache.Get[int64](s.cache, constant.CacheKeyPermissionConfigVersion); found {
		return v, nil
	}

	return defaultPermissionConfigVersion, nil
}
