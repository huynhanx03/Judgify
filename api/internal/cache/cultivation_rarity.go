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

type Rarity struct {
	store Store
	loads singleflight.Group
}

func NewRarity(store Store) *Rarity { return &Rarity{store: store} }
func (c *Rarity) Get(ctx context.Context, id uuid.UUID, load ports.RarityCacheLoader) (*contract.RarityResponse, error) {
	return commoncache.Fetch[*contract.RarityResponse](c.store, &c.loads, rarityKey(id), constant.CultivationCacheTTL, func() (*contract.RarityResponse, error) { return load(ctx) })
}
func (c *Rarity) FindAll(ctx context.Context, load ports.RarityListCacheLoader) ([]*contract.RarityResponse, error) {
	return commoncache.Fetch[[]*contract.RarityResponse](c.store, &c.loads, constant.CultivationRarityAllKey, constant.CultivationCacheTTL, func() ([]*contract.RarityResponse, error) { return load(ctx) })
}
func (c *Rarity) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		if id != uuid.Nil {
			commoncache.Delete(c.store, rarityKey(id))
		}
		commoncache.Delete(c.store, constant.CultivationRarityAllKey)
		return nil
	})
}
func rarityKey(id uuid.UUID) string { return constant.CultivationRarityByIDKeyPrefix + id.String() }

var _ ports.RarityCache = (*Rarity)(nil)
