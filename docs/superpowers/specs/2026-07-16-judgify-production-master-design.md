# Judgify Production Master Design

**Date:** 2026-07-16  
**Status:** Draft for written review; implementation has not started  
**Deployment target:** Single tenant, one physical host initially  
**Runtime stack:** Go, Gin, Ent, PostgreSQL, Casbin with Ent adapter, Docker sandbox, Next.js  

## 1. Purpose

This document defines the target architecture, product scope, quality bar, migration strategy, and delivery sequence for upgrading Judgify and go-common into a production-grade DSA judging product.

The design preserves the current technology stack and existing domain folders. It replaces unsafe runtime behavior incrementally through compatibility-first vertical slices. PostgreSQL is the durable source of truth; local queues, caches, schedulers, and WebSocket hubs are accelerators only.

## 2. Design Goals

1. Never lose an acknowledged submission.
2. Produce correct, reproducible verdicts for untrusted contestant code.
3. Apply authentication and authorization changes without partial policy state.
4. Give administrators complete, audited control over users, policy, problems, contests, judge jobs, content, and production health.
5. Replace frontend mock, stale, and mismatched contracts with real typed APIs.
6. Support responsive light, dark, and system themes with one centralized Vietnamese text catalog.
7. Deliver committed changes through a bounded, authorized WebSocket protocol with resynchronization.
8. Turn go-common into a reusable production library without embedding Judgify domain concepts.
9. Prove production readiness through migration, security, crash, load, restore, and rollback gates.

## 3. Non-Goals for the Initial Production Release

- Multi-tenant Casbin domains.
- Domain microservices or per-module databases.
- Redis, Kafka, or a service mesh for Judgify runtime coordination.
- Multi-host high availability before a second host exists.
- Event sourcing or a generic saga/CQRS framework.
- Virtual/team contests, plagiarism detection, interactive judging, MFA, and learning-path recommendations before the stable production release.

These capabilities remain in the long-term roadmap and must not block the correctness and security foundation.

## 4. Current Production Blockers

The current codebase has useful CRUD and domain logic, but it is not production-safe. The principal blockers are:

- Startup can block before HTTP begins serving because a long-running contest orchestrator is called synchronously with an uncancellable context.
- Migrations use bigint identifiers while current Ent schemas use UUID identifiers and include entities absent from the migration sequence.
- Submission creation and local queue publication are separate operations, so a crash can leave an acknowledged submission pending forever.
- The worker commits queue offsets before asynchronous judge work and its side effects complete.
- The runner can classify a non-zero contestant exit as success, buffers output without a hard cap, and has no proven per-run resource protocol.
- The sandbox runs as root and reuses containers without proving cleanup of processes and hidden files.
- JWT middleware does not enforce access-versus-refresh token purpose.
- CORS reflects arbitrary origins while allowing credentials.
- Role and policy replacement deletes and adds rules in multiple non-atomic operations.
- Frontend entity identifiers are frequently numeric while backend contracts use UUIDs.
- Browser tokens and cached profiles are stored in localStorage.
- The admin role page still targets the pre-Casbin permission/resource model.
- Theme switching is disabled by a forced dark theme.
- Contest SSE and submission polling are not authorized, replayable, or reliable enough for production.
- Overall backend coverage is low and critical judge, infrastructure, adapter, and migration paths are largely untested.

## 5. Architectural Style

Judgify remains a modular monolith:

- one repository;
- one binary artifact;
- one PostgreSQL database;
- one Ent schema set;
- explicit business-module ownership enforced in code and architecture tests;
- separate runtime processes only at the judge isolation boundary.

The binary exposes these roles:

```text
judgify api
judgify judge-worker
judgify migrate
judgify all            # local development only
```

### 5.1 API role

The API role owns HTTP, WebSocket, authentication, authorization, product commands and queries, lightweight outbox and leased security-email dispatch, local bounded caches, and health endpoints. It must not connect to the Docker socket or execute contestant code.

### 5.2 Judge-worker role

The judge-worker role claims PostgreSQL jobs, maintains leases and heartbeats, schedules work fairly, executes disposable sandboxes, finalizes verdicts transactionally, and emits outbox events. It is the only process with sandbox privileges.

### 5.3 Migrate role

The migrate role is the only application role allowed to change schema. API and worker runtime database roles receive DML permission but not DDL permission.

## 6. Application Lifecycle

The application root loads and validates configuration, initializes logging, connects dependencies, constructs modules explicitly, registers lifecycle components, and runs them under one signal-derived context.

Lifecycle rules:

- no global mutable dependency ownership;
- no component catches operating-system signals independently;
- no long-running component uses an uncancellable `context.Background()`;
- component failures cancel sibling components through the root lifecycle;
- shutdown stops intake, drains HTTP and WebSocket, stops job claims, waits or cancels in-flight work, releases leases, destroys sandboxes, and closes infrastructure in reverse order;
- components return errors instead of calling `Fatal`;
- all shutdown operations are idempotent and bounded by timeouts.

## 7. Business Ownership

The existing domain directories remain, with ownership clarified as follows.

### Identity

Owns users, credentials, OAuth identities, sessions, refresh tokens, roles, user-role mappings, Casbin policy desired state, user status, profile identity data, and the semantics and redacted payloads of identity/security audit events. The shared audit rows and retention lifecycle remain owned by `api/internal/platform/audit`.

### Problem

Owns problem catalog, tags, difficulties, publication lifecycle, immutable problem revisions, immutable testset revisions, testcase groups, checker configuration, runtime constraints, and judge specification reads.

### Submission

Owns the submission aggregate, idempotent intake, source visibility, the canonical user/problem solved-state projection and its versioned transition history, judge jobs, judge attempts, per-test results, current-generation processing state, active contestant judgement/verdict, and user-facing submission queries. Processing state and the active result are separate truths: a failed or cancelled rejudge never erases a prior valid contestant result.

### Judge execution component

`internal/judge` remains a worker and sandbox execution component, not an independent business owner of submission state. It operates through narrow submission and problem contracts and never imports foreign repositories, Ent entities, or transport handlers.

### Contest

Owns contest lifecycle, problem membership and ordering, registration, eligibility, scoring, standings, freeze state, announcements, clarifications, and rating history.

### Cultivation

Owns elements, traits, rarities, levels, ranks, deterministic progression rules, immutable reward ledger, user progression, and rankings.

### Material

Owns learning categories, articles, publication and visibility, revisions, slugs, reading metadata, and relationships to learning content.

### Notification

`api/internal/notification` owns persisted notifications, preferences for supported channels, unread/read/archive state, idempotent domain-event consumers, and notification REST commands/queries. Realtime only delivers its committed browser-safe events.

### Platform surfaces

Realtime is a delivery adapter, not a business domain. Admin is a product surface that composes module commands and queries, not a god-domain with direct access to every repository.

`api/internal/platform/audit` owns the shared append-only audit schema, redaction contract, retention/archive job, and authorized audit query. It exposes a transaction-aware appender so a domain mutation and its successful audit record commit together; it does not own domain policy.

`api/internal/platform/operation` owns the durable control-plane record for bulk operation identity, progress, pause/cancel eligibility, idempotent resume, and terminal status. Each business module still owns selection, validation, batching, and mutation semantics for its operation type.

`api/internal/platform/contentrender` owns one versioned, deterministic rendering policy for all authored Markdown: problem statements, contest descriptions, announcements, clarifications, notification campaigns, and materials. It disables raw HTML, enforces URL/image allowlists, renders KaTeX with strict untrusted settings, escapes highlighted code, emits checksums, and stores immutable render-source registry rows plus immutable sanitized artifacts. Each business module owns its raw revision and binds that revision to exactly one shared render source. Composite foreign keys then bind every published revision/source pointer and artifact/source pointer, so the database—not a polymorphic UUID convention—makes a cross-domain or mismatched artifact unrepresentable. A renderer upgrade creates a new artifact for the same source and never mutates an authored revision.

`api/internal/platform/transaction` owns Judgify's cross-module `UnitOfWork`. Its stable contract is:

```go
type UnitOfWork interface {
    Within(ctx context.Context, fn func(txCtx context.Context) error) error
}
```

`Within` starts one Ent transaction, binds a transaction-scoped client to a private context key used only by database adapters, commits only when the callback returns nil, rolls back on error or panic, and returns commit/rollback failures without hiding the original cause. The scoped context becomes invalid after the callback and may not escape into goroutines. Nested transactions, network calls, Docker work, and unbounded computation inside the callback are rejected by design/review. Application and domain contracts see only `context.Context`; Ent types never cross their boundary.

Cross-module onboarding writes live in `api/internal/application/workflow/onboarding`; profile, home, and admin overview read composition lives in focused query composers under `api/internal/application/query`. These packages depend only on narrow module contracts and return dedicated view models. They do not expose repositories or become a generic service locator.

Contest-aware submission intake lives in `api/internal/application/workflow/submissionintake`. Inside one `UnitOfWork`, it first asks Identity's narrow account-eligibility participant to lock and revalidate active user state, then asks Submission to reserve per-user admission, Contest to reserve lifecycle/registration eligibility, database time, a monotonic contest-submission sequence, and the exact immutable contest judge snapshot, and Problem to verify the referenced immutable judge tuple. It finally asks Submission to create the aggregate, job, and outbox against those values. Every participant follows the declared global lock order, and end, cancel, unregister, ban, suspension, publication, and capability-retirement commands take compatible locks. Ordinary submissions use Problem's current published snapshot. Neither workflow performs network, Docker, or WebSocket I/O inside the transaction.

Broad notification scheduling and delivery live in `api/internal/application/workflow/notificationcampaign`. Scheduling uses one `UnitOfWork`: Notification locks the exact campaign/confirmation, Identity supplies a narrow hard-bounded audience snapshot, and Notification freezes content/selector, inserts unique recipients, audit, and operation state. Delivery uses bounded UnitOfWork batches in which Identity rechecks active/role eligibility immediately before Notification creates the persisted notification and outbox event. Neither module imports the other's repository or Ent type, and no raw confirmation nonce or external delivery I/O enters the transaction.

## 8. Module Dependency Rules

A module may not import another module's repository, Ent type, domain entity, adapter, HTTP handler, or broad `ports` package.

Cross-module behavior uses exactly one of these patterns:

1. A top-level application workflow and one database transaction when immediate consistency is required.
2. A query composer for read-only views combining several module contracts.
3. A versioned PostgreSQL outbox event for retryable asynchronous side effects.

Contracts contain identifiers, immutable snapshots, result types, and versioned events. They do not expose storage or vendor types.

The onboarding workflow is the concrete owner of registration steps spanning Identity and Cultivation. `api/internal/platform/transaction.UnitOfWork` supplies the transaction-bound callback context to narrow module command participants; no application contract exposes an Ent client/type, and neither module imports the other's repository. Profile, home, and admin overview use read-only query composers; they never perform hidden cross-module writes.

Architecture tests prevent new cross-module imports and new imports of legacy global packages.

## 9. Durability and Consistency

PostgreSQL is the source of truth for:

- domain state;
- sessions and refresh-token state;
- leased security-email delivery intents and short-lived encrypted transport payloads;
- Casbin policies and authorization revision;
- submission jobs and leases;
- judge attempts and results;
- outbox and inbox records;
- audit records;
- persisted notifications.

Local cache, WebSocket Hub, scheduler queues, Forge, and warm sandbox preparation are disposable accelerators. Restarting a process must not lose any acknowledged command.

Synchronous invariants use a single database transaction. Asynchronous consumers use at-least-once delivery with idempotent effects protected by inbox records and unique constraints.

## 10. Authentication and Authorization Summary

Browser authentication uses secure same-origin HttpOnly cookies, CSRF protection, short-lived access tokens, one-time rotating refresh tokens, session revocation, token-family reuse detection, issuer/audience/purpose validation, and key identifiers.

Authorization is single-tenant Casbin RBAC plus explicit application object checks. Judgify owns an Ent schema compatible with `casbin_rules`, role catalog, mappings, authorization revision, audit, and outbox state. The Ent adapter is pinned and patched to avoid runtime schema creation.

Policy mutation validates a complete candidate, writes desired state atomically in one Ent transaction, commits audit and outbox, and atomically swaps a prepared runtime enforcer. Runtime revision must equal desired revision before protected traffic is ready.

## 11. Submission and Judge Summary

Submission acknowledgement occurs only after a transaction commits the submission, immutable judge-plan references and limits, durable judge job, and outbox event.

Workers claim jobs using `FOR UPDATE SKIP LOCKED`, a unique lease token, expiration, and heartbeat. Infrastructure failures retry with bounded exponential backoff; contestant verdicts do not retry. Exhausted infrastructure jobs become dead jobs and transition the submission to a visible system-error state.

Finalization verifies the lease token and atomically writes the attempt, per-test results, current-generation outcome, completed job, active contestant judgement/verdict when the generation produced a valid contestant evaluation, the versioned solved-state transition when it changes, and the associated outbox events. Infrastructure failure and cancellation are visible generation outcomes but never active contestant judgements.

The trusted supervisor, checker, and hidden expected output remain in the worker process outside contestant namespaces. Compilation uses one disposable non-root container; every testcase uses a fresh disposable run container and cgroup so process, filesystem, CPU, and memory state reset per test. Containers use pinned runtime images, disabled network, read-only root filesystems, capability removal, no-new-privileges, seccomp, no host bind mounts, and strict process/file/output/resource limits. The worker communicates only through bounded Docker copy/attach/wait/stats channels, validates copied artifacts as hostile input, kills the whole container on timeout, and produces a structured trusted result containing exit status, signal, wall time, CPU time, peak memory, and truncation flags.

## 12. Realtime and Notifications Summary

go-common exposes `pkg/common/websocket`, backed privately by `coder/websocket`. Judgify exposes one WebSocket connection per browser tab through a reusable frontend provider. An explicitly anonymous connection may subscribe only to the closed public-topic catalog; authenticated sessions use the same endpoint for private user, submission, contest-participant, notification, and admin topics. A present but invalid credential is rejected rather than downgraded to anonymous.

Only committed, explicitly realtime-safe outbox events may reach the browser. Subscriptions are authorized by topic, principal, ownership, contest visibility, and admin permission.

Ordering is guaranteed per aggregate through monotonically increasing aggregate versions. A single database-serialized replay writer assigns and commits contiguous global cursor offsets before broadcast; a PostgreSQL sequence value alone is not treated as commit order. Clients deduplicate repeated events, replay within the retained window, and refetch canonical REST snapshots when the server reports a gap or `resync_required`.

Slow clients cannot consume unbounded memory. Latest-state events coalesce, ephemeral events may drop with metrics, and a blocked critical stream closes with code 1013 and requires resynchronization.

Notifications are persisted, support unread count and read/read-all operations, and use WebSocket only as a low-latency delivery mechanism.

## 13. Frontend Architecture Summary

The frontend remains Next.js App Router with the existing UI stack.

Frontend invariants:

- `EntityID` is a UUID string everywhere;
- API contracts are generated or validated from one schema and checked in CI;
- browser authentication uses cookies and `/api/session/me`, never localStorage tokens;
- one API client, one session provider, and one WebSocket provider serve all pages;
- public/read-heavy screens use server rendering where it improves latency and caching;
- client islands own filters, mutations, editors, and realtime state;
- every route distinguishes loading, empty, unauthorized, forbidden, not found, retryable error, and reconnecting states;
- light, dark, and system themes are supported without hydration flash;
- all product-authored Vietnamese UI copy comes from a typed centralized catalog, while domain/user-authored content remains data;
- accessibility, keyboard control, reduced motion, responsive layout, and performance budgets are release requirements;
- no functional mock or placeholder remains in a production route.

## 14. Product Capability Scope

The stable production release includes:

- secure account, OAuth, password recovery, session/device management, profile editing, and user status;
- public and admin problem lifecycle, immutable revisions, hidden-test protection, runtime/checker configuration, and publish validation;
- durable submissions, detailed verdicts, per-test results, realtime progress, source authorization, retry and rejudge controls;
- correct contest lifecycle, eligibility, scoring, penalty, freeze, rating, announcements, clarifications, and reconciliation;
- revisioned/idempotent onboarding trait offers, deterministic cultivation rewards, ledger, progression history, and rankings;
- material publication, visibility, revisions, safe rendering, reading metadata, and related content;
- persisted notifications;
- audited administration for identity, policy, problems, judge jobs, contests, cultivation, materials, notifications, and system operations;
- production health, metrics, alerts, backups, restore, deployment, and rollback.

Advanced editorials, learning paths, achievements, seasons, virtual/team contests, advanced checkers, plagiarism detection, MFA/passkeys, API tokens, and multi-node operation follow the stable production release.

## 15. Logging and Correlation

go-common owns generic correlation generation, validation, context storage, and propagation. A correlation ID crosses HTTP, gRPC, WebSocket, queues, jobs, outbox events, workers, and notifications.

go-common logging does not know Judgify identifiers. Infrastructure adapters add generic job, event, connection, and causation identifiers; Judgify enriches logs with submission, problem, contest, and user identifiers.

All logs are structured and contain timestamp, level, service, version, environment, component, message, and correlation ID. Relevant flows additionally include bounded identifiers, duration, error code, attempt, and retry classification.

Development defaults to debug. Production defaults to info and may enable time-limited component-level debug dynamically. Debug statements must still redact secrets, tokens, source code, hidden tests, policy secrets, and lease tokens. Hot-loop and message-level logs are sampled or rate-limited.

Major state transitions remain info-level so a production correlation ID reconstructs the complete command path without debug logging.

## 16. go-common Strategy

go-common remains one repository and one Go module with its existing broad package structure. It adds clean protocol and security subpackages while retaining compatibility wrappers for moved APIs.

Stable public packages expose explicit context, lifecycle, ownership, boundedness, and durability semantics. Package APIs avoid Judgify domain concepts and avoid leaking vendor types unless the adapter package itself is explicitly vendor-specific.

Every package is classified as stable, experimental, deprecated, or internal. Forge may become a reliable single-owner embedded queue, but it is not the authoritative Judgify queue. Unsafe runtime linknames, spinlocks around I/O, and unproven custom structures are removed, rewritten, or isolated from stable critical paths.

## 17. Delivery Roadmap

### Wave 0: Baseline and safety

- preserve the current dirty worktree;
- establish clean checkout, build, test, migration, contract, and architecture baselines;
- align Go versions and remove release-time local replacements;
- make UUID the canonical identifier target and run a mandatory migration-history/data inventory before changing schema;
- if and only if the inventory proves there is no authoritative deployment/data, replace the unreleased bigint history with a generated UUID baseline after backup and explicit approval; otherwise use additive UUID shadows, one persisted idempotent UUIDv7 mapping row per legacy identifier, compatibility writes, verified bounded backfill, pointer/read switch, and a later contract release—never a destructive reset.

### Wave 1: Production kernel

- fix critical go-common correctness and security issues;
- add lifecycle, health, security, correlation, transport, logging, and compatibility packages;
- introduce application roles and explicit dependency ownership;
- fix frontend UUID, theme, text, error-state, and API-contract foundations.

### Wave 2: Identity and access

- implement sessions, secure cookies, refresh rotation, CSRF, user state, Casbin desired/runtime revision, atomic admin mutation, audit, and frontend session/policy surfaces.

### Wave 3: Problem, submission, and judge

- implement revisions, durable jobs, leases, idempotency, structured runner, sandbox hardening, per-test results, secure queries, retry/rejudge admin, and correctness/crash gates.

### Wave 4: Outbox, WebSocket, and notifications

- implement retained outbox/inbox, generic WebSocket package, topic ACL, replay/backpressure, persisted notifications, frontend provider, and removal of SSE/polling.

### Wave 5: Contest correctness

- implement complete lifecycle, eligibility, scoring, penalty, freeze, rating, announcements, clarifications, realtime standings, and reconciliation.

### Wave 6: Cultivation, materials, profile, and ranking

- implement revisioned onboarding trait offers, reward ledger, idempotent projections, material visibility and revisions, profile/security completion, rankings, and remaining functional placeholder removal.

### Wave 7: Administration and product polish

- complete operations/audit surfaces, bulk dry-run actions, authoring UX, accessibility, responsive behavior, performance, theme and text audits, and route-state parity.

### Wave 8: Production proof and launch

- pass restore, crash, load, soak, security, migration, deployment, and rollback rehearsals.

### Wave 9: Differentiators

- deliver advanced product capabilities only after the stable production gates remain healthy under real use.

## 18. Migration and Rollback Rules

All data changes follow:

```text
expand -> backfill -> switch -> verify -> contract in a later release
```

- destructive migration never ships in the same release as its replacement;
- binary rollback must work without database rollback;
- Forge and PostgreSQL are never dual authoritative queues;
- a compatibility wrapper remains until consumer count is zero;
- read compatibility is retained through rollout;
- a new worker starts with claims disabled, advertises separate normalized rows for the complete current and immediately previous capability tuples, and passes tuple-specific sandbox/readiness smoke tests;
- old workers stop claiming and drain or safely release leases before new-worker claims are enabled for the existing queue;
- a new API starts on a secondary port and may switch traffic only after its exact emitted protocol/runtime/image/manifest/supervisor/checker/capability-profile tuple has a durable activation backed by a fresh claims-enabled worker row and tuple-specific smoke evidence; the old API then drains;
- rollback keeps the old API usable because the new worker has separately proved the previous complete tuple; support for that previous tuple is contracted only in a later verified release.

## 19. Production Quality Gates

Required CI checks include formatting, vet, lint, race tests, vulnerability checks, unit and tagged integration tests, frontend type/lint/build/tests, generated-code drift, module tidiness, API-contract drift, migration-from-zero and upgrade tests, schema diff, secret scanning, SBOM, dependency licensing, and container scanning.

Critical authentication, authorization, job, scheduler, runner, outbox, and sandbox packages require at least 90 percent meaningful coverage. Other core packages require at least 80 percent changed-package coverage and an overall non-generated coverage ratchet toward 70 percent before production.

Production proof includes:

- 1,000 concurrent WebSockets including reconnect storms and slow clients;
- 250 mixed HTTP requests per second;
- acceptance of 500 submissions during a 50-per-second ten-second burst without lost jobs or unbounded memory;
- compiler and testcase fault injection proving that every testcase receives a fresh container/cgroup and that contestant code cannot read hidden expected data or influence the trusted control result;
- adversarial authored-content fixtures proving raw HTML, unsafe URL schemes, hostile SVG/image input, KaTeX trust escapes, and highlighted-code payloads cannot execute script on any problem, contest, announcement, clarification, or material surface;
- concurrent contest-end and historical-rerate tests proving rating runs apply in one global sequence and rebuild a deterministic suffix;
- a four-hour soak without unbounded goroutine, memory, file descriptor, or queue growth;
- process kills before and after every submission, lease, finalize, and outbox boundary;
- PostgreSQL and Docker restarts;
- disk-pressure behavior;
- successful encrypted off-host backup restore into a blank environment;
- deployment and rollback rehearsal.

Initial single-host recovery targets are RPO at most five minutes and RTO at most sixty minutes. The design does not claim host-level high availability.

## 20. Sub-Specification Index

- `2026-07-16-go-common-production-library-design.md`
- `2026-07-16-identity-authentication-authorization-design.md`
- `2026-07-16-problem-submission-judge-design.md`
- `2026-07-16-realtime-contest-notifications-design.md`
- `2026-07-16-cultivation-material-admin-design.md`
- `2026-07-16-frontend-design-system-data-design.md`
- `2026-07-16-production-operations-release-design.md`

Each sub-spec inherits the architecture, durability, migration, logging, and quality rules in this master document. A sub-spec may tighten these rules but may not weaken or contradict them.

## 21. Acceptance Definition

Judgify is production-complete only when every current route uses a real compatible API, authentication and authorization are secure and audited, acknowledged submissions cannot be lost, verdicts are reproducible and sandboxed, contest scoring is correct, realtime delivery is authorized and resynchronizable, notifications and operations surfaces exist, light/dark/system and centralized text cover every page, and all backup, restore, migration, crash, load, security, and rollback gates pass.
