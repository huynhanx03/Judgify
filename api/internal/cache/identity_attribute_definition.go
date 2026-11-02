package cache

import (
	"context"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"

	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

type AttributeDefinition struct {
	store Store
}

func NewAttributeDefinition(store Store) *AttributeDefinition {
	return &AttributeDefinition{store: store}
}
func (c *AttributeDefinition) Get(ctx context.Context, id uuid.UUID, load ports.AttributeDefinitionCacheLoader) (*entity.AttributeDefinition, error) {
	return commoncache.Fetch[*entity.AttributeDefinition](ctx, c.store, tenantKey(ctx, constant.CacheKeyPrefixAttrID+id.String()), constant.EntityCacheTTL, func(loadCtx context.Context) (*entity.AttributeDefinition, error) { return load(loadCtx) })
}
func (c *AttributeDefinition) Invalidate(ctx context.Context, id uuid.UUID, key string) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		keys := make([]string, 0, 2)
		if id != uuid.Nil {
			keys = append(keys, tenantKey(ctx, constant.CacheKeyPrefixAttrID+id.String()))
		}
		if key != "" {
			keys = append(keys, tenantKey(ctx, constant.CacheKeyPrefixAttrKey+key))
		}
		return c.store.DeleteAll(deleteCtx, keys...)
	})
}

var _ ports.AttributeDefinitionCache = (*AttributeDefinition)(nil)
