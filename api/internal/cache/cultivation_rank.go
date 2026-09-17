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

type Rank struct {
	store Store
	loads singleflight.Group
}

func NewRank(store Store) *Rank { return &Rank{store: store} }
func (c *Rank) Get(ctx context.Context, id uuid.UUID, load ports.RankCacheLoader) (*contract.RankResponse, error) {
	return commoncache.Fetch[*contract.RankResponse](c.store, &c.loads, rankKey(id), constant.CultivationCacheTTL, func() (*contract.RankResponse, error) { return load(ctx) })
}
func (c *Rank) FindAll(ctx context.Context, load ports.RankListCacheLoader) ([]*contract.RankResponse, error) {
	return commoncache.Fetch[[]*contract.RankResponse](c.store, &c.loads, constant.CultivationRankAllKey, constant.CultivationCacheTTL, func() ([]*contract.RankResponse, error) { return load(ctx) })
}
func (c *Rank) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		if id != uuid.Nil {
			commoncache.Delete(c.store, rankKey(id))
		}
		commoncache.Delete(c.store, constant.CultivationRankAllKey)
		return nil
	})
}
func rankKey(id uuid.UUID) string { return constant.CultivationRankByIDKeyPrefix + id.String() }

var _ ports.RankCache = (*Rank)(nil)
