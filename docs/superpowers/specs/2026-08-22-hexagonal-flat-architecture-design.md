# Judgify Hexagonal Flat Architecture Design

## Goal

Refactor the Go API into a domain-first modular monolith with explicit Hexagonal boundaries, a single internal port catalog, and flat feature files within every package.

## Decisions

- `internal/ports` is one Go package with one file per producer domain: `identity.go`, `problem.go`, `submission.go`, `contest.go`, `cultivation.go`, `material.go`, `notification.go`, `judge.go`, and `platform.go`.
- `internal/constant` follows the same one-file-per-domain layout. Exported names are domain-prefixed because all files share package scope.
- Domain models use `uuid.UUID` internally. HTTP path/query/JSON identifiers are strings and are parsed/formatted only by HTTP adapters.
- The current `core` layer is removed. It becomes `domain` (pure business model and rules) plus `usecase` (commands/queries that use ports and transactions).
- Domain packages do not import `ports`, adapters, DI, infrastructure, Ent, Gin, or other business domains.
- `internal/application` owns workflows that coordinate more than one domain. A domain never imports another domain's use case or repository.
- `adapters/driver/http` owns HTTP DTOs and DTO mappers. `adapters/driven/db` owns Ent repositories, Ent mappers, and Ent mutation builders.
- Adapter packages are flat by feature. A feature starts as one file; the preferred permanent split is `<feature>_handler.go`, `<feature>_dto.go`, `<feature>_mapper.go` for HTTP and `<feature>_repository.go`, `<feature>_mapper.go`, `<feature>_builder.go` for DB. Do not create `dto/`, `mapper/`, or `builder/` directories.
- `internal/di` is the only composition layer. It contains one file per domain plus API and worker role composition. `internal/infrastructure` owns process/bootstrap/server mechanics.
- `internal/judge` remains a dedicated worker/sandbox component. `realtime` is treated as a platform delivery capability; onboarding and profile composition live under `internal/application`.

## Target Tree

```text
internal/
  application/{onboarding,submissionintake,notificationcampaign,query}
  architecture/
  constant/{identity,problem,submission,contest,cultivation,material,notification,judge,platform}.go
  di/{container,platform,identity,problem,submission,contest,cultivation,material,notification,api,worker}.go
  ent/
  infrastructure/{bootstrap,configuration,httpserver,runtime,worker,health,logging,observability,operator}
  judge/
  platform/{audit,authctx,contentrender,eventdelivery,eventing,operation,realtime,transaction}
  ports/{identity,problem,submission,contest,cultivation,material,notification,judge,platform}.go
  testutil/
  <business-domain>/{domain,usecase,adapters/driver/http,adapters/driven/db}
```

## Boundary Data Flow

```text
HTTP request DTO (string IDs)
  -> HTTP mapper
  -> port command/result (uuid.UUID IDs)
  -> use case
  -> domain entity/rules
  -> root port
  -> DB adapter mapper/builder
  -> Ent/PostgreSQL
```

Read-only list/dashboard endpoints may use a DB projection through a focused query use case; they do not need to hydrate a full domain aggregate.

## Test Model

- Domain tests use `testing` table tests and no persistence mocks.
- Use-case tests use small fakes by default and generated GoMock mocks only for meaningful interaction/order assertions.
- DB adapters use PostgreSQL integration tests; HTTP adapters use `httptest` contract tests.
- Generated mocks are test-only under `internal/testutil/mocks` or beside the consumer test, never production domain packages.
- Architecture tests enforce the dependency direction above before each migration milestone.

## Migration Constraints

- Preserve external HTTP routes, JSON contracts, authorization semantics, migrations, and event names unless a separate compatibility decision is approved.
- Every milestone must leave `go test ./...` or its documented focused equivalent runnable.
- Move one bounded vertical slice at a time; never mass-move every domain before a pilot verifies the conventions.
- The current worktree is dirty. Refactor commits must be created only in a clean, isolated worktree after the user selects execution.
