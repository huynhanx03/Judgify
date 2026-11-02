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

// Difficulty is the process-local cache adapter for Problem difficulties.
type Difficulty struct {
	store Store
}

func NewDifficulty(store Store) *Difficulty { return &Difficulty{store: store} }

func (cache *Difficulty) Get(ctx context.Context, id uuid.UUID, load ports.DifficultyCacheLoader) (*contract.DifficultyResult, error) {
	key := difficultyByIDKey(id)
	return commoncache.Fetch[*contract.DifficultyResult](ctx, cache.store, key, constant.ProblemCacheTTL, func(loadCtx context.Context) (*contract.DifficultyResult, error) {
		return load(loadCtx)
	})
}

func (cache *Difficulty) FindAll(ctx context.Context, load ports.DifficultyListCacheLoader) ([]*contract.DifficultyResult, error) {
	return commoncache.Fetch[[]*contract.DifficultyResult](ctx, cache.store, constant.ProblemDifficultyAllKey, constant.ProblemCacheTTL, func(loadCtx context.Context) ([]*contract.DifficultyResult, error) {
		return load(loadCtx)
	})
}

func (cache *Difficulty) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		keys := make([]string, 0, 2)
		if id != uuid.Nil {
			keys = append(keys, difficultyByIDKey(id))
		}
		keys = append(keys, constant.ProblemDifficultyAllKey)
		return cache.store.DeleteAll(deleteCtx, keys...)
	})
}

func (cache *Difficulty) InvalidateAll(ctx context.Context) error {
	return platformtx.AfterCommitOrNow(ctx, func(deleteCtx context.Context) error {
		return cache.store.DeleteAll(deleteCtx, constant.ProblemDifficultyAllKey)
	})
}

func difficultyByIDKey(id uuid.UUID) string {
	return constant.ProblemDifficultyByIDKeyPrefix + id.String()
}

var _ ports.DifficultyCache = (*Difficulty)(nil)
