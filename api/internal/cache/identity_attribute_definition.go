package cache

import (
	"context"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"
	"golang.org/x/sync/singleflight"

	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

type AttributeDefinition struct {
	store Store
	loads singleflight.Group
}

func NewAttributeDefinition(store Store) *AttributeDefinition {
	return &AttributeDefinition{store: store}
}
func (c *AttributeDefinition) Get(ctx context.Context, id uuid.UUID, load ports.AttributeDefinitionCacheLoader) (*entity.AttributeDefinition, error) {
	return commoncache.Fetch[*entity.AttributeDefinition](c.store, &c.loads, constant.CacheKeyPrefixAttrID+id.String(), constant.EntityCacheTTL, func() (*entity.AttributeDefinition, error) { return load(ctx) })
}
func (c *AttributeDefinition) Invalidate(ctx context.Context, id uuid.UUID, key string) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		if id != uuid.Nil {
			commoncache.Delete(c.store, constant.CacheKeyPrefixAttrID+id.String())
		}
		if key != "" {
			commoncache.Delete(c.store, constant.CacheKeyPrefixAttrKey+key)
		}
		return nil
	})
}

var _ ports.AttributeDefinitionCache = (*AttributeDefinition)(nil)
