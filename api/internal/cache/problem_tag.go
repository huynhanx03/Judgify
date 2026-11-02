package cache

import (
	"context"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"

	"github.com/huynhanx03/judgify/internal/constant"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
)

// Tag is the process-local cache adapter for Problem tags.
type Tag struct {
	store Store
}

func NewTag(store Store) *Tag { return &Tag{store: store} }

func (cache *Tag) Get(ctx context.Context, id uuid.UUID, load ports.TagCacheLoader) (*contract.TagResult, error) {
	key := tagByIDKey(id)
	return commoncache.Fetch[*contract.TagResult](ctx, cache.store, key, constant.ProblemCacheTTL, func(loadCtx context.Context) (*contract.TagResult, error) {
		return load(loadCtx)
	})
}

func (cache *Tag) FindAll(ctx context.Context, load ports.TagListCacheLoader) ([]*contract.TagResult, error) {
	return commoncache.Fetch[[]*contract.TagResult](ctx, cache.store, constant.ProblemTagAllKey, constant.ProblemCacheTTL, func(loadCtx context.Context) ([]*contract.TagResult, error) {
		return load(loadCtx)
	})
}

// Invalidate removes every cached projection derived from one tag only after
// the surrounding transaction commits; outside a transaction it runs now.
func (cache *Tag) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		keys := make([]string, 0, 2)
		if id != uuid.Nil {
			keys = append(keys, tagByIDKey(id))
		}
		keys = append(keys, constant.ProblemTagAllKey)
		return cache.store.DeleteAll(deleteCtx, keys...)
	})
}

func (cache *Tag) InvalidateAll(ctx context.Context) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		return cache.store.DeleteAll(deleteCtx, constant.ProblemTagAllKey)
	})
}

func tagByIDKey(id uuid.UUID) string { return constant.ProblemTagByIDKeyPrefix + id.String() }

var _ ports.TagCache = (*Tag)(nil)
