# Shared Cache Coordinator and Lease Fence

## Goal

Unify the cache-aside policy used by `go-common` local and remote helpers, and prevent a loader that began before invalidation from repopulating a cache with stale data. Keep the design ready for Judgify's L1 local cache and L2 Redis without adding cross-replica L1 invalidation.

## Current State

`go-common/pkg/common/cache/local_helper.go` and `remote_helper.go` independently implement typed reads, cache-aside loading, `singleflight`, TTL jitter, XFetch refresh, negative caching, and internal envelopes. A successful loader writes without checking whether the key was invalidated while it ran. `LocalCache` only exposes unconditional `Set` and `Delete`; `CacheEngine` likewise has no conditional cache-fill contract. The Redis engine implements the current remote interface.

Judgify's cache adapters call `commoncache.Fetch` and invalidate after the owning transaction commits. That transaction ordering remains the right policy and will not be moved into the cache coordinator.

## Chosen Design

Keep one context-first local entry point (`Fetch`) and the remote entry point (`FetchRemote`), both delegating to one generic cache-aside coordinator in `pkg/common/cache`. The loader always receives the caller's context. Remove the redundant contextless `Fetch` wrapper and `GetOrLoad` alias. The coordinator owns the shared policy: initial read, local `singleflight`, second read after entering the flight, loader invocation, TTL jitter, negative caching, XFetch refresh, and conditional fill.

Use an opaque, per-key **generation token** rather than a permanent `key + ":version"` counter exposed as an ordinary cache entry. A loader snapshots the current token; concurrent loaders may share it, so this is not a distributed lock. A loader may populate its tier only if its token remains current. Invalidation rotates the token and removes the value. The check-and-store operation must be atomic within each tier. A missing/evicted token always rejects a fill carrying an older token; it is never interpreted as a reusable default generation.

- The local Ristretto adapter stores generation tokens under an internal namespace and synchronizes token rotation, conditional fill, and value deletion. Token eviction is fail-closed for in-flight fills and needs no unbounded side map.
- The remote Redis adapter stores generation tokens in its internal namespace. A Redis Lua script atomically checks the token and stores the value; invalidation atomically rotates the token and deletes the value. Tokens may expire or be evicted: a fill with an absent or different token is rejected, so metadata need not be permanent.
- L1 and L2 use the same coordinator policy but retain tier-specific atomic storage operations. No Redis Pub/Sub, Redis Streams, Redis Client Tracking, or replica-to-replica L1 invalidation is added in this change.

The critical ordering is:

```text
cache miss -> snapshot generation token -> load source -> SetIfVersion(token)
mutation commit -> rotate generation token + delete value
```

If invalidation happens before the conditional store, the stale fill is rejected. If the conditional store happens first, the later invalidation deletes it. A request already in flight may still return the value it loaded; the guarantee is that this result cannot poison the cache for subsequent requests.

## API and Compatibility

Cache consumers continue to call `Fetch`/`FetchRemote`; invalidation uses an explicit version-aware helper such as `DeleteWithVersion`. Tokens remain internal to `go-common`; use cases and Judgify cache adapters do not construct or persist them. Keep the existing broad `LocalCache` and `CacheEngine` interfaces source-compatible; add narrow version-aware capabilities for the coordinator and implement them in the Ristretto and Redis adapters. A backend without the required capability must return an explicit unsupported-capability error from the versioned fetch path; it must never silently fall back to an unconditional fill.

Remote cache outages remain cache failures, not source-of-truth failures: reads fall back to the loader and return its result; cache write failure does not discard a successful source result. Version-aware invalidation failure remains an error so callers can observe it and the existing after-commit policy is preserved. Remote cache operation errors must not be silently discarded by the coordinator.

## Out of Scope

- Cross-replica L1 invalidation or its delivery guarantees.
- Distributed request coalescing across API replicas. Existing `singleflight` remains process-local; a Redis distributed lock is not introduced.
- Changing TTL values, cache keys, or which Judgify domains are cached.
- Replacing Ristretto or introducing a new Redis deployment/configuration.

## Verification Strategy

The implementation plan should cover deterministic interleaving cases for both tiers: a loader pauses after obtaining an old source value, invalidation revokes its lease, and the loader resumes; its fill must be rejected. It should also cover invalidation after a successful fill, lease expiry, canceled waiters, concurrent misses, negative caching, XFetch, and Redis cache-operation failures. Existing cache behavior must remain compatible outside the new stale-fill guarantee.

## References

- Facebook's Memcache lease tokens address stale sets and thundering herds: [Scaling Memcache at Facebook (NSDI '13)](https://www.usenix.org/system/files/conference/nsdi13/nsdi13-final170.pdf).
- Redis guarantees atomic execution for scripts, enabling compare-token-and-store / revoke-and-delete operations: [Redis Lua scripting](https://redis.io/docs/latest/develop/programmability/eval-intro/).
