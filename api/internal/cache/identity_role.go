package cache

import (
	"context"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"

	"github.com/huynhanx03/judgify/internal/constant"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

type Role struct {
	store Store
}

func NewRole(store Store) *Role { return &Role{store: store} }
func (c *Role) Get(ctx context.Context, id uuid.UUID, load ports.RoleCacheLoader) (*dto.RoleResponse, error) {
	return commoncache.Fetch[*dto.RoleResponse](ctx, c.store, roleByIDKey(id), constant.EntityCacheTTL, func(loadCtx context.Context) (*dto.RoleResponse, error) { return load(loadCtx) })
}
func (c *Role) FindAll(ctx context.Context, load ports.RoleListCacheLoader) ([]*dto.RoleResponse, error) {
	return commoncache.Fetch[[]*dto.RoleResponse](ctx, c.store, constant.IdentityRoleAllKey, constant.EntityCacheTTL, func(loadCtx context.Context) ([]*dto.RoleResponse, error) { return load(loadCtx) })
}
func (c *Role) ResolveByKey(ctx context.Context, key string, load ports.RoleEntityCacheLoader) (*entity.Role, error) {
	return commoncache.Fetch[*entity.Role](ctx, c.store, constant.CacheKeyPrefixRoleName+key, constant.EntityCacheTTL, func(loadCtx context.Context) (*entity.Role, error) { return load(loadCtx) })
}
func (c *Role) Invalidate(ctx context.Context, id uuid.UUID, key string) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		keys := make([]string, 0, 3)
		if id != uuid.Nil {
			keys = append(keys, roleByIDKey(id))
		}
		keys = append(keys, constant.IdentityRoleAllKey)
		if key != "" {
			keys = append(keys, constant.CacheKeyPrefixRoleName+key)
		}
		return c.store.DeleteAll(deleteCtx, keys...)
	})
}
func roleByIDKey(id uuid.UUID) string { return constant.IdentityRoleByIDKeyPrefix + id.String() }

var _ ports.RoleCache = (*Role)(nil)
