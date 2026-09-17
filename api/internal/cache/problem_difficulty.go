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

// Difficulty is the process-local cache adapter for Problem difficulties.
type Difficulty struct {
	store Store
	loads singleflight.Group
}

func NewDifficulty(store Store) *Difficulty { return &Difficulty{store: store} }

func (cache *Difficulty) Get(ctx context.Context, id uuid.UUID, load ports.DifficultyCacheLoader) (*contract.DifficultyResult, error) {
	key := difficultyByIDKey(id)
	return commoncache.Fetch[*contract.DifficultyResult](cache.store, &cache.loads, key, constant.ProblemCacheTTL, func() (*contract.DifficultyResult, error) {
		return load(ctx)
	})
}

func (cache *Difficulty) FindAll(ctx context.Context, load ports.DifficultyListCacheLoader) ([]*contract.DifficultyResult, error) {
	return commoncache.Fetch[[]*contract.DifficultyResult](cache.store, &cache.loads, constant.ProblemDifficultyAllKey, constant.ProblemCacheTTL, func() ([]*contract.DifficultyResult, error) {
		return load(ctx)
	})
}

func (cache *Difficulty) Invalidate(ctx context.Context, id uuid.UUID) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		if id != uuid.Nil {
			commoncache.Delete(cache.store, difficultyByIDKey(id))
		}
		commoncache.Delete(cache.store, constant.ProblemDifficultyAllKey)
		return nil
	})
}

func (cache *Difficulty) InvalidateAll(ctx context.Context) error {
	return platformtx.AfterCommitOrNow(ctx, func(context.Context) error {
		commoncache.Delete(cache.store, constant.ProblemDifficultyAllKey)
		return nil
	})
}

func difficultyByIDKey(id uuid.UUID) string {
	return constant.ProblemDifficultyByIDKeyPrefix + id.String()
}

var _ ports.DifficultyCache = (*Difficulty)(nil)
