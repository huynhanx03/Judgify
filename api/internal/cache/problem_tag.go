package cache

import (
	"context"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"
	"golang.org/x/sync/singleflight"

	"github.com/huynhanx03/judgify/internal/constant"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
)

// Tag is the process-local cache adapter for Problem tags.
type Tag struct {
	store Store
	loads singleflight.Group
}

func NewTag(store Store) *Tag { return &Tag{store: store} }

func (cache *Tag) Get(ctx context.Context, id uuid.UUID, load ports.TagCacheLoader) (*contract.TagResult, error) {
	key := tagByIDKey(id)
	return commoncache.Fetch[*contract.TagResult](cache.store, &cache.loads, key, constant.ProblemCacheTTL, func() (*contract.TagResult, error) {
		return load(ctx)
	})
}

func (cache *Tag) FindAll(ctx context.Context, load ports.TagListCacheLoader) ([]*contract.TagResult, error) {
	return commoncache.Fetch[[]*contract.TagResult](cache.store, &cache.loads, constant.ProblemTagAllKey, constant.ProblemCacheTTL, func() ([]*contract.TagResult, error) {
		return load(ctx)
	})
}

// Invalidate removes every cached projection derived from one tag only after
// the surrounding transaction commits; outside a transaction it runs now.
func (cache *Tag) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		if id != uuid.Nil {
			commoncache.Delete(cache.store, tagByIDKey(id))
		}
		commoncache.Delete(cache.store, constant.ProblemTagAllKey)
		return nil
	})
}

func (cache *Tag) InvalidateAll(ctx context.Context) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		commoncache.Delete(cache.store, constant.ProblemTagAllKey)
		return nil
	})
}

func tagByIDKey(id uuid.UUID) string { return constant.ProblemTagByIDKeyPrefix + id.String() }

var _ ports.TagCache = (*Tag)(nil)
