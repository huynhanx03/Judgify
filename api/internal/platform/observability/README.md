# Judgify runtime metrics contract

This package owns Judgify-specific metric names, fixed label vocabularies and
runtime wiring. `go-common/pkg/common/observability` remains product-neutral and
provides only the bounded registry and exposition transport.

## Collection model

- Transition counters are recorded at the code path that owns the transition.
- Durable gauges are refreshed from PostgreSQL immediately before a scrape.
- Process, WebSocket and worker gauges are read from lock-safe aggregate
  snapshots. IDs, CIDs, paths, SQL, payloads and error text are never labels.
- The worker-owned solve-reward Forge bridge exports only fixed `main`/`dlq`
  storage and segment classes plus aggregate consumer lag and retained DLQ
  records. Configured topic/group names and event identities are never labels.
- One process permits at most 32 named collectors. A scrape has one shared
  two-second context deadline. Concurrent scrapes do not start duplicate
  database work; they export the registry's current bounded state while the
  in-flight collector owns the refresh.
- Only a valid `GET` exposition request runs collectors. Snapshot-backed
  counters advance their process-local checkpoint only after the corresponding
  registry update succeeds, so cancellation or a failed scrape cannot silently
  discard a delta. Counter resets are treated as a new cumulative epoch.
- Administrative operation kinds and states are folded to the fixed catalog in
  PostgreSQL before rows cross the collector boundary. The query returns at
  most `(registered kinds + other) * six states`, including with legacy data.
- Health outcomes preserve dependency criticality: optional failures are
  `degraded`, critical failures are `unhealthy`, and deadlines use each
  dependency's configured timeout.

## Database coverage

`bootstrap.Process` wraps the Ent dialect driver once. Generated Ent queries,
custom `QueryContext`/`ExecContext` calls made through generated entity clients,
and Ent transactions therefore emit the closed `read`, `write`, `transaction`
or `other` latency classes.

There is an intentional, currently visible coverage boundary: adapters that
borrow the concrete `*sql.DB` through `EntClient.DialectDriver` bypass the Ent
wrapper. This includes the judge lease/finalization repositories and other
callers that require raw PostgreSQL transactions. Scrape-time collector SQL is
also deliberately unobserved, preventing the metrics scrape from measuring
itself. `judgify_database_query_duration_seconds` must therefore be read as
"Ent-driver calls", not as complete PostgreSQL traffic. Pool saturation and
durable queue gauges still use the shared underlying pool.

`judgify_database_pool_connections{state="wait_count"}` is the standard
library's cumulative `DBStats.WaitCount`; Go does not expose a current-waiter
gauge. Dashboards must use a rate/delta for pressure and must not describe that
series as instantaneous waiters.

Closing that gap requires a separately designed observed raw-SQL boundary (or
repository-owned fixed operation observers); it must not be approximated by
putting SQL text, table names or caller-provided values into labels.

## Host-owned metrics

The API and judge-worker processes emit only storage they can truthfully
observe. The worker emits `judgify_host_disk_available_bytes{storage="judge_staging"}`.
The host collector/operator owns `storage="database"` and
`storage="backup_staging"`, plus backup and restore success/failure metrics.
Application processes must not be granted database-host filesystem access just
to fill those series.
