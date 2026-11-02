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

type Rank struct {
	store Store
}

func NewRank(store Store) *Rank { return &Rank{store: store} }
func (c *Rank) Get(ctx context.Context, id uuid.UUID, load ports.RankCacheLoader) (*contract.RankResponse, error) {
	return commoncache.Fetch[*contract.RankResponse](ctx, c.store, rankKey(id), constant.CultivationCacheTTL, func(loadCtx context.Context) (*contract.RankResponse, error) { return load(loadCtx) })
}
func (c *Rank) FindAll(ctx context.Context, load ports.RankListCacheLoader) ([]*contract.RankResponse, error) {
	return commoncache.Fetch[[]*contract.RankResponse](ctx, c.store, constant.CultivationRankAllKey, constant.CultivationCacheTTL, func(loadCtx context.Context) ([]*contract.RankResponse, error) { return load(loadCtx) })
}
func (c *Rank) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		keys := make([]string, 0, 2)
		if id != uuid.Nil {
			keys = append(keys, rankKey(id))
		}
		keys = append(keys, constant.CultivationRankAllKey)
		return c.store.DeleteAll(deleteCtx, keys...)
	})
}
func rankKey(id uuid.UUID) string { return constant.CultivationRankByIDKeyPrefix + id.String() }

var _ ports.RankCache = (*Rank)(nil)
