# Hexagonal Flat Refactor Baseline

Date: 2026-08-22

## Execution location

- Primary checkout: `/Users/jerry/nhanhmc/coding/Judgify` on `refactor/cleanup`.
- Isolated checkout created at `/Users/jerry/nhanhmc/coding/judgify-hexagonal-flat`
  on `refactor/hexagonal-flat`.
- The isolated checkout does not contain the untracked architecture source from
  the primary checkout, so the user explicitly authorized direct work in the
  primary dirty checkout. No commits are made from that checkout.

## Initial architecture baseline

The initial focused command was:

```sh
cd api && go test ./internal/architecture
```

It initially failed because `internal/architecture/testdata/known_violations.txt`
was absent and `runtime_roles_test.go` assumed a removed legacy `global/`
directory existed. The test now treats that directory as optional and the
baseline fixture exists. The command passes.

## Current verified checkpoint

The following commands passed after introducing root catalog files, migrating
Identity constants, and moving the Identity campaign-audience cross-domain port:

```sh
go test ./internal/architecture
go test ./internal/constant ./internal/identity/... ./internal/operator/bootstrapadmin
go test ./internal/ports ./internal/identity/adapters/driven/db \
  ./internal/application/workflow/notificationcampaign ./internal/notification/...
go test ./...
```

This is a migration checkpoint, not a declaration that the full architectural
refactor is complete.
