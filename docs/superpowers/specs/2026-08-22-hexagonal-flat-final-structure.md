# Judgify Final Hexagonal Structure

## Goal

Refactor the API into a domain-first modular monolith with visible Hexagonal
boundaries, flat feature files, root catalogs, and a focused PostgreSQL test
runtime.

## Decisions

- `internal/ports` and `internal/constant` are shared packages with exactly
  one non-test source file per domain.
- `internal/di` is the sole composition root. No business domain has a `di/`
  directory.
- A business domain contains only layer directories: `domain/`, `contract/`,
  `usecase/`, `adapters/`, and optionally test-only data. No loose source files
  sit at `internal/<domain>/`.
- `domain/` contains entities, value objects, and business rules. It imports
  no ports, adapters, Ent, infrastructure, or other business domains.
- `contract/` contains command, query, and result types used across the
  application boundary. It imports no ports, use cases, or adapters.
- `usecase/` orchestrates domain rules through root ports. Its mapper maps
  `domain` values to `contract` results only.
- HTTP uses `<feature>.go`, `<feature>_dto.go`, and `<feature>_mapper.go`.
  HTTP identifiers are strings and are parsed at this boundary.
- DB uses `<feature>.go`, `<feature>_builder.go`, and `<feature>_mapper.go`.
  DB maps Ent values to domain values and accepts internal `uuid.UUID` IDs.
- Generated GoMock doubles live only in `internal/testutil/mocks/<domain>.go`.
  Shared fixtures live in `internal/testutil/<domain>.go`.
- Unit tests run without Docker. PostgreSQL integration tests have the
  `integration` build tag and use one runner-managed PostgreSQL container per
  suite, based on the production-pinned image.

## Domain Shape

```text
internal/material/
  domain/material.go
  contract/material.go
  usecase/material.go
  usecase/material_mapper.go
  adapters/driver/http/material.go
  adapters/driver/http/material_dto.go
  adapters/driver/http/material_mapper.go
  adapters/driven/db/material.go
  adapters/driven/db/material_builder.go
  adapters/driven/db/material_mapper.go
```

## Boundary Flow

```text
HTTP DTO -> HTTP mapper -> contract command -> usecase -> domain
domain -> usecase mapper -> contract result -> HTTP mapper -> HTTP DTO
domain -> DB builder/mapper -> Ent/PostgreSQL
```

## Compatibility Constraints

- Preserve routes, JSON fields, HTTP statuses, authorization, migrations, and
  event names.
- Root port and internal IDs use `uuid.UUID`.
- Do not commit from the current dirty worktree.
