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

type Level struct {
	store Store
}

func NewLevel(store Store) *Level { return &Level{store: store} }
func (c *Level) Get(ctx context.Context, id uuid.UUID, load ports.LevelCacheLoader) (*contract.LevelResponse, error) {
	return commoncache.Fetch[*contract.LevelResponse](ctx, c.store, levelKey(id), constant.CultivationCacheTTL, func(loadCtx context.Context) (*contract.LevelResponse, error) { return load(loadCtx) })
}
func (c *Level) FindAll(ctx context.Context, load ports.LevelListCacheLoader) ([]*contract.LevelResponse, error) {
	return commoncache.Fetch[[]*contract.LevelResponse](ctx, c.store, constant.CultivationLevelAllKey, constant.CultivationCacheTTL, func(loadCtx context.Context) ([]*contract.LevelResponse, error) { return load(loadCtx) })
}
func (c *Level) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		keys := make([]string, 0, 2)
		if id != uuid.Nil {
			keys = append(keys, levelKey(id))
		}
		keys = append(keys, constant.CultivationLevelAllKey)
		return c.store.DeleteAll(deleteCtx, keys...)
	})
}
func levelKey(id uuid.UUID) string { return constant.CultivationLevelByIDKeyPrefix + id.String() }

var _ ports.LevelCache = (*Level)(nil)
