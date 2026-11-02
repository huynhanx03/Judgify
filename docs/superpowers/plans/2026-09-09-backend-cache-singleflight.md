# Backend Cache Singleflight Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking. Chỉ bắt đầu sau khi người dùng đổi model implementation và yêu cầu triển khai.

**Goal:** Các caller đồng thời cùng cache key dùng chung một loader invocation, không copy synchronization state.

**Architecture:** Mỗi cache adapter có một instance dùng pointer và sở hữu một singleflight.Group. Registry giữ pointer đến adapter; usecase tiếp tục nhận narrow cache ports hiện có. Không thay go-common cache implementation, TTL hoặc transaction hooks.

**Tech Stack:** Go 1.27.1, `golang.org/x/sync/singleflight`, `testing/synctest`, cache ports hiện tại.

**Spec:** [Audit/design, F02](/Users/jerry/nhanhmc/coding/Judgify/docs/superpowers/specs/2026-09-09-backend-audit-and-upgrade-design.md).

**Plan validation:** Test code bên dưới đã compile trên bản sao source trong module tạm ngày 2026-09-09. Same-key test fail đúng `overlapping backend loads = 2, want 1`; different-key test qua. Chưa thêm test hoặc fix vào production repository.

## Global Constraints

- Giữ Go 1.27.1.
- Không stage/commit/reset/clean working tree.
- Không tắt copylocks, staticcheck hoặc race detector để làm xanh.
- Không đổi public HTTP contract, DB schema hoặc cache key/TTL.
- Giữ invalidation sau commit và bỏ invalidation khi rollback.
- Không gộp sửa session/rating/counter vào patch cache.

## Task 1: Regression cho singleflight ở public adapter boundary

**Files:** Create `api/internal/cache/singleflight_test.go`.

**Interfaces:** Consumes `NewTag(Store)`, `Tag.Get(context.Context, uuid.UUID, ports.TagCacheLoader)` và LocalCache interface. Sử dụng typed cache port làm boundary; test không dựa trên tên private field `loads`.

- [ ] **Step 1: Viết test dưới đây.**

```go
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
    mu sync.Mutex
    values map[string]any
}

func (s *memoryCacheStore) Get(key string) (any, bool) {
    s.mu.Lock(); defer s.mu.Unlock()
    value, ok := s.values[key]
    return value, ok
}
func (s *memoryCacheStore) Set(key string, value any) bool {
    s.mu.Lock(); defer s.mu.Unlock()
    s.values[key] = value
    return true
}
func (s *memoryCacheStore) SetWithTTL(key string, value any, _ time.Duration) bool { return s.Set(key, value) }
func (s *memoryCacheStore) Delete(key string) { s.mu.Lock(); defer s.mu.Unlock(); delete(s.values, key) }
func (s *memoryCacheStore) Clear() { s.mu.Lock(); defer s.mu.Unlock(); clear(s.values) }
func (s *memoryCacheStore) Close() {}
func (s *memoryCacheStore) Stats() commoncache.Stats { return commoncache.Stats{} }

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
        for range 2 { if err := <-errors; err != nil { t.Fatal(err) } }
        if observed != 1 { t.Fatalf("overlapping backend loads = %d, want 1", observed) }
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
        for range 2 { if err := <-errors; err != nil { t.Fatal(err) } }
        if observed != 2 { t.Fatalf("different-key backend loads = %d, want 2", observed) }
    })
}
```

In-memory storage chỉ phục vụ test đồng thời trong synctest bubble; TTL không phải hành vi đang kiểm tra. Tránh Ristretto/background goroutine bên ngoài bubble. Không dùng sleep để đoán các caller đã tới singleflight.

- [ ] **Step 2: Chạy test RED.**

Từ `/Users/jerry/nhanhmc/coding/Judgify/api`:

```sh
go test ./internal/cache -run 'TestTag(Overlapping|Different)' -count=1
```

Kỳ vọng test same-key báo 2 backend loads thay vì 1; test different-key qua. Lỗi compile không được coi là regression đã tái hiện.

## Task 2: Chuyển ownership của toàn bộ adapter sang pointer

**Files:**

- Modify `api/internal/cache/cache.go`.
- Modify `api/internal/cache/problem_tag.go`, `problem_difficulty.go`, `material_category.go`.
- Modify `api/internal/cache/cultivation_element.go`, `cultivation_level.go`, `cultivation_rank.go`, `cultivation_rarity.go`, `cultivation_trait.go`.
- Modify `api/internal/cache/identity_role.go`, `identity_attribute_definition.go`.
- Existing verification: tests của usecase identity/problem/cultivation/material và DI.

**Interfaces:** Constructors giữ tên hiện có nhưng trả pointer đến concrete adapter. Narrow ports không đổi. Registry đổi các field concrete sang pointer tương ứng.

- [ ] **Step 1: Đổi constructor, receiver và assertion.**

Với Tag, thay đúng ba dạng sau:

```go
func NewTag(store Store) *Tag { return &Tag{store: store} }

func (cache *Tag) Get(ctx context.Context, id uuid.UUID, load ports.TagCacheLoader) (*contract.TagResult, error) {
    key := tagByIDKey(id)
    return commoncache.Fetch[*contract.TagResult](cache.store, &cache.loads, key, constant.ProblemCacheTTL, func() (*contract.TagResult, error) {
        return load(ctx)
    })
}

var _ ports.TagCache = (*Tag)(nil)
```

Đổi receiver của cả FindAll/Invalidate/InvalidateAll sang pointer. Giữ nguyên thân method. Áp dụng mapping đầy đủ, không bỏ sót adapter:

| Constructor return | Receiver type | Port assertion |
|---|---|---|
| `*Tag` | `*Tag` | `ports.TagCache` |
| `*Difficulty` | `*Difficulty` | `ports.DifficultyCache` |
| `*MaterialCategory` | `*MaterialCategory` | `ports.MaterialCategoryCache` |
| `*Element` | `*Element` | `ports.ElementCache` |
| `*Level` | `*Level` | `ports.LevelCache` |
| `*Rank` | `*Rank` | `ports.RankCache` |
| `*Rarity` | `*Rarity` | `ports.RarityCache` |
| `*Trait` | `*Trait` | `ports.TraitCache` |
| `*Role` | `*Role` | `ports.RoleCache` |
| `*AttributeDefinition` | `*AttributeDefinition` | `ports.AttributeDefinitionCache` |

- [ ] **Step 2: Đổi Registry thành pointer fields.**

```go
type Registry struct {
    ProblemTag                  *Tag
    ProblemDifficulty           *Difficulty
    MaterialCategory            *MaterialCategory
    CultivationLevel            *Level
    CultivationElement          *Element
    CultivationRank             *Rank
    CultivationRarity           *Rarity
    CultivationTrait            *Trait
    IdentityRole                *Role
    IdentityAttributeDefinition *AttributeDefinition
}
```

`New(store Store) Registry` tiếp tục gọi các constructor hiện có. Usecase constructors đã nhận port interfaces nên không cần thay chúng để truyền pointer. Chỉ cập nhật call site nếu compilation chỉ ra explicit concrete value construction; không sửa generated mocks.

- [ ] **Step 3: Chạy GREEN và kiểm tra concurrency.**

```sh
gofmt -w internal/cache
go test -race ./internal/cache -count=1
go test ./internal/identity/usecase ./internal/problem/usecase ./internal/cultivation/usecase ./internal/material/usecase ./internal/di -count=1
go vet ./...
go test ./... -count=1
```

Kỳ vọng cache regression qua; existing commit/rollback invalidation tests qua; 64 copy-lock diagnostics của baseline biến mất. Nếu có lỗi khác, phân loại riêng thay vì tắt vet.

- [ ] **Step 4: Review patch.**

Kiểm tra cache key/TTL/body method không bị thay đổi, field loads chỉ có một owner sống cùng adapter, goroutine trong tests kết thúc. Ghi commands và kết quả vào audit. Không commit.

## Checkpoint lint cleanup

Sau task cache, golangci-lint có thể vẫn báo staticcheck từ baseline: import cùng package qua hai alias và tên `IdentityErrCampaignAudienceTooLarge`. Đây là task cleanup riêng: gộp import và cập nhật selector bằng rename có kiểm soát, rename exported internal error thành `ErrIdentityCampaignAudienceTooLarge` cùng toàn bộ call sites; chạy unit/architecture/linter. Không dùng regex toàn repo để đổi token không kiểm tra và không sửa generated code.

Task cache hoàn tất khi public same-key concurrency đúng, key khác độc lập, transaction invalidation giữ nguyên và vet không còn copy lock. Tình trạng linter còn lại phải được báo riêng; không gọi toàn bộ `make verify` xanh khi staticcheck còn lỗi.
