package cache

import (
	"context"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"
	"golang.org/x/sync/singleflight"

	"github.com/huynhanx03/judgify/internal/constant"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

type Role struct {
	store Store
	loads singleflight.Group
}

func NewRole(store Store) *Role { return &Role{store: store} }
func (c *Role) Get(ctx context.Context, id uuid.UUID, load ports.RoleCacheLoader) (*dto.RoleResponse, error) {
	return commoncache.Fetch[*dto.RoleResponse](c.store, &c.loads, roleByIDKey(id), constant.EntityCacheTTL, func() (*dto.RoleResponse, error) { return load(ctx) })
}
func (c *Role) FindAll(ctx context.Context, load ports.RoleListCacheLoader) ([]*dto.RoleResponse, error) {
	return commoncache.Fetch[[]*dto.RoleResponse](c.store, &c.loads, constant.IdentityRoleAllKey, constant.EntityCacheTTL, func() ([]*dto.RoleResponse, error) { return load(ctx) })
}
func (c *Role) ResolveByKey(ctx context.Context, key string, load ports.RoleEntityCacheLoader) (*entity.Role, error) {
	return commoncache.Fetch[*entity.Role](c.store, &c.loads, constant.CacheKeyPrefixRoleName+key, constant.EntityCacheTTL, func() (*entity.Role, error) { return load(ctx) })
}
func (c *Role) Invalidate(ctx context.Context, id uuid.UUID, key string) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		if id != uuid.Nil {
			commoncache.Delete(c.store, roleByIDKey(id))
		}
		commoncache.Delete(c.store, constant.IdentityRoleAllKey)
		if key != "" {
			commoncache.Delete(c.store, constant.CacheKeyPrefixRoleName+key)
		}
		return nil
	})
}
func roleByIDKey(id uuid.UUID) string { return constant.IdentityRoleByIDKeyPrefix + id.String() }

var _ ports.RoleCache = (*Role)(nil)
