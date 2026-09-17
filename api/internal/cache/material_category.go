package cache

import (
	"context"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"
	"golang.org/x/sync/singleflight"

	"github.com/huynhanx03/judgify/internal/constant"
	contract "github.com/huynhanx03/judgify/internal/material/contract"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

// MaterialCategory is the L1 cache adapter for material categories.
type MaterialCategory struct {
	store Store
	loads singleflight.Group
}

func NewMaterialCategory(store Store) *MaterialCategory { return &MaterialCategory{store: store} }

func (cache *MaterialCategory) Get(ctx context.Context, id uuid.UUID, load ports.MaterialCategoryCacheLoader) (*contract.MaterialCategoryResult, error) {
	return commoncache.Fetch[*contract.MaterialCategoryResult](cache.store, &cache.loads, materialCategoryByIDKey(id), constant.MaterialCacheTTL, func() (*contract.MaterialCategoryResult, error) { return load(ctx) })
}

func (cache *MaterialCategory) FindAll(ctx context.Context, load ports.MaterialCategoryListCacheLoader) ([]*contract.MaterialCategoryResult, error) {
	return commoncache.Fetch[[]*contract.MaterialCategoryResult](cache.store, &cache.loads, constant.MaterialCategoryAllKey, constant.MaterialCacheTTL, func() ([]*contract.MaterialCategoryResult, error) { return load(ctx) })
}

func (cache *MaterialCategory) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		if id != uuid.Nil {
			commoncache.Delete(cache.store, materialCategoryByIDKey(id))
		}
		commoncache.Delete(cache.store, constant.MaterialCategoryAllKey)
		return nil
	})
}

func materialCategoryByIDKey(id uuid.UUID) string {
	return constant.MaterialCategoryByIDKeyPrefix + id.String()
}

var _ ports.MaterialCategoryCache = (*MaterialCategory)(nil)
