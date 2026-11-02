# Durable contest rating implementation plan

1. Add failing service tests for completion, rollback-safe failures, idempotency, and retrying
   already-ended contests.
2. Add the settlement port and `rating_completed_at` schema/index.
3. Implement the Ent repository with bounded pending scans, transaction-required row locking,
   and guarded completion.
4. Wire settlement into rating service, orchestrator, DI, logs, and tests.
5. Regenerate Ent and mocks once coordinated schema changes are stable.
6. Run contest tests with race detection and vet; regenerate the migration baseline only after
   all concurrent schema work is finished.

