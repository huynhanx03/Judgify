package cache

import (
	"context"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"
	"golang.org/x/sync/singleflight"

	"github.com/huynhanx03/judgify/internal/constant"
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

type Trait struct {
	store Store
	loads singleflight.Group
}

func NewTrait(store Store) *Trait { return &Trait{store: store} }
func (c *Trait) Get(ctx context.Context, id uuid.UUID, load ports.TraitCacheLoader) (*contract.TraitResponse, error) {
	return commoncache.Fetch[*contract.TraitResponse](c.store, &c.loads, traitKey(id), constant.CultivationCacheTTL, func() (*contract.TraitResponse, error) { return load(ctx) })
}
func (c *Trait) FindAll(ctx context.Context, load ports.TraitListCacheLoader) ([]*contract.TraitResponse, error) {
	return commoncache.Fetch[[]*contract.TraitResponse](c.store, &c.loads, constant.CultivationTraitAllKey, constant.CultivationCacheTTL, func() ([]*contract.TraitResponse, error) { return load(ctx) })
}
func (c *Trait) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		if id != uuid.Nil {
			commoncache.Delete(c.store, traitKey(id))
		}
		commoncache.Delete(c.store, constant.CultivationTraitAllKey)
		return nil
	})
}
func traitKey(id uuid.UUID) string { return constant.CultivationTraitByIDKeyPrefix + id.String() }

var _ ports.TraitCache = (*Trait)(nil)
