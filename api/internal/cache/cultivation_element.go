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

type Element struct {
	store Store
	loads singleflight.Group
}

func NewElement(store Store) *Element { return &Element{store: store} }
func (c *Element) Get(ctx context.Context, id uuid.UUID, load ports.ElementCacheLoader) (*contract.ElementResponse, error) {
	return commoncache.Fetch[*contract.ElementResponse](c.store, &c.loads, elementKey(id), constant.CultivationCacheTTL, func() (*contract.ElementResponse, error) { return load(ctx) })
}
func (c *Element) FindAll(ctx context.Context, load ports.ElementListCacheLoader) ([]*contract.ElementResponse, error) {
	return commoncache.Fetch[[]*contract.ElementResponse](c.store, &c.loads, constant.CultivationElementAllKey, constant.CultivationCacheTTL, func() ([]*contract.ElementResponse, error) { return load(ctx) })
}
func (c *Element) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		if id != uuid.Nil {
			commoncache.Delete(c.store, elementKey(id))
		}
		commoncache.Delete(c.store, constant.CultivationElementAllKey)
		return nil
	})
}
func elementKey(id uuid.UUID) string { return constant.CultivationElementByIDKeyPrefix + id.String() }

var _ ports.ElementCache = (*Element)(nil)
