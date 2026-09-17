# Judgify Verification Baseline — 2026-07-18

Captured in the existing `refactor/cleanup` workspace without resetting, staging, committing, or modifying any acknowledged source change.

## Environment

| Tool/repository | Observed value |
|---|---|
| Platform | `darwin/arm64` |
| Go | `go1.26.4` |
| Node.js | `v24.18.0` |
| npm | `11.16.0` |
| Judgify HEAD | `cace718` |
| go-common HEAD | `293860a` |

## Command Outcomes

| Surface | Command | Exit/result |
|---|---|---|
| go-common | `cd ../go-common && go test ./...` | Exit `0` |
| Judgify API | `cd api && go test ./...` | Exit `0` |
| Frontend lint | `cd website && npm run lint` | Exit `1`; 34 findings: 27 errors and 7 warnings |
| Frontend production build | `cd website && npm run build` | Remained at `Creating an optimized production build ...` for several minutes and was manually interrupted; this is a non-terminal observation and is neither a pass nor a fail result |

The lint result was reproduced immediately before writing this document. The Go outcomes and build observation are the preserved baseline run recorded by the approved master plan.

## Frontend Lint Inventory

| File | Rule/category | Count/severity |
|---|---|---:|
| `website/app/(main)/about/page.tsx` | `react/no-unescaped-entities` | 6 errors |
| `website/app/(main)/materials/[id]/page.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/app/admin/(dashboard)/layout.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/app/admin/(dashboard)/roles/page.tsx` | `@typescript-eslint/no-unused-vars` | 1 warning |
| `website/components/theme-toggle.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/hooks/use-contest-sse.ts` | `react-hooks/immutability` | 1 error |
| `website/modules/admin/dialogs/contest-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/dialogs/difficulty-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/dialogs/element-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/dialogs/level-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/dialogs/material-category-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/dialogs/material-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/dialogs/rank-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/dialogs/rarity-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/dialogs/role-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/dialogs/tag-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/dialogs/trait-dialog.tsx` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/admin/hooks/use-paginated-crud.ts` | `@typescript-eslint/no-explicit-any` | 5 errors |
| `website/modules/admin/problem-form.tsx` | `@typescript-eslint/no-unused-vars` | 1 warning |
| `website/modules/admin/problem-form.tsx` | unused `react-hooks/exhaustive-deps` disable | 1 warning |
| `website/modules/arena/hooks/use-arena-filters.ts` | `react-hooks/set-state-in-effect` | 1 error |
| `website/modules/contest/contest-standings-table.tsx` | `@typescript-eslint/no-unused-vars` | 2 warnings |
| `website/modules/layout/header.tsx` | `@next/next/no-img-element` | 1 warning |
| `website/modules/ranking/ranking-spiritual-root-grid.tsx` | `@typescript-eslint/no-unused-vars` | 1 warning |

Totals:

- `react/no-unescaped-entities`: 6 errors.
- `react-hooks/set-state-in-effect`: 15 errors.
- `react-hooks/immutability`: 1 error.
- `@typescript-eslint/no-explicit-any`: 5 errors.
- `@typescript-eslint/no-unused-vars`: 5 warnings.
- unused ESLint disable: 1 warning.
- `@next/next/no-img-element`: 1 warning.

## Interpretation

- Go baselines are green at the recorded revisions.
- Frontend lint is an acknowledged red baseline and must be corrected without blanket rule suppression.
- The frontend build has no terminal baseline result. Its non-termination requires bounded diagnosis in the frontend foundation plan before any success or failure claim.
- `docs/superpowers/baseline/2026-07-18-worktree-status.txt` is the exact `git status --short --untracked-files=all` snapshot taken before this baseline document was added.

## Architecture Ratchet Baseline

The first focused run of:

```bash
cd api
GOCACHE=/private/tmp/judgify-go-build \
  go test ./internal/architecture -run 'Test(ModuleDependencies|APIRoleHasNoDocker)' -count=1
```

failed because `api/internal/architecture/testdata/known_violations.txt` did not yet exist and reported these exact sorted findings:

```text
api-role|github.com/huynhanx03/judgify/internal/infrastructure|github.com/huynhanx03/judgify/internal/judge/executor
api-role|github.com/huynhanx03/judgify/internal/judge|github.com/huynhanx03/judgify/internal/judge/executor
module|github.com/huynhanx03/judgify/internal/identity/adapters/driver/http|github.com/huynhanx03/judgify/internal/cultivation/core/entity
module|github.com/huynhanx03/judgify/internal/identity/adapters/driver/http|github.com/huynhanx03/judgify/internal/cultivation/ports
module|github.com/huynhanx03/judgify/internal/identity/core/service|github.com/huynhanx03/judgify/internal/cultivation/core/entity
module|github.com/huynhanx03/judgify/internal/identity/core/service|github.com/huynhanx03/judgify/internal/cultivation/ports
module|github.com/huynhanx03/judgify/internal/identity/core/service|github.com/huynhanx03/judgify/internal/problem/ports
module|github.com/huynhanx03/judgify/internal/identity/di|github.com/huynhanx03/judgify/internal/cultivation/ports
module|github.com/huynhanx03/judgify/internal/identity/di|github.com/huynhanx03/judgify/internal/problem/ports
```

Default architecture checks now require this exact set so no violation can be added, removed, or reordered without review. `JUDGIFY_ARCH_STRICT=1` intentionally rejects every row until later runtime/module slices remove all known debt.

## go-common Scylla Readiness Regression

The first complete go-common checkpoint outside the filesystem/network sandbox reproduced:

```text
pkg/database/widecolumn TestClient_Integration
gocql: unable to create session: unable to discover protocol version:
read tcp 127.0.0.1:<ephemeral>->127.0.0.1:<ephemeral>: read: connection reset by peer
```

A second focused run reproduced the same failure. Testcontainers declared the Scylla container ready roughly two seconds after startup because the test waited only for the early `Scylla version` banner; the CQL protocol was not accepting sessions yet.

The test readiness strategy now retains the banner check and additionally polls:

```text
cqlsh -e "SELECT release_version FROM system.local;"
```

with the existing two-minute deadline. Evidence after the change:

- focused `TestClient_Integration`: exit `0`, all five CRUD subtests passed;
- complete `make test-common`: exit `0`;
- no arbitrary sleep was introduced.
