# Judgify Backend Architecture Guide

> Hướng dẫn kiến trúc và quy ước code cho backend. Mục tiêu: bất kỳ ai join project cũng có thể đọc guide này và code đúng pattern từ ngày đầu.

---

## 1. Kiến trúc tổng quan

Judgify BE dùng **Hexagonal Architecture** (Ports & Adapters), chia theo **domain module**. Mỗi domain là 1 bounded context độc lập.

```
api/
├── cmd/server/main.go          # Entry point
├── config/                     # Config files (local.yaml, production.yaml)
├── internal/
│   ├── infrastructure/         # Bootstrap: config, DB, router, server
│   ├── di/                     # Dependency injection (root container)
│   ├── constant/               # Shared constants (MQ topics)
│   ├── ent/                    # Ent ORM schemas + generated code
│   │   ├── schema/             # Schema definitions (source of truth)
│   │   ├── mixin/              # Shared mixins (soft delete)
│   │   └── generate/           # Auto-generated code (DO NOT EDIT)
│   │
│   │── contest/                # Domain module (ví dụ mẫu)
│   │   ├── adapters/
│   │   │   ├── driver/http/    # Driving adapters (HTTP handlers)
│   │   │   └── driven/db/      # Driven adapters (DB repositories)
│   │   ├── core/
│   │   │   ├── dto/            # Request/Response structs
│   │   │   ├── entity/         # Domain entities (pure Go structs)
│   │   │   ├── mapper/         # Entity <-> DTO mapping
│   │   │   └── service/        # Business logic
│   │   ├── ports/              # Interfaces (contracts)
│   │   ├── constant/           # Domain constants
│   │   ├── di/                 # Domain DI container
│   │   └── store/              # Domain-specific stores (SSE hub, etc.)
│   │
│   ├── identity/               # Auth & user module
│   ├── problem/                # Problems & test cases
│   ├── submission/             # Code submissions
│   ├── cultivation/            # Gamification (XP, elements, gacha)
│   └── material/               # Learning materials
│
├── pkg/                        # Shared packages
│   ├── common/
│   │   ├── http/
│   │   │   ├── handler/        # Generic handler wrapper (Wrap[T,R])
│   │   │   ├── middlewares/    # Auth, CORS, rate limit, circuit breaker
│   │   │   ├── request/        # Request parsing & validation
│   │   │   ├── response/       # Response codes, messages, wrapper
│   │   │   └── validation/     # go-playground/validator wrapper
│   │   ├── apperr/             # AppError type
│   │   └── cache/              # Local cache (Ember)
│   ├── database/ent/           # Shared Ent helpers (pagination, filters, mixins)
│   ├── dto/                    # Shared DTOs (QueryOptions, Paginated)
│   ├── mq/forge/               # Message queue broker
│   ├── permissions/            # RBAC permission mapping
│   ├── constraints/            # Context keys, header constants
│   ├── utils/                  # JWT, crypto helpers
│   ├── logger/                 # Zap logger wrapper
│   ├── settings/               # Config struct definitions
│   └── algorithm/              # Circuit breaker, etc.
│
├── global/                     # Global singletons (Config, EntClient, Logger, Cache)
└── storages/                   # Local storage (MQ data)
```

### Data flow

```
HTTP Request
  → Gin middleware (auth, rate limit, circuit breaker)
  → handler.Wrap[T,R]() — parse request, validate, call handler
  → Handler (driver adapter) — extract context, call service via port interface
  → Service (core) — business logic, uses repository via port interface
  → Repository (driven adapter) — Ent queries, maps to domain entity
  → Ent ORM → PostgreSQL
```

---

## 2. Cách tạo 1 domain module mới

Giả sử cần thêm module `notification`. Tạo cấu trúc:

```
internal/notification/
├── adapters/
│   ├── driver/
│   │   └── http/
│   │       ├── handler.go          # HandlerGroup + RegisterPublic/RegisterProtected
│   │       └── notification.go     # Handler interface + implementation
│   └── driven/
│       └── db/
│           ├── notification.go     # Repository implementation
│           ├── builder/
│           │   └── notification.go # Ent create/update builders
│           └── mapper/
│               └── notification.go # Ent model → domain entity
├── core/
│   ├── dto/
│   │   └── notification.go         # Request/Response DTOs
│   ├── entity/
│   │   └── notification.go         # Domain entity (pure struct)
│   ├── mapper/
│   │   └── notification.go         # Entity → DTO mapping
│   └── service/
│       └── notification.go         # Business logic
├── ports/
│   └── notification.go             # Repository + Service interfaces
├── constant/
│   └── constant.go                 # Domain constants
├── di/
│   └── di.go                       # Domain DI container
└── store/                          # Optional: SSE hub, etc.
```

### Bước implementation

1. **Ent schema** (`internal/ent/schema/notification.go`)
2. **Generate code**: `go generate ./internal/ent`
3. **Entity** (`core/entity/`) — pure Go struct, không import Ent hay Gin
4. **DTO** (`core/dto/`) — request/response với `json` + `validate` tags
5. **Ports** (`ports/`) — định nghĩa interfaces cho Repository và Service
6. **Service** (`core/service/`) — implement business logic qua port interfaces
7. **Mapper** (`adapters/driven/db/mapper/`) — Ent model → Entity
8. **Builder** (`adapters/driven/db/builder/`) — Ent create/update mutations
9. **Repository** (`adapters/driven/db/`) — implement port interface, gọi Ent
10. **Core mapper** (`core/mapper/`) — Entity → DTO
11. **Handler** (`adapters/driver/http/`) — implement handler interface
12. **DI** (`di/di.go`) — wire everything
13. **Register routes** trong `infrastructure/router.go`

---

## 3. Quy ước code chi tiết

### 3.1 Entity (`core/entity/`)

Pure Go struct, **KHÔNG** import Ent hay framework.

```go
package entity

import "time"

type Contest struct {
    ID              int
    Title           string
    Description     string
    StartTime       time.Time
    EndTime         time.Time
    Status          string
    AuthorID        int
    MaxParticipants int
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

### 3.2 DTO (`core/dto/`)

Tách biệt Request và Response. Dùng `validate` tags cho request, `json` tags cho cả 2.

```go
package dto

// Request — validate với go-playground
type CreateContestRequest struct {
    Title       string    `json:"title" validate:"required,max=300"`
    StartTime   time.Time `json:"start_time" validate:"required"`
    ProblemIDs  []int     `json:"problem_ids"`
}

// Update dùng pointer cho partial update
type UpdateContestRequest struct {
    ID          int        `json:"-" uri:"id"`
    Title       *string    `json:"title,omitempty"`
    Description *string    `json:"description,omitempty"`
}

// Response — trả về cho client
type ContestResponse struct {
    ID      int    `json:"id"`
    Title   string `json:"title"`
    Status  string `json:"status"`
}
```

### 3.3 Ports (`ports/`)

Định nghĩa interfaces. Repository dùng entity, Service dùng DTO.

```go
package ports

// Repository interface — works với domain entities
type ContestRepository interface {
    Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Contest], error)
    Get(ctx context.Context, id int) (*entity.Contest, error)
    Create(ctx context.Context, e *entity.Contest) error
    Update(ctx context.Context, e *entity.Contest) error
    Delete(ctx context.Context, id int) error
}

// Service interface — works với DTOs
type ContestService interface {
    Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.ContestResponse], error)
    Get(ctx context.Context, id int, userID int) (*dto.ContestResponse, error)
    Create(ctx context.Context, authorID int, req *dto.CreateContestRequest) (*dto.ContestResponse, error)
}
```

### 3.4 Service (`core/service/`)

Implement business logic. Chỉ phụ thuộc vào port interfaces, **KHÔNG** import Ent hay Gin.

```go
package service

type contestService struct {
    contestRepo ports.ContestRepository
    regRepo     ports.RegistrationRepository
}

func NewContestService(contestRepo ports.ContestRepository, regRepo ports.RegistrationRepository) ports.ContestService {
    return &contestService{contestRepo: contestRepo, regRepo: regRepo}
}

func (s *contestService) Create(ctx context.Context, authorID int, req *dto.CreateContestRequest) (*dto.ContestResponse, error) {
    // 1. Validate business rules
    if !req.StartTime.Before(req.EndTime) {
        return nil, apperr.New(response.CodeBadRequest, "start_time must be before end_time", nil)
    }

    // 2. Map DTO → Entity
    e := &entity.Contest{
        Title:     req.Title,
        Status:    constant.StatusDraft,
        AuthorID:  authorID,
    }

    // 3. Persist
    if err := s.contestRepo.Create(ctx, e); err != nil {
        return nil, err
    }

    // 4. Map Entity → DTO response
    return mapper.ToContestResponse(e, 0, nil), nil
}
```

### 3.5 Repository (`adapters/driven/db/`)

Implement port interface, dùng Ent trực tiếp. Luôn map Ent error qua `commonEnt.MapEntError()`.

```go
package db

type ContestRepository struct {
    client *dbEnt.EntClient
}

func NewContestRepository(client *dbEnt.EntClient) ports.ContestRepository {
    return &ContestRepository{client: client}
}

func (r *ContestRepository) Get(ctx context.Context, id int) (*entity.Contest, error) {
    record, err := r.client.DB(ctx).Contest.Get(ctx, id)
    if err != nil {
        return nil, commonEnt.MapEntError(err, "Contest")
    }
    return mapper.ToContestEntity(record), nil
}

func (r *ContestRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Contest], error) {
    client := r.client.DB(ctx)
    query := client.Contest.Query()

    if opts != nil {
        query.Where(func(s *sql.Selector) {
            commonEnt.ApplyFilters(opts.Filters, s)
        })
    }

    total, err := query.Clone().Count(ctx)
    if err != nil {
        return nil, commonEnt.MapEntError(err, "Contest")
    }

    if opts != nil {
        query.Where(func(s *sql.Selector) {
            commonEnt.ApplySort(opts.Sort, s)
            commonEnt.ApplyPagination(opts.Pagination, s)
        })
    }

    records, err := query.All(ctx)
    // ... map to entities + pagination
}
```

### 3.6 Mapper layer

2 loại mapper, ở 2 nơi khác nhau:

**DB Mapper** (`adapters/driven/db/mapper/`): Ent model → Domain entity
```go
func ToContestEntity(m *generate.Contest) *entity.Contest {
    if m == nil { return nil }
    return &entity.Contest{
        ID:    m.ID,
        Title: m.Title,
        // ...
    }
}
```

**Core Mapper** (`core/mapper/`): Entity → DTO response
```go
func ToContestResponse(e *entity.Contest, count int, ids []int) *dto.ContestResponse {
    if e == nil { return nil }
    return &dto.ContestResponse{
        ID:               e.ID,
        ParticipantCount: count,
        // ...
    }
}
```

### 3.7 Builder (`adapters/driven/db/builder/`)

Tách Ent mutation building ra khỏi repository, giúp repository gọn.

```go
func BuildCreateContest(ctx context.Context, e *entity.Contest) *generate.ContestCreate {
    return global.EntClient.DB(ctx).Contest.Create().
        SetTitle(e.Title).
        SetStatus(contest.Status(e.Status))
}
```

### 3.8 Handler (`adapters/driver/http/`)

Handler định nghĩa interface và implement. Mỗi method nhận `(ctx context.Context, req *T) (R, error)`.

```go
// Interface — để domain DI có thể inject
type ContestHandler interface {
    Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.ContestResponse], error)
    Get(ctx context.Context, req *dto.GetContestRequest) (*dto.ContestResponse, error)
    Create(ctx context.Context, req *dto.CreateContestRequest) (*dto.ContestResponse, error)
}

// Implementation
type contestHandler struct {
    handler.BaseHandler
    contestService ports.ContestService
}

func NewContestHandler(svc ports.ContestService) ContestHandler {
    return &contestHandler{contestService: svc}
}

func (h *contestHandler) Get(ctx context.Context, req *dto.GetContestRequest) (*dto.ContestResponse, error) {
    userID := extractUserID(ctx)
    return h.contestService.Get(ctx, req.ID, userID)
}
```

### 3.9 HandlerGroup + Route Registration

Mỗi domain có 1 `HandlerGroup` với `RegisterPublic()` và `RegisterProtected()`.

```go
type ContestHandlerGroup struct {
    ContestHandler      ContestHandler
    RegistrationHandler RegistrationHandler
    StandingHandler     *standingHandler
    RatingHandler       RatingHandler
}

func (h *ContestHandlerGroup) RegisterPublic(r *gin.RouterGroup) {
    contests := r.Group("/contests")
    {
        contests.POST("/find", handler.Wrap(h.ContestHandler.Find))
        contests.GET("/:id", handler.Wrap(h.ContestHandler.Get))
    }
}

func (h *ContestHandlerGroup) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.PermissionChecker) {
    contests := r.Group("/contests")
    {
        contests.POST("",
            permChecker.RequirePermission(permissions.ResourceKeyContest, permissions.PermissionScopeCreate),
            handler.Wrap(h.ContestHandler.Create),
        )
    }
}
```

Dùng `handler.Wrap[T, R]()` để tự động parse request, validate, và format response.

### 3.10 DI Container

**Domain container** (`internal/<domain>/di/di.go`):

```go
type ContestContainer struct {
    ContestHandlerGroup *contestHttp.ContestHandlerGroup
    ContestRepo         ports.ContestRepository
    ContestService      ports.ContestService
    // ...
}

func NewContestContainer(userStatsRepo cultivationPorts.UserStatsRepository) *ContestContainer {
    client := global.EntClient

    // Repos
    contestRepo := db.NewContestRepository(client)

    // Services
    contestService := service.NewContestService(contestRepo, regRepo)

    // Handlers
    handlerGroup := &contestHttp.ContestHandlerGroup{
        ContestHandler: contestHttp.NewContestHandler(contestService),
    }

    return &ContestContainer{
        ContestHandlerGroup: handlerGroup,
        ContestRepo:         contestRepo,
        ContestService:      contestService,
    }
}
```

**Root container** (`internal/di/wire.go`): wire tất cả domain containers.

### 3.11 Thêm routes

Sửa `infrastructure/router.go`:

```go
// 1. Thêm handler vào RouterGroup struct
type RouterGroup struct {
    // ...existing handlers
    NotificationHandler *notificationHttp.NotificationHandlerGroup
}

// 2. Thêm vào constructor NewRouterGroup()

// 3. Đăng ký trong registerRoutes()
func (rg *RouterGroup) registerRoutes(r *gin.Engine) {
    publicAuth := r.Group("/")
    // ...
    rg.NotificationHandler.RegisterPublic(publicAuth)

    protected := r.Group("/")
    // ...
    rg.NotificationHandler.RegisterProtected(protected, rg.PermChecker)
}
```

---

## 4. Request/Response Flow

### Generic Handler Wrapper

`handler.Wrap[T, R]` tự động hóa toàn bộ flow:

```
1. ParseRequest[T]()  → bind URI + JSON + validate
2. h(ctx, req)        → gọi handler method
3. SuccessResponse()  → format {code, message, data}
   hoặc ErrorResponse() → format {code, message, data: null}
```

### Error handling pattern

Dùng `apperr.New(code, message, cause)` cho business errors:

```go
// Trong service
return nil, apperr.New(response.CodeBadRequest, "start_time must be before end_time", nil)
return nil, apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, "contest"), nil)

// Wrapper tự map AppError.Code → HTTP status qua GetHTTPCode()
```

### Response codes

| Range | Meaning | HTTP Status |
|-------|---------|-------------|
| 20000-29999 | Success | 200/201 |
| 40000-40999 | Client error | 400 |
| 41000-41999 | Auth error | 401 |
| 43000-43999 | Forbidden | 403 |
| 44000-44999 | Not found | 404 |
| 49000-49999 | Conflict | 409 |
| 50000-59999 | Server error | 500 |

---

## 5. Ent ORM Conventions

### Schema

```go
// internal/ent/schema/contest.go
type Contest struct {
    ent.Schema
}

func (Contest) Mixin() []ent.Mixin {
    return []ent.Mixin{
        e.TimeMixin{},          // created_at, updated_at
        mixin.SoftDeleteMixin{}, // deleted_at
    }
}

func (Contest) Fields() []ent.Field { /* ... */ }
func (Contest) Indexes() []ent.Index { /* ... */ }
func (Contest) Edges() []ent.Edge { /* ... */ }
```

### Generate

```bash
go generate ./internal/ent
```

**QUAN TRỌNG**: Không bao giờ sửa code trong `internal/ent/generate/` — tất cả là auto-generated.

---

## 6. Middleware Stack

Middleware được apply theo thứ tự trong `NewEngine()`:

1. `RecoveryMiddleware` — panic recovery
2. `RequestLogger()` — log mọi request
3. `CORSMiddleware` — CORS headers
4. Global `RateLimit` — rate limiting cho tất cả routes
5. `CircuitBreakerMiddleware` — circuit breaker pattern
6. `Authentication` (protected routes) — JWT RSA verification
7. `PermissionChecker` (per route) — RBAC check

---

## 7. Authentication & Authorization

### JWT

- RSA key pair (configurable path trong config)
- Token sign bằng RSA private key, verify bằng public key
- Claims: `UserID`, `Username`
- Middleware set vào `context` qua `constraints.ContextKeyUserID`, `constraints.ContextKeyUsername`

### RBAC

- Resources load từ DB table `resources` vào map lúc startup (`Initialized()`)
- Middleware `RequirePermission(resourceKey, scope)` check permission
- Permission cached trong local cache (`Ember`)

---

## 8. Message Queue

Dùng internal Forge MQ broker (`pkg/mq/forge/`):

```go
// Tạo broker
broker, _ := forge.NewBroker("./storages/mq")

// Producer — gửi message
producer, _ := broker.NewProducer("topic-name")
producer.Publish(ctx, message)

// Consumer — nhận message
consumer, _ := broker.NewConsumer("worker-name", "topic-name")
```

Các topic hiện tại:
- `judge` — submission judging
- `contest-judge` — contest submission judging
- `exp-reward` — EXP reward events

---

## 9. Background Workers

Khởi tạo trong `infrastructure/run.go`:

- **Judge Worker**: nhận submission từ MQ, chạy code trong Docker sandbox, trả kết quả
- **EXP Reward Worker**: nhận event khi solve problem, tính EXP và update stats
- **Contest Orchestrator**: tick mỗi 30s, check và update contest status
- **Scheduler**: CRON job cho stats recalculation

---

## 10. Config Management

- Viper load từ `config/<env>.yaml` (`GO_ENV` env var, default: `local`)
- Merge với `.env` file
- Unmarshal vào `global.Config` (struct defined trong `pkg/settings/`)
- Global singletons: `global.Config`, `global.EntClient`, `global.LoggerZap`, `global.Ember`

---

## 11. Quy tắc vàng

| Quy tắc | Giải thích |
|---------|-----------|
| **Core không import adapter** | `core/` không biết gì về Ent, Gin, MQ. Chỉ dùng ports. |
| **Entity không có tag** | `core/entity/` là pure Go struct, không json/validate tags. |
| **DTO có tag** | `core/dto/` có `json` + `validate` tags. |
| **Port ở giữa** | `ports/` định nghĩa interface, core import ports, adapter implements ports. |
| **1 file = 1 concept** | `contest.go` cho contest, `registration.go` cho registration. |
| **Null check trong mapper** | Luôn check `if m == nil { return nil }` trong mapper functions. |
| **MapEntError luôn** | Repository luôn wrap Ent error qua `commonEnt.MapEntError()`. |
| **Wrap handler** | Luôn dùng `handler.Wrap()` thay vì viết manual Gin handler. |
| **apperr cho business error** | Service luôn trả `apperr.New()`, không bao giờ trả raw error. |
| **Logger từ context** | Dùng `logger.FromContext(ctx)` thay vì `global.LoggerZap` trong service. |

---

## 12. Checklist: Thêm API endpoint mới

- [ ] Thêm/update Ent schema nếu cần
- [ ] `go generate ./internal/ent`
- [ ] Thêm entity field nếu cần (`core/entity/`)
- [ ] Thêm DTO request/response (`core/dto/`)
- [ ] Thêm/update port interface (`ports/`)
- [ ] Implement service method (`core/service/`)
- [ ] Thêm DB mapper (`adapters/driven/db/mapper/`)
- [ ] Thêm builder nếu create/update (`adapters/driven/db/builder/`)
- [ ] Implement repository method (`adapters/driven/db/`)
- [ ] Thêm core mapper (`core/mapper/`)
- [ ] Implement handler method (`adapters/driver/http/`)
- [ ] Đăng ký route trong HandlerGroup
- [ ] Update DI container nếu có dependency mới
- [ ] Update `router.go` nếu là domain mới
