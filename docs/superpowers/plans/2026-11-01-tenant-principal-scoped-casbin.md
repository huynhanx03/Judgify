# Tenant- and principal-scoped Casbin runtime

## Goal
Keep authorization evaluation in process memory while avoiding global membership graphs in each Casbin enforcer. Keep the current durable global revision as the cache invalidation fence.

## Design
1. In go-common, extend the read-only Ent adapter filters so an application can load a scope's policy and role graph without loading user membership edges.
2. In go-common, add a request-scoped relation evaluation API. It overlays only the authenticated subject's g relations for the requested scope and g2 relations for the requested resource while Casbin evaluates, then removes the overlay. The matcher remains synchronous and database-free.
3. In Judgify, make the bounded scoped runtime cache load a real filtered runtime per tenant and revision. Keep a small system runtime for the global revision fence and system-scope policy.
4. Add a bounded LRU keyed by revision, scope, and user for memberships, plus resource-specific entries for g2 relations. On cache miss, load only that user's rows. Reuse memberships across resource IDs, confirm the durable revision around reads, and fail closed on mismatch.
5. Invalidate tenant runtimes and subject relations when a committed revision is published or reconciled. Stop retaining per-user role and effective-capability maps in the global authorization view; derive requested-user views from the relation cache.
6. Preserve existing transaction, audit, outbox, revision, and role mutation behavior.

## Work sequence
- Add scoped filter and dynamic relation support in go-common.
- Add repository methods for filtered tenant snapshots and one-user relations in Judgify.
- Wire bounded caches and revision fences through authorization startup, request evaluation, mutation publication, and reconciliation.
- Format touched Go files and inspect the final diff.

## Constraints
- No database work inside Casbin's matcher or role manager.
- No per-user enforcer copies.
- No cache entry survives a policy revision change.
- Preserve all unrelated working-tree changes; do not stage or commit.
