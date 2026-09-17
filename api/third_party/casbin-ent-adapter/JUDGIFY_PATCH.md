# Judgify patch to casbin/ent-adapter v1.4.0

This directory is an unmodified copy of upstream `github.com/casbin/ent-adapter`
at tag `v1.4.0` except for two constructor options in `adapter.go`:

- `SkipSchemaCreate()` keeps schema ownership in Atlas migrations and permits
  API credentials without DDL privileges.
- `WithContext(ctx)` binds startup reads to the caller's cancellation budget.
- `WithPolicyLimit(n)` bounds loaded rows with a sentinel query and rejects
  oversized policy snapshots instead of silently truncating them.

Policy writes still belong to Judgify's transaction-aware repository; Casbin
AutoSave is disabled. Keep the module replacement pinned until upstream offers
an equivalent no-DDL constructor.
