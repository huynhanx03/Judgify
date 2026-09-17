package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	commoncache "github.com/huynhanx03/go-common/pkg/common/cache"
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
)

type memoryCacheStore struct {
	mu     sync.Mutex
	values map[string]any
}

func (store *memoryCacheStore) Get(key string) (any, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, ok := store.values[key]
	return value, ok
}

func (store *memoryCacheStore) Set(key string, value any) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.values[key] = value
	return true
}

func (store *memoryCacheStore) SetWithTTL(key string, value any, _ time.Duration) bool {
	return store.Set(key, value)
}

func (store *memoryCacheStore) Delete(key string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.values, key)
}

func (store *memoryCacheStore) Clear() {
	store.mu.Lock()
	defer store.mu.Unlock()
	clear(store.values)
}

func (store *memoryCacheStore) Close() {}

func (store *memoryCacheStore) Stats() commoncache.Stats { return commoncache.Stats{} }

func TestTagOverlappingMissesShareOneLoad(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		adapter := NewTag(&memoryCacheStore{values: map[string]any{}})
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
		adapter := NewTag(&memoryCacheStore{values: map[string]any{}})
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
