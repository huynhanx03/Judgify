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

type Rarity struct {
	store Store
}

func NewRarity(store Store) *Rarity { return &Rarity{store: store} }
func (c *Rarity) Get(ctx context.Context, id uuid.UUID, load ports.RarityCacheLoader) (*contract.RarityResponse, error) {
	return commoncache.Fetch[*contract.RarityResponse](ctx, c.store, rarityKey(id), constant.CultivationCacheTTL, func(loadCtx context.Context) (*contract.RarityResponse, error) { return load(loadCtx) })
}
func (c *Rarity) FindAll(ctx context.Context, load ports.RarityListCacheLoader) ([]*contract.RarityResponse, error) {
	return commoncache.Fetch[[]*contract.RarityResponse](ctx, c.store, constant.CultivationRarityAllKey, constant.CultivationCacheTTL, func(loadCtx context.Context) ([]*contract.RarityResponse, error) { return load(loadCtx) })
}
func (c *Rarity) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		keys := make([]string, 0, 2)
		if id != uuid.Nil {
			keys = append(keys, rarityKey(id))
		}
		keys = append(keys, constant.CultivationRarityAllKey)
		return c.store.DeleteAll(deleteCtx, keys...)
	})
}
func rarityKey(id uuid.UUID) string { return constant.CultivationRarityByIDKeyPrefix + id.String() }

var _ ports.RarityCache = (*Rarity)(nil)
