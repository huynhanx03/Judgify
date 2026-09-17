# Production migration contract

`cmd/migrate` is the only production schema writer. API and judge-worker
startup never call Ent auto-migration and do not embed the checked-in SQL
directory.

## Safety model

- SQL files and `atlas.sum` are embedded in the operator binary. The checksum,
  canonical filenames, versions, size limits, and PostgreSQL statement parse
  are verified before any product DDL runs.
- A PostgreSQL session advisory lock serializes operators. The lock connection
  is kept separate from the one transaction connection and is always released
  before the pool closes; closing its owning session is the final release path
  after cancellation.
- A database with no revision table must be clean before the runner creates
  operational metadata. There is no `allow-dirty` or implicit baseline mode.
- Applied database revisions must form a linear prefix of the embedded chain.
  File hashes, descriptions, statement totals, and partial statement hashes
  must match exactly. Drift stops the run and is never repaired automatically.
- Each file runs in one PostgreSQL transaction together with its revision
  writes. A commit therefore contains both the DDL and ledger state, or neither.
  Checkpoints and non-transactional (`atlas:txmode none`) files are rejected.
- The whole run, database connection, lock wait, and every query are bounded.
  Logs carry CID and migration coordinates but never raw SQL, DSNs, passwords,
  or revision failure statements.

## Operator workflow

1. Generate a new versioned diff with `make migrate-diff name=<description>`.
2. Review the SQL, destructive changes, and regenerated `atlas.sum`.
3. Never edit or reorder a migration already applied to any environment.
4. Build and run the role-pure artifact: `go build ./cmd/migrate`.

`make migrate-apply` uses the same production runner. `make migrate-status`
is read-only and requires an explicit `DB_URL` environment/make variable.
