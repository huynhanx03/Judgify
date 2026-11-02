package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
	"github.com/huynhanx03/judgify/internal/testutil"
)

type memoryCacheStore struct {
	mu       sync.Mutex
	values   map[string]any
	versions map[string]string
	next     uint64
}

func (store *memoryCacheStore) Get(_ context.Context, key string) (any, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, ok := store.values[key]
	return value, ok
}

func (store *memoryCacheStore) SetWithTTL(_ context.Context, key string, value any, _ time.Duration) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.values[key] = value
	return true
}

func (store *memoryCacheStore) Close() {}

func (store *memoryCacheStore) SnapshotVersion(_ context.Context, key string) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if token := store.versions[key]; token != "" {
		return token, nil
	}
	store.next++
	if store.versions == nil {
		store.versions = make(map[string]string)
	}
	token := fmt.Sprintf("%d", store.next)
	store.versions[key] = token
	return token, nil
}
func (store *memoryCacheStore) SetWithVersion(_ context.Context, versionKey, storageKey string, value any, _ time.Duration, token string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.versions[versionKey] == token {
		store.values[storageKey] = value
	}
	return nil
}
func (store *memoryCacheStore) DeleteValueWithVersion(_ context.Context, versionKey, storageKey, token string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.versions[versionKey] == token {
		delete(store.values, storageKey)
	}
	return nil
}
func (store *memoryCacheStore) Revoke(_ context.Context, key string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.next++
	if store.versions == nil {
		store.versions = make(map[string]string)
	}
	store.versions[key] = fmt.Sprintf("%d", store.next)
	delete(store.values, key)
	delete(store.values, key+":neg")
	return nil
}

func (store *memoryCacheStore) RevokeMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := store.Revoke(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

var _ commoncache.L1Engine = (*memoryCacheStore)(nil)

func newMemoryCacheCoordinator(store *memoryCacheStore) *commoncache.Cache {
	return testutil.NewCacheWithL1(store)
}

func TestTagOverlappingMissesShareOneLoad(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		adapter := NewTag(newMemoryCacheCoordinator(&memoryCacheStore{values: map[string]any{}}))
		key := uuid.MustParse("11111111-1111-4111-8111-111111111111")
		release := make(chan struct{})
		errors := make(chan error, 2)
		var loads atomic.Int32
		load := func(context.Context) (*contract.TagResult, error) {
			loads.Add(1)
			<-release
			return &contract.TagResult{}, nil
		}
		for range 2 {
			go func() {
				_, err := adapter.Get(context.Background(), key, load)
				errors <- err
			}()
		}
		synctest.Wait()
		observed := loads.Load()
		close(release)
		for range 2 {
			if err := <-errors; err != nil {
				t.Fatal(err)
			}
		}
		if observed != 1 {
			t.Fatalf("overlapping backend loads = %d, want 1", observed)
		}
	})
}

func TestTagDifferentKeysLoadIndependently(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		adapter := NewTag(newMemoryCacheCoordinator(&memoryCacheStore{values: map[string]any{}}))
		keys := []uuid.UUID{
			uuid.MustParse("11111111-1111-4111-8111-111111111111"),
			uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		}
		release := make(chan struct{})
		errors := make(chan error, 2)
		var loads atomic.Int32
		load := func(context.Context) (*contract.TagResult, error) {
			loads.Add(1)
			<-release
			return &contract.TagResult{}, nil
		}
		for _, key := range keys {
			go func() {
				_, err := adapter.Get(context.Background(), key, load)
				errors <- err
			}()
		}
		synctest.Wait()
		observed := loads.Load()
		close(release)
		for range 2 {
			if err := <-errors; err != nil {
				t.Fatal(err)
			}
		}
		if observed != 2 {
			t.Fatalf("different-key backend loads = %d, want 2", observed)
		}
	})
}
