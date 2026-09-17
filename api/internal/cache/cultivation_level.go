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

type Level struct {
	store Store
	loads singleflight.Group
}

func NewLevel(store Store) *Level { return &Level{store: store} }
func (c *Level) Get(ctx context.Context, id uuid.UUID, load ports.LevelCacheLoader) (*contract.LevelResponse, error) {
	return commoncache.Fetch[*contract.LevelResponse](c.store, &c.loads, levelKey(id), constant.CultivationCacheTTL, func() (*contract.LevelResponse, error) { return load(ctx) })
}
func (c *Level) FindAll(ctx context.Context, load ports.LevelListCacheLoader) ([]*contract.LevelResponse, error) {
	return commoncache.Fetch[[]*contract.LevelResponse](c.store, &c.loads, constant.CultivationLevelAllKey, constant.CultivationCacheTTL, func() ([]*contract.LevelResponse, error) { return load(ctx) })
}
func (c *Level) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		if id != uuid.Nil {
			commoncache.Delete(c.store, levelKey(id))
		}
		commoncache.Delete(c.store, constant.CultivationLevelAllKey)
		return nil
	})
}
func levelKey(id uuid.UUID) string { return constant.CultivationLevelByIDKeyPrefix + id.String() }

var _ ports.LevelCache = (*Level)(nil)
