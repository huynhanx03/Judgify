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

type Element struct {
	store Store
}

func NewElement(store Store) *Element { return &Element{store: store} }
func (c *Element) Get(ctx context.Context, id uuid.UUID, load ports.ElementCacheLoader) (*contract.ElementResponse, error) {
	return commoncache.Fetch[*contract.ElementResponse](ctx, c.store, elementKey(id), constant.CultivationCacheTTL, func(loadCtx context.Context) (*contract.ElementResponse, error) { return load(loadCtx) })
}
func (c *Element) FindAll(ctx context.Context, load ports.ElementListCacheLoader) ([]*contract.ElementResponse, error) {
	return commoncache.Fetch[[]*contract.ElementResponse](ctx, c.store, constant.CultivationElementAllKey, constant.CultivationCacheTTL, func(loadCtx context.Context) ([]*contract.ElementResponse, error) { return load(loadCtx) })
}
func (c *Element) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		keys := make([]string, 0, 2)
		if id != uuid.Nil {
			keys = append(keys, elementKey(id))
		}
		keys = append(keys, constant.CultivationElementAllKey)
		return c.store.DeleteAll(deleteCtx, keys...)
	})
}
func elementKey(id uuid.UUID) string { return constant.CultivationElementByIDKeyPrefix + id.String() }

var _ ports.ElementCache = (*Element)(nil)
