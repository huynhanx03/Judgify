package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	"github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	problemdto "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	problemmocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func TestDifficultyDeleteCacheInvalidationWaitsForOuterCommitAndSkipsRollback(t *testing.T) {
	t.Run("commit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repository := problemmocks.NewMockDifficultyRepository(ctrl)
		cache := newTaxonomyTrackingCache()
		id := uuid.New()
		cache.Set(constant.ProblemDifficultyByIDKeyPrefix+id.String(), "cached")
		cache.Set(constant.ProblemDifficultyAllKey, "cached")
		repository.EXPECT().Exists(gomock.Any(), id).Return(true, nil)
		repository.EXPECT().Delete(gomock.Any(), id).Return(nil)
		service := NewDifficultyService(repository, appcache.New(taxonomyCacheCoordinator(cache)).ProblemDifficulty).(*difficultyService)
		uow, sqlMock := newTaxonomyServiceUOW(t)
		sqlMock.ExpectBegin()
		sqlMock.ExpectCommit()

		require.NoError(t, uow.Within(context.Background(), func(txCtx context.Context) error {
			require.NoError(t, service.Delete(txCtx, id))
			require.True(t, cache.Has(constant.ProblemDifficultyByIDKeyPrefix+id.String()))
			require.True(t, cache.Has(constant.ProblemDifficultyAllKey))
			return nil
		}))
		require.False(t, cache.Has(constant.ProblemDifficultyByIDKeyPrefix+id.String()))
		require.False(t, cache.Has(constant.ProblemDifficultyAllKey))
		require.NoError(t, sqlMock.ExpectationsWereMet())
	})

	t.Run("rollback", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repository := problemmocks.NewMockDifficultyRepository(ctrl)
		cache := newTaxonomyTrackingCache()
		id := uuid.New()
		cache.Set(constant.ProblemDifficultyByIDKeyPrefix+id.String(), "cached")
		cache.Set(constant.ProblemDifficultyAllKey, "cached")
		repository.EXPECT().Exists(gomock.Any(), id).Return(true, nil)
		repository.EXPECT().Delete(gomock.Any(), id).Return(nil)
		service := NewDifficultyService(repository, appcache.New(taxonomyCacheCoordinator(cache)).ProblemDifficulty).(*difficultyService)
		uow, sqlMock := newTaxonomyServiceUOW(t)
		sqlMock.ExpectBegin()
		sqlMock.ExpectRollback()
		rollback := errors.New("rollback")

		err := uow.Within(context.Background(), func(txCtx context.Context) error {
			require.NoError(t, service.Delete(txCtx, id))
			return rollback
		})
		require.ErrorIs(t, err, rollback)
		require.True(t, cache.Has(constant.ProblemDifficultyByIDKeyPrefix+id.String()))
		require.True(t, cache.Has(constant.ProblemDifficultyAllKey))
		require.NoError(t, sqlMock.ExpectationsWereMet())
	})
}

func TestDifficultyVersionConflictLeavesCacheUntouched(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := problemmocks.NewMockDifficultyRepository(ctrl)
	cache := newTaxonomyTrackingCache()
	id := uuid.New()
	cache.Set(constant.ProblemDifficultyByIDKeyPrefix+id.String(), "cached")
	cache.Set(constant.ProblemDifficultyAllKey, "cached")
	repository.EXPECT().Get(gomock.Any(), id).Return(&entity.Difficulty{
		ID: id, Name: "Hard", Level: 9, Version: 1,
	}, nil)
	repository.EXPECT().Update(gomock.Any(), gomock.Any()).Return(ports.ErrDifficultyVersionConflict)
	service := NewDifficultyService(repository, appcache.New(taxonomyCacheCoordinator(cache)).ProblemDifficulty).(*difficultyService)
	name := "Expert"

	_, err := service.Update(context.Background(), id, &problemdto.UpdateDifficultyCommand{Name: &name})
	require.ErrorIs(t, err, ports.ErrDifficultyVersionConflict)
	require.True(t, cache.Has(constant.ProblemDifficultyByIDKeyPrefix+id.String()))
	require.True(t, cache.Has(constant.ProblemDifficultyAllKey))
}

func TestTagUpdateRegistersInvalidationBeforePresentationRefetchFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := problemmocks.NewMockTagRepository(ctrl)
	cache := newTaxonomyTrackingCache()
	id := uuid.New()
	cache.Set(constant.ProblemTagByIDKeyPrefix+id.String(), "cached")
	cache.Set(constant.ProblemTagAllKey, "cached")
	repository.EXPECT().Get(gomock.Any(), id).Return(&entity.Tag{ID: id, Name: "old", Version: 1}, nil)
	repository.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	refetchFailure := apperr.New(apperr.CodeDatabaseError, "refetch failed", nil)
	repository.EXPECT().Get(gomock.Any(), id).DoAndReturn(func(context.Context, uuid.UUID) (*entity.Tag, error) {
		require.False(t, cache.Has(constant.ProblemTagByIDKeyPrefix+id.String()), "ID cache must be evicted before re-fetch")
		require.False(t, cache.Has(constant.ProblemTagAllKey), "list cache must be evicted before re-fetch")
		return nil, refetchFailure
	})
	service := NewTagService(repository, appcache.New(taxonomyCacheCoordinator(cache)).ProblemTag).(*tagService)
	name := "new"

	response, err := service.Update(context.Background(), id, &problemdto.UpdateTagCommand{Name: &name})
	require.Nil(t, response)
	require.ErrorIs(t, err, refetchFailure)
}

func TestTagUpdateDefersCacheInvalidationUntilOuterCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := problemmocks.NewMockTagRepository(ctrl)
	cache := newTaxonomyTrackingCache()
	id := uuid.New()
	cache.Set(constant.ProblemTagByIDKeyPrefix+id.String(), "cached")
	cache.Set(constant.ProblemTagAllKey, "cached")
	repository.EXPECT().Get(gomock.Any(), id).Return(&entity.Tag{ID: id, Name: "old", Version: 1}, nil)
	repository.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	repository.EXPECT().Get(gomock.Any(), id).Return(&entity.Tag{ID: id, Name: "new", Version: 2}, nil)
	service := NewTagService(repository, appcache.New(taxonomyCacheCoordinator(cache)).ProblemTag).(*tagService)
	uow, sqlMock := newTaxonomyServiceUOW(t)
	sqlMock.ExpectBegin()
	sqlMock.ExpectCommit()
	name := "new"

	require.NoError(t, uow.Within(context.Background(), func(txCtx context.Context) error {
		response, err := service.Update(txCtx, id, &problemdto.UpdateTagCommand{Name: &name})
		require.NoError(t, err)
		require.Equal(t, int64(2), response.Version)
		require.True(t, cache.Has(constant.ProblemTagByIDKeyPrefix+id.String()))
		require.True(t, cache.Has(constant.ProblemTagAllKey))
		return nil
	}))
	require.False(t, cache.Has(constant.ProblemTagByIDKeyPrefix+id.String()))
	require.False(t, cache.Has(constant.ProblemTagAllKey))
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func newTaxonomyServiceUOW(t *testing.T) (platformtx.UnitOfWork, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, database)))
	t.Cleanup(func() { _ = client.Close() })
	return platformtx.NewEntUnitOfWork(client), mock
}

type taxonomyTrackingCache struct {
	values   map[string]any
	deletes  []string
	versions map[string]string
	next     uint64
}

func newTaxonomyTrackingCache() *taxonomyTrackingCache {
	return &taxonomyTrackingCache{values: make(map[string]any), versions: make(map[string]string)}
}

func (cache *taxonomyTrackingCache) Get(_ context.Context, key string) (any, bool) {
	value, ok := cache.values[key]
	return value, ok
}

func (cache *taxonomyTrackingCache) Set(key string, value any) bool {
	cache.values[key] = value
	return true
}

func (cache *taxonomyTrackingCache) SetWithTTL(_ context.Context, key string, value any, _ time.Duration) bool {
	return cache.Set(key, value)
}

func (*taxonomyTrackingCache) Close() {}
func (cache *taxonomyTrackingCache) Has(key string) bool {
	_, ok := cache.Get(key)
	return ok
}

func (cache *taxonomyTrackingCache) SnapshotVersion(_ context.Context, key string) (string, error) {
	if token := cache.versions[key]; token != "" {
		return token, nil
	}
	cache.next++
	token := fmt.Sprintf("%d", cache.next)
	cache.versions[key] = token
	return token, nil
}
func (cache *taxonomyTrackingCache) SetWithVersion(_ context.Context, versionKey, storageKey string, value any, _ time.Duration, token string) error {
	if cache.versions[versionKey] == token {
		cache.values[storageKey] = value
	}
	return nil
}
func (cache *taxonomyTrackingCache) DeleteValueWithVersion(_ context.Context, versionKey, storageKey, token string) error {
	if cache.versions[versionKey] == token {
		delete(cache.values, storageKey)
	}
	return nil
}
func (cache *taxonomyTrackingCache) Revoke(_ context.Context, key string) error {
	cache.next++
	cache.versions[key] = fmt.Sprintf("%d", cache.next)
	cache.deletes = append(cache.deletes, key)
	delete(cache.values, key)
	delete(cache.values, key+":neg")
	return nil
}

func (cache *taxonomyTrackingCache) RevokeMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := cache.Revoke(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

var _ commoncache.L1Engine = (*taxonomyTrackingCache)(nil)

func taxonomyCacheCoordinator(store *taxonomyTrackingCache) *commoncache.Cache {
	return testutil.NewCacheWithL1(store)
}
