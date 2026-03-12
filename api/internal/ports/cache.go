package ports

import "context"

// CacheService defines the interface for cache-related operations.
type CacheService interface {
	// InvalidatePermissionConfig increments the permission config version, invalidating all role permission caches.
	InvalidatePermissionConfig(ctx context.Context) error

	// GetPermissionConfigVersion retrieves the current permission config version.
	GetPermissionConfigVersion(ctx context.Context) (int64, error)
}
