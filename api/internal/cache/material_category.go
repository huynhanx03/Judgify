package cache

import (
	"context"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"

	"github.com/huynhanx03/judgify/internal/constant"
	contract "github.com/huynhanx03/judgify/internal/material/contract"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

// MaterialCategory is the L1 cache adapter for material categories.
type MaterialCategory struct {
	store Store
}

func NewMaterialCategory(store Store) *MaterialCategory { return &MaterialCategory{store: store} }

func (cache *MaterialCategory) Get(ctx context.Context, id uuid.UUID, load ports.MaterialCategoryCacheLoader) (*contract.MaterialCategoryResult, error) {
	return commoncache.Fetch[*contract.MaterialCategoryResult](ctx, cache.store, tenantKey(ctx, materialCategoryByIDKey(id)), constant.MaterialCacheTTL, func(loadCtx context.Context) (*contract.MaterialCategoryResult, error) { return load(loadCtx) })
}

func (cache *MaterialCategory) FindAll(ctx context.Context, load ports.MaterialCategoryListCacheLoader) ([]*contract.MaterialCategoryResult, error) {
	return commoncache.Fetch[[]*contract.MaterialCategoryResult](ctx, cache.store, tenantKey(ctx, constant.MaterialCategoryAllKey), constant.MaterialCacheTTL, func(loadCtx context.Context) ([]*contract.MaterialCategoryResult, error) { return load(loadCtx) })
}

func (cache *MaterialCategory) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		keys := make([]string, 0, 2)
		if id != uuid.Nil {
			keys = append(keys, tenantKey(ctx, materialCategoryByIDKey(id)))
		}
		keys = append(keys, tenantKey(ctx, constant.MaterialCategoryAllKey))
		return cache.store.DeleteAll(deleteCtx, keys...)
	})
}

func materialCategoryByIDKey(id uuid.UUID) string {
	return constant.MaterialCategoryByIDKeyPrefix + id.String()
}

var _ ports.MaterialCategoryCache = (*MaterialCategory)(nil)
