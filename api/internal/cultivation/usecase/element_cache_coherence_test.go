package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	"github.com/huynhanx03/judgify/internal/constant"
	cultivationdto "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	cultivationmocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func TestElementCreateCacheInvalidationWaitsForOuterCommitAndSkipsRollback(t *testing.T) {
	t.Run("commit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repository := cultivationmocks.NewMockElementRepository(ctrl)
		cache := newElementTrackingCache()
		id := uuid.New()
		cache.Set(constant.CultivationElementByIDKeyPrefix+id.String(), "cached")
		cache.Set(constant.CultivationElementAllKey, "cached")
		repository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, value *entity.Element) error {
				value.ID = id
				value.Version = 1
				return nil
			},
		)
		service := NewElementService(repository, appcache.New(cache).CultivationElement).(*elementService)
		uow, sqlMock := newElementServiceUOW(t)
		sqlMock.ExpectBegin()
		sqlMock.ExpectCommit()

		require.NoError(t, uow.Within(context.Background(), func(txCtx context.Context) error {
			response, err := service.Create(txCtx, &cultivationdto.CreateElementCommand{
				Name: "Water", Code: "water",
			})
			require.NoError(t, err)
			require.Equal(t, int64(1), response.Version)
			require.True(t, cache.Has(constant.CultivationElementByIDKeyPrefix+id.String()))
			require.True(t, cache.Has(constant.CultivationElementAllKey))
			return nil
		}))
		require.False(t, cache.Has(constant.CultivationElementByIDKeyPrefix+id.String()))
		require.False(t, cache.Has(constant.CultivationElementAllKey))
		require.NoError(t, sqlMock.ExpectationsWereMet())
	})

	t.Run("rollback", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repository := cultivationmocks.NewMockElementRepository(ctrl)
		cache := newElementTrackingCache()
		id := uuid.New()
		cache.Set(constant.CultivationElementByIDKeyPrefix+id.String(), "cached")
		cache.Set(constant.CultivationElementAllKey, "cached")
		repository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, value *entity.Element) error {
				value.ID = id
				value.Version = 1
				return nil
			},
		)
		service := NewElementService(repository, appcache.New(cache).CultivationElement).(*elementService)
		uow, sqlMock := newElementServiceUOW(t)
		sqlMock.ExpectBegin()
		sqlMock.ExpectRollback()
		rollback := errors.New("rollback")

		err := uow.Within(context.Background(), func(txCtx context.Context) error {
			_, createErr := service.Create(txCtx, &cultivationdto.CreateElementCommand{
				Name: "Water", Code: "water",
			})
			require.NoError(t, createErr)
			return rollback
		})
		require.ErrorIs(t, err, rollback)
		require.True(t, cache.Has(constant.CultivationElementByIDKeyPrefix+id.String()))
		require.True(t, cache.Has(constant.CultivationElementAllKey))
		require.NoError(t, sqlMock.ExpectationsWereMet())
	})
}

func newElementServiceUOW(t *testing.T) (platformtx.UnitOfWork, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, database)))
	t.Cleanup(func() { _ = client.Close() })
	return platformtx.NewEntUnitOfWork(client), mock
}

type elementTrackingCache struct{ values map[string]any }

func newElementTrackingCache() *elementTrackingCache {
	return &elementTrackingCache{values: make(map[string]any)}
}

func (cache *elementTrackingCache) Get(key string) (any, bool) {
	value, ok := cache.values[key]
	return value, ok
}

func (cache *elementTrackingCache) Set(key string, value any) bool {
	cache.values[key] = value
	return true
}

func (cache *elementTrackingCache) SetWithTTL(key string, value any, _ time.Duration) bool {
	return cache.Set(key, value)
}

func (cache *elementTrackingCache) Delete(key string) { delete(cache.values, key) }
func (cache *elementTrackingCache) Clear()            { clear(cache.values) }
func (*elementTrackingCache) Close()                  {}
func (cache *elementTrackingCache) Stats() commoncache.Stats {
	return commoncache.Stats{KeyCount: int64(len(cache.values))}
}
func (cache *elementTrackingCache) Has(key string) bool {
	_, ok := cache.Get(key)
	return ok
}

var _ commoncache.LocalCache[string, any] = (*elementTrackingCache)(nil)
