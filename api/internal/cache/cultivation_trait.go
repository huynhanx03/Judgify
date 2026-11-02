package cache

import (
	"context"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"

	"github.com/huynhanx03/judgify/internal/constant"
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

type Trait struct {
	store Store
}

func NewTrait(store Store) *Trait { return &Trait{store: store} }
func (c *Trait) Get(ctx context.Context, id uuid.UUID, load ports.TraitCacheLoader) (*contract.TraitResponse, error) {
	return commoncache.Fetch[*contract.TraitResponse](ctx, c.store, traitKey(id), constant.CultivationCacheTTL, func(loadCtx context.Context) (*contract.TraitResponse, error) { return load(loadCtx) })
}
func (c *Trait) FindAll(ctx context.Context, load ports.TraitListCacheLoader) ([]*contract.TraitResponse, error) {
	return commoncache.Fetch[[]*contract.TraitResponse](ctx, c.store, constant.CultivationTraitAllKey, constant.CultivationCacheTTL, func(loadCtx context.Context) ([]*contract.TraitResponse, error) { return load(loadCtx) })
}
func (c *Trait) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		keys := make([]string, 0, 2)
		if id != uuid.Nil {
			keys = append(keys, traitKey(id))
		}
		keys = append(keys, constant.CultivationTraitAllKey)
		return c.store.DeleteAll(deleteCtx, keys...)
	})
}
func traitKey(id uuid.UUID) string { return constant.CultivationTraitByIDKeyPrefix + id.String() }

var _ ports.TraitCache = (*Trait)(nil)
