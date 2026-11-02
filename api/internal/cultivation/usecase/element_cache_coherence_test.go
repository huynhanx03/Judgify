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

	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	"github.com/huynhanx03/judgify/internal/constant"
	cultivationdto "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/testutil"
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
		service := NewElementService(repository, appcache.New(elementCacheCoordinator(cache)).CultivationElement).(*elementService)
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
		service := NewElementService(repository, appcache.New(elementCacheCoordinator(cache)).CultivationElement).(*elementService)
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

type elementTrackingCache struct {
	values   map[string]any
	versions map[string]string
	next     uint64
}

func newElementTrackingCache() *elementTrackingCache {
	return &elementTrackingCache{values: make(map[string]any), versions: make(map[string]string)}
}

func (cache *elementTrackingCache) Get(_ context.Context, key string) (any, bool) {
	value, ok := cache.values[key]
	return value, ok
}

func (cache *elementTrackingCache) Set(key string, value any) bool {
	cache.values[key] = value
	return true
}

func (cache *elementTrackingCache) SetWithTTL(_ context.Context, key string, value any, _ time.Duration) bool {
	return cache.Set(key, value)
}

func (*elementTrackingCache) Close() {}
func (cache *elementTrackingCache) Has(key string) bool {
	_, ok := cache.Get(key)
	return ok
}

func (cache *elementTrackingCache) SnapshotVersion(_ context.Context, key string) (string, error) {
	if token := cache.versions[key]; token != "" {
		return token, nil
	}
	cache.next++
	token := fmt.Sprintf("%d", cache.next)
	cache.versions[key] = token
	return token, nil
}
func (cache *elementTrackingCache) SetWithVersion(_ context.Context, versionKey, storageKey string, value any, _ time.Duration, token string) error {
	if cache.versions[versionKey] == token {
		cache.values[storageKey] = value
	}
	return nil
}
func (cache *elementTrackingCache) DeleteValueWithVersion(_ context.Context, versionKey, storageKey, token string) error {
	if cache.versions[versionKey] == token {
		delete(cache.values, storageKey)
	}
	return nil
}
func (cache *elementTrackingCache) Revoke(_ context.Context, key string) error {
	cache.next++
	cache.versions[key] = fmt.Sprintf("%d", cache.next)
	delete(cache.values, key)
	delete(cache.values, key+":neg")
	return nil
}

func (cache *elementTrackingCache) RevokeMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := cache.Revoke(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

var _ commoncache.L1Engine = (*elementTrackingCache)(nil)

func elementCacheCoordinator(store *elementTrackingCache) *commoncache.Cache {
	return testutil.NewCacheWithL1(store)
}
