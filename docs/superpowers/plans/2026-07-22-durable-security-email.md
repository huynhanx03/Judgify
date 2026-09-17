# Durable security-email implementation plan

1. Replace password-reset after-commit delivery with a transaction-bound scheduling port and
   prove rollback/commit behavior in service tests.
2. Add a versioned, bounded AEAD payload codec and tamper/expiry/key-id tests.
3. Implement the Ent delivery repository: enqueue, SKIP LOCKED claim/reclaim, fenced sent/retry/
   dead transitions, and ciphertext erasure.
4. Implement a lifecycle dispatcher with bounded concurrency, timeouts, exponential backoff,
   shutdown, CID-aware logs, and health state.
5. Pass stable idempotency keys to the provider adapter and test the request contract.
6. Wire settings, DI, API lifecycle, health, schema invariants, and migration inventory.
7. Run identity/infrastructure tests with race detection and vet; include the schema in the final
   consolidated migration baseline.

