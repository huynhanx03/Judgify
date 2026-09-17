# Judgify Cultivation, Material, and Admin Design

**Date:** 2026-07-16  
**Status:** Draft for written review  
**Scope:** Production behavior for the cultivation system, learning materials, and the complete single-tenant administration surface

## 1. Outcome

Judgify keeps its distinctive cultivation system and learning library, but makes every reward, content publication, and administrative change explainable, idempotent, permission-checked, and auditable. The admin product composes capabilities owned by each domain; it is not a god service that bypasses business rules or writes arbitrary repositories.

The fixed principles are:

- Cultivation is derived from durable domain achievements, never from best-effort queue messages.
- Every EXP/rating/manual adjustment has an immutable ledger entry and a stable idempotency source.
- Public material reads can never expose a draft or an unauthorized visibility class.
- Published content points at an immutable revision.
- Admin UI mutations call domain commands with invariants, revision checks, reason fields, and audit records.
- Sensitive actions support preview/dry-run and return the committed canonical state.
- The production API never exposes generic “edit any row” behavior for user progression, permissions, judge state, or audit data.

## 2. Domain ownership

### 2.1 Cultivation owns

- Element, trait, rarity, level, and rank catalogs.
- User trait assignment.
- Total EXP, per-element EXP, rating display, level/rank projections, and public leaderboards.
- Reward rules and immutable progression ledger entries.
- Difficulty/tag solved projections used by profiles.

Submission owns canonical user/problem solved state. Contest owns contest results and rating decisions. Cultivation consumes their committed events and creates its own ledger transaction.

### 2.2 Material owns

- Material identity, slug, metadata, category, difficulty, tags, author, and lifecycle.
- Immutable content revisions and publication pointers.
- Visibility decisions and safe public/read projections.
- View aggregates and reading metadata.

Problem tags and difficulties are referenced through narrow IDs/snapshots. Material must not import Problem repositories or Ent entities directly.

Material owns immutable raw revisions and its draft/published pointers. `api/internal/platform/contentrender` owns the single deterministic Markdown/math/code sanitization policy, render interface, shared `content_render_sources` parent registry, and shared immutable `content_render_artifacts` table used by Problem, Contest, Material, and authored notification campaigns. Material never implements a second sanitizer or a domain-specific artifact table.

### 2.3 Admin is a product surface

Admin routes and pages compose domain application services. Each domain retains validation, transactions, policy checks, events, and audit semantics. An admin handler cannot directly instantiate a generic Ent update for a foreign domain.

### 2.4 Shared audit and operation control plane

`api/internal/platform/audit` owns the shared append-only audit schema, redaction, authorized search, and archival lifecycle. Domains use its transaction-aware appender so successful audit and mutation commit together. `api/internal/platform/operation` owns durable IDs, progress, resume, and terminal state for bounded bulk work; the selected domain still owns validation and each batch mutation. Neither package is an admin god-service.

## 3. Cultivation production model

### 3.1 Catalog entities

The existing catalogs remain, with strengthened contracts:

- **Element:** unique stable code, display name, description, display order, active flag.
- **Rarity:** unique stable code, display name, positive weight, display order, active flag.
- **Trait:** stable code, type (`root_bone` or `talent`), rarity, description, typed effect specification, active flag, revision.
- **Level:** stable code/name, unique nonnegative `min_exp`, strictly increasing thresholds, display order.
- **Rank:** stable code/name, unique nonnegative `min_rating`, strictly increasing thresholds, display order.

Names may change without changing stable codes. A catalog row referenced by a user or ledger is archived rather than hard-deleted. Catalog commands validate the complete candidate set before commit so thresholds cannot overlap or become non-monotonic.

Trait effects use a versioned closed schema, not arbitrary business logic hidden in unvalidated JSON:

```json
{
  "v": 1,
  "kind": "exp_multiplier",
  "scope": "element",
  "element_codes": ["fire"],
  "multiplier_delta_bps": 2500
}
```

Accepted initial kinds are `exp_multiplier` and `exp_bonus`; accepted scopes are `all` and `element`. A multiplier is an integer delta in basis points (`2500` means +25%); a flat bonus is an integer EXP value. Values have configured safe bounds, and all intermediate arithmetic uses checked integers. Unknown versions or kinds are rejected at admin validation and never silently ignored in reward calculation.

Reward inputs are append-only and effective-dated. `trait_revisions` stores the validated effect/checksum for each trait revision. `reward_formula_revisions` stores formula version, fixed-point parameters, checksum, and database `effective_at`. Every assignment/reassignment creates a `user_reward_profile_revision` plus normalized profile-trait rows; the profile revision records its database effective time and immutable assignment checksum. “Current” catalog/profile rows are never reward inputs.

Reward-formula publication, trait-effect publication, and user reward-profile changes take the exclusive transaction-scoped reward-timeline advisory lock and assign `effective_at = clock_timestamp()` as their final mutation step. Submission solve-transition creation takes the shared form of the same lock while capturing its database `occurred_at` and immutable Problem reward facts. These lock boundaries make a concurrent edit linearize wholly before or after the solve event. Effective selection uses the greatest append-only revision at or before `occurred_at` with its stable revision tie-break; overlapping/ambiguous history is an invariant failure.

### 3.2 Registration trait selection

`api/internal/application/workflow/onboarding` coordinates Identity account creation and Cultivation trait selection through `api/internal/platform/transaction.UnitOfWork.Within`. Identity and Cultivation commands receive only its transaction-bound callback context; no contract exposes an Ent type, and neither module imports the other's repository.

The launch keeps the existing cultivation roll as a real onboarding mechanic rather than a cosmetic public randomizer. Catalog publication creates an immutable `registration_trait_catalog_revision` containing the exact selectable trait revisions/weights, `weighted-without-replacement-v1` algorithm, one root-bone offer, six distinct talent offers, three required talent selections, configured maximum offers per pre-session, expiry, and checksum. Activation is rejected unless at least one positive-weight root bone and six positive-weight talents exist. Admin edits create a new revision; they never mutate a pool that issued an offer.

`POST /api/onboarding/trait-offers` requires exact Origin/Fetch Metadata, the same signed anonymous pre-session CSRF identity used by registration, and one client idempotency UUID. It is rate-limited by trusted remote IP and pre-session. The launch policy allows a finite configured number of offers per pre-session, with the seeded production default recorded as three including the initial offer; this is onboarding fairness/abuse control, not a claim of economically scarce randomness. An idempotent retry returns the exact existing offer and consumes no additional quota.

```text
onboarding_trait_offers
  id UUIDv7 primary key
  pre_session_id UUID
  idempotency_key UUID
  offer_sequence bigint
  catalog_revision_id UUID
  algorithm_version weighted-without-replacement-v1
  root_bone_trait_id UUID
  root_bone_trait_revision bigint
  offer_checksum string
  status issued|consumed|expired|superseded
  issued_at, expires_at timestamptz
  consumed_by_user_id UUID nullable
  consumed_at timestamptz nullable
  unique (pre_session_id, idempotency_key)
  unique (pre_session_id, offer_sequence)

onboarding_trait_offer_talents
  offer_id UUID
  trait_id UUID
  trait_revision bigint
  display_order integer
  primary key (offer_id, trait_id)
  unique (offer_id, display_order)
```

The sampler uses `crypto/rand` with unbiased bounded integer draws over positive integer weights and removes each chosen trait before the next draw. Entropy failure returns a retryable typed error before quota, supersession, or offer mutation. It never uses the process-global pseudorandom generator, floating-point probability state, or client entropy. The durable offer rows, algorithm/catalog revision, and checksum are the reproducibility record; logs contain only CID, offer ID, revision, outcome, and bounded quota class—not the full draw or personal form data. Tests inject a deterministic entropy reader without adding a production seed API.

Offer creation takes a transaction-scoped lock keyed by pre-session, rechecks idempotency/quota, and assigns the next contiguous `offer_sequence`. Under launch policy, committing a newer logical offer atomically marks its prior unconsumed `issued` offer `superseded`; an exact retry of the same idempotency UUID never does so. Thus concurrent rerolls serialize and reroll deliberately replaces the visible choice instead of letting a client hoard every prior draw. The response includes offer ID/sequence, catalog revision/checksum, expiry, remaining-offer count, one root bone, six distinct talents, and safe display data. Clients ignore a late response with a lower sequence than their current canonical offer. The trait codex remains a read-only explanation of active traits and rarity; it is not a way to select an unoffered trait.

The registration command supplies `offer_id`, its offered root bone, and exactly three selected talent IDs. Its onboarding transaction locks the pre-session/offer before creating a user and verifies:

- The offer is issued, unexpired, belongs to the server-signed pre-session, and its checksum/catalog revision match the request.
- The root bone is exactly the offer's immutable root-bone revision.
- Exactly three distinct talent IDs are selected from that offer's six immutable talent revisions.
- The offer's immutable trait/catalog rows remain internally valid; a later active catalog revision does not rewrite or invalidate an already issued unexpired offer unless an audited security revocation explicitly closes it.
- The same user cannot receive duplicate `(user_id, trait_id)` rows.
- Offer consumption, user traits/reward-profile revision, initial stats, Identity account/session, issuance receipt, audit, and outbox effects all commit or all roll back.

An exact registration/OAuth-finalize retry after commit resolves the session issuance receipt and committed offer/user binding; it does not require the offer to become issued again or consume a second offer. A different account, pre-session, payload hash, or idempotency key cannot recover or reuse a consumed offer. The legacy unauthenticated `GET /traits/roll` endpoint is removed after the frontend switches; it is neither entitlement proof nor a production compatibility route.

### 3.3 Progression ledger

All automatic and manual progression changes create an immutable ledger:

```text
progression_ledger
  id UUIDv7 primary key
  user_id UUID
  source_type string
  source_id UUID
  source_event_id UUID nullable
  source_aggregate_id UUID nullable
  source_aggregate_version bigint nullable
  source_transition string nullable
  rule_version integer
  adjustment_type string
  total_exp_delta bigint
  element_deltas jsonb
  difficulty_id UUID nullable
  tag_ids UUID[]
  reason string
  actor_id UUID nullable
  correlation_id string
  occurred_at timestamptz
  reversed_entry_id UUID nullable
  provenance jsonb
  unique (source_event_id) where source_event_id is not null
  unique (source_type, source_aggregate_id, source_aggregate_version, adjustment_type, rule_version)
  unique (source_type, source_id, adjustment_type, rule_version) where source_event_id is null
  unique (reversed_entry_id) where reversed_entry_id is not null
```

`user_stats`, `user_element_exp`, difficulty/tag projections, the ledger entry, the consumer inbox marker, and an outbox event commit in one Ent transaction. A duplicate event returns the existing ledger result without changing balances again.

Entries are never updated or deleted. Corrections create a compensating entry linked through `reversed_entry_id`. Admin adjustments require a nonempty reason and an explicit permission distinct from ordinary catalog update.

### 3.4 First-solve reward

Submission owns the canonical versioned `user_problem_solves` state keyed by `(user_id, problem_id)`. Successful finalization or rejudge recomputes it and emits `problem.solve_status_changed.v1` only after commit. The event contains event ID, stable solve aggregate ID, user/problem IDs, previous/current state, contiguous aggregate version, active accepted submission/generation/revision provenance when solved, database `occurred_at`, source CID, and immutable Problem reward facts: difficulty reward value/revision, tag/element mapping revision IDs, and their checksum. Submission retains the immutable transition history needed for replay/reconciliation.

Cultivation stores one `solve_reward_cursor` per solve aggregate with `last_applied_version`, current state, last event ID/hash, active reward-ledger entry ID, and gap/reconciliation state. Consumption is serialized per aggregate:

- `version == last_applied_version + 1` applies exactly one transition and advances the cursor in the same ledger/projection/inbox transaction.
- `version <= last_applied_version` is an idempotent stale/duplicate only when its event ID and canonical transition hash agree with the durable inbox/processed-transition record for that version; conflicting provenance is dead-lettered and alerts rather than being ignored.
- `version > last_applied_version + 1` applies nothing. The cursor enters `blocked_gap`, records the missing range, and requests the immutable transition range from Submission. Missing versions replay in order. If retained history cannot satisfy the range, an audited reconciliation compares the canonical solve snapshot/version and writes explicit compensating ledger entries; it never jumps the cursor silently.

`unsolved -> solved` creates one reward entry and stores it as active. `solved -> unsolved` creates exactly one compensating entry linked to that active reward. `solved -> solved` may move the accepted-submission pointer but creates no reward, and a later `unsolved -> solved` after compensation creates a new version-keyed reward. Every automatic entry records source aggregate/version/transition/event, accepted submission/generation/problem revision, and the exact reward-input snapshots in `provenance`; duplicate, out-of-order, or replayed versions cannot change a balance twice.

A migrated `legacy_reconstructed` active grant is never reinterpreted through the live formula. Its ledger provenance references the approved `legacy_reward_allocation_manifest` row/checksum, and a later `solved -> unsolved` compensates that exact baseline ledger vector. Any separate non-solve opening balance is not attached to a solve cursor and therefore is not reversed when one problem becomes unsolved.

For `unsolved -> solved`, the consumer selects the exact reward-formula revision, user reward-profile revision, and each assigned trait revision effective at the event's database `occurred_at`. Missing, overlapping, or checksum-invalid history blocks the event; the consumer never substitutes mutable current catalogs. It records these inputs and a combined provenance checksum before applying:

- Problem-provided difficulty reward value/revision and tag/element mapping revisions/checksum.
- User reward-profile and normalized trait-assignment revision.
- Effective trait-effect revisions and checksums.
- Effective reward-formula revision and checksum.

The initial fixed-point formula is:

```text
total EXP = round(base EXP ×
            (10,000 + all multiplier deltas + matched element deltas) / 10,000)
            + all flat bonuses + matched element flat bonuses

element EXP = round(base EXP ×
              (10,000 + all multiplier deltas + that element delta) / 10,000)
              + all flat bonuses + that element flat bonus
```

Rounding is integer half-away-from-zero and is covered by golden tests. Negative final rewards are clamped to zero. A formula version change affects new ledger entries only unless an explicit, audited rebuild command creates compensations.

### 3.5 Rating and leaderboard projections

Contest rating history is authoritative for rating deltas. Cultivation mirrors the committed current rating as a profile/ranking projection. It does not independently recalculate contest rating.

Contest algorithm version `expected-rank-v1` uses `1200` as the initial rating. New onboarding creates `user_stats.rating = 1200`, and Cultivation projection events persist that exact Contest algorithm/run version rather than inventing a local rating version. Migration sets users with zero authoritative rating-history rows to `1200`; users with rated history retain the current value derived from that history. Any mismatch between a rated user's history and projection blocks cutover for that user until reconciled rather than being overwritten by the default.

Leaderboards use deterministic ordering:

- Rating: rating descending, last rating update ascending, user ID ascending.
- EXP: total EXP descending, last progression time ascending, user ID ascending.

Public results use cursor pagination, a maximum page size, stable rank calculation, and privacy-safe user projection. Cache keys include projection revision; committed ledger/rating events advance the revision and invalidate stale keys. A fixed ten-minute cache without event invalidation is not accepted for correctness-sensitive rank display.

### 3.6 Cultivation API capabilities

Public/authenticated APIs provide:

- Active element, rarity, trait, level, and rank catalogs.
- Signed-pre-session, CSRF-protected, revisioned/idempotent onboarding trait offers plus exact-offer recovery.
- Rating and EXP leaderboards.
- Current user's cultivation profile and progression history.
- Public profile cultivation summary when user privacy permits.

Admin APIs provide:

- Catalog list/detail/create/update/archive with revision checks.
- Whole-catalog validation preview.
- User progression ledger query.
- Audited user-trait assignment/reassignment with full candidate validation, expected version, reason, and immutable before/after audit; raw create/delete assignment routes are retired.
- Manual adjustment and compensating reversal commands.
- Projection rebuild with dry-run counts and an operations permission.

Direct create/update/delete endpoints for raw user stats and user element EXP are removed from ordinary production admin navigation. Compatibility routes are permission-restricted and retired after the new commands are adopted.

## 4. Material publication and revision model

### 4.1 Material identity and immutable revisions

`materials` stores stable identity and publication state:

```text
id UUIDv7
current_slug_id UUID
current_slug_state current
author_id UUID
status draft|in_review|published|archived
visibility public|authenticated
current_draft_revision_id UUID
published_revision_id UUID nullable
published_render_source_id UUID nullable
published_render_artifact_id UUID nullable
published_render_source_kind material_body nullable
published_source_checksum string nullable
published_at timestamptz nullable
archived_at timestamptz nullable
view_count bigint
version bigint
created_at, updated_at
```

Canonical and historical slugs are explicit rows rather than one overwritten string:

```text
material_slugs
  id UUIDv7
  material_id UUID
  slug string globally unique
  state current|retired
  created_at, retired_at timestamptz nullable
  unique (id, material_id)
  unique (id, material_id, state)
  unique (material_id) where state = current
  check (state = current and retired_at is null
         or state = retired and retired_at is not null)
```

The composite `(materials.current_slug_id, materials.id, materials.current_slug_state)` foreign key references `material_slugs(id, material_id, state)` and the state column has `CHECK = current`, so the canonical pointer can reference only that material's current row. Slug identity and value are immutable; the only row transition is one-way `current -> retired`. A deferrable slug-change transaction retires the prior current row, inserts the new globally unique current row, advances `current_slug_id`, and writes audit/outbox. Looking up any retired slug yields its stable `material_id`; the resolver follows that material's single current pointer and returns a permanent canonical redirect. No redirect pointer or chain is stored, no historical rows are rewritten on later renames, and slugs are never deleted/recycled, so an old URL cannot resolve to another material.

`material_revisions` is append-only:

```text
id UUIDv7
material_id UUID
revision_number bigint
title string
description string
content_markdown text
category_id UUID
difficulty_id UUID
estimated_read_minutes integer
author_id UUID
created_at timestamptz
change_summary string
normalized_source_checksum string
render_source_id UUID unique
render_source_kind material_body
unique (material_id, revision_number)
unique (id, render_source_id, render_source_kind, normalized_source_checksum)
```

The raw revision is the immutable authoring source. Rendered output is never updated in that row. Tags are normalized and revision-scoped:

```text
material_revision_tags
  revision_id UUID
  tag_id UUID
  display_order integer
  primary key (revision_id, tag_id)

content_render_sources  # shared parent; owned by api/internal/platform/contentrender
  id UUIDv7
  source_kind typed enum including material_body
  owner_revision_id UUID
  source_checksum string
  created_by UUID nullable
  created_at timestamptz
  unique (source_kind, owner_revision_id)
  unique (id, source_kind, source_checksum)
  unique (id, owner_revision_id, source_kind, source_checksum)

content_render_artifacts  # shared; owned by api/internal/platform/contentrender
  id UUIDv7
  render_source_id UUID
  render_source_kind typed enum
  renderer_version integer
  sanitizer_policy_version integer
  sanitized_html text nullable
  source_checksum string
  artifact_checksum string nullable
  status ready|rejected
  diagnostics jsonb
  created_by UUID nullable
  created_at timestamptz
  unique (id, render_source_id, render_source_kind, source_checksum)
  unique (render_source_id, renderer_version, sanitizer_policy_version)
```

Editing creates a new raw draft revision plus normalized tag rows; neither is mutated after creation. In the same transaction Material normalizes the bounded Markdown once, hashes those exact normalized bytes, registers exactly one shared source with `source_kind = material_body`, its revision ID, and that checksum, then stores the `render_source_id`, kind, and `normalized_source_checksum` on the raw revision. The composite foreign key `(render_source_id, id, render_source_kind, normalized_source_checksum)` targets `content_render_sources(id, owner_revision_id, source_kind, source_checksum)`. Thus revision ownership, kind, and the bytes-to-render checksum are one database-enforced identity rather than separate application-prose checks.

Material then calls the shared renderer with that registered source and exact normalized bytes. `content_render_artifacts` has a composite `(render_source_id, render_source_kind, source_checksum)` foreign key to the shared source's `(id, source_kind, source_checksum)`; its artifact checksum covers renderer/sanitizer versions plus `sanitized_html`. A `ready` artifact has non-null HTML/checksum, while a deterministic `rejected` artifact has no servable HTML and only bounded safe diagnostics.

The selected revision/source/artifact relationship is also enforced in the database, not only by application validation. `material_revisions` additionally exposes `unique (id, material_id)`; `(materials.published_revision_id, materials.id)` has a composite foreign key to it. `materials.published_render_source_kind` is nullable only while unpublished and otherwise has `CHECK = 'material_body'`; all five publication-pointer fields are null together or non-null together. The composite `(published_revision_id, published_render_source_id, published_render_source_kind, published_source_checksum)` has a foreign key to `material_revisions(id, render_source_id, render_source_kind, normalized_source_checksum)`, while `(published_render_artifact_id, published_render_source_id, published_render_source_kind, published_source_checksum)` has a foreign key to `content_render_artifacts(id, render_source_id, render_source_kind, source_checksum)`. Publishing requires a `ready` artifact produced by the current approved pipeline and atomically sets these exact pointers with lifecycle, audit, and outbox. This makes a wrong-domain source, a revision from another material, a checksum substitution, and a mismatched source/artifact pair unrepresentable after commit.

A renderer/security upgrade creates a new artifact from the same immutable raw revision and switches only the render-artifact pointer through a bounded audited command after validation. Raw history and the previous artifact remain available for authorized audit/rollback. Public readers never render Markdown on demand and never infer “latest artifact”; they read the exact committed revision/artifact pair, so a failed rerender cannot partially change published content.

### 4.2 Lifecycle

```text
draft -> in_review -> published -> archived
  ^          |             |
  +----------+-------------+
```

- Authors with edit permission can create draft revisions.
- Submit-for-review requires title, summary, nonempty content, category, difficulty, valid tags, and a safe render.
- Reviewers can approve/publish or return to draft with a reason.
- Publishing a new revision preserves prior published revisions for history and rollback.
- Archiving removes the material from browse/search while retaining authorized historical access.
- Rollback means republishing a prior immutable revision and creates a new audit/event, not rewriting history.

The existing `group` visibility is disabled at validation until a real group membership model exists. The launch visibility classes are `public` and `authenticated`; storing a visibility that the read path cannot enforce is forbidden.

### 4.3 Read policy and projections

Public browse/detail queries always apply:

- `status = published`.
- `visibility = public` for anonymous users.
- `visibility IN (public, authenticated)` for active authenticated users.
- `published_revision_id IS NOT NULL`.
- `published_render_artifact_id` references a `ready` artifact for that exact revision and the currently allowed renderer/sanitizer policy.

Admin/author preview uses a separate command/query and explicit permission. It cannot be reached by adding arbitrary filters to the public endpoint.

List responses omit full content and return ID, the current `material_slugs.slug`, title, summary, category, difficulty, tags from `material_revision_tags`, author summary, read time, published time, and view count. Detail returns sanitized HTML from the pointed `content_render_artifacts` row plus raw-revision/artifact versions safe for display. Markdown permits GFM, math, and approved code formatting; raw HTML, scriptable URLs, unsafe embeds, and event attributes are removed by the reviewed versioned rendering pipeline.

### 4.4 View counting

A read never starts an unowned `context.Background()` goroutine. Qualified views generate a bounded, idempotent view event keyed by material, coarse viewer/session identity, and time window. The aggregate counter is eventually updated through a durable consumer. Failure to increment a counter never fails content delivery, and retries cannot inflate the count repeatedly.

### 4.5 Search and navigation

Launch search uses PostgreSQL capabilities and supports:

- Text query over title and description.
- Category, difficulty, and tag filters through normalized indexed joins; array/JSON containment is not the canonical tag relation.
- Published date, popularity, and relevance sorts from a closed allowlist.
- Cursor pagination with deterministic tie-breaker.

Slug lookup resolves through globally unique `material_slugs`. A `current` row serves the material; a `retired` row resolves its stable material and redirects to `materials.current_slug_id`, so no redirect chain exists. Material–problem learning paths, collections, bookmarks, reading progress, and recommendations are post-launch capabilities built on explicit entities rather than fields added opportunistically to `materials`.

## 5. Administration information architecture

The admin app has these top-level sections:

1. **Overview** — service health, judge capacity, queue age, active contests, submission/verdict rates, outbox backlog, security alerts, and content awaiting review.
2. **Users & access** — users, account status, sessions, roles, policies, profile attribute schema (not Casbin/ABAC attributes), security events.
3. **Problems & judging** — problem drafts/published revisions, testsets, runtimes, submissions, judge jobs, dead jobs, rejudge batches.
4. **Contests** — lifecycle, participants, problems, freeze, standings, clarifications, announcements, rating runs.
5. **Cultivation** — catalogs, validation, progression ledger, adjustments, rankings/projection health.
6. **Materials** — categories, drafts, review queue, publication history, archive, visibility.
7. **Notifications** — announcement composer, delivery state, templates/keys, preferences summary.
8. **Audit & operations** — immutable audit search, deployment/schema version, dependency health, backup status, capacity, and runbook links.

Navigation is permission-derived on the server and confirmed on every API operation. Hiding a menu item is not authorization.

## 6. Admin command contract

Sensitive commands use a common request shape:

```json
{
  "expected_version": 17,
  "reason": "Correct reward after verified incident INC-42",
  "dry_run": false,
  "payload": {}
}
```

Rules:

- `expected_version` prevents lost updates and returns a conflict with current version and safe diff metadata.
- `reason` is mandatory for permission changes, account state, rejudge, rating rerun, progression adjustment, content rollback, and destructive archive actions.
- `dry_run` performs full authorization and validation and returns impact counts/diff without mutating state.
- Mutations return the committed resource, new version, audit ID, and emitted event IDs.
- Bulk operations use a server-generated operation ID, bounded batches, progress query, idempotent resume, and cancellation only before irreversible steps.

## 7. Audit model

Every successful sensitive domain mutation writes its append-only audit row in the same transaction:

```text
audit_log
  id UUIDv7
  occurred_at timestamptz
  actor_user_id UUID nullable
  actor_session_id UUID nullable
  actor_type user|system|worker
  action string
  resource_type string
  resource_id string
  before jsonb nullable
  after jsonb nullable
  reason string nullable
  correlation_id string
  source_ip_hash string nullable
  user_agent_summary string nullable
  outcome succeeded|denied|failed
  metadata jsonb
```

Audit records redact passwords, tokens, source code, hidden testcase content, OAuth secrets, and private keys. Query filters are allowlisted and access requires `audit.read`. Audit rows cannot be changed through application APIs. Retention is at least one year for security/admin actions and is enforced by an explicit archival job. Denied and validation-rejected sensitive commands are sent to a bounded security-audit writer when PostgreSQL is available. If PostgreSQL itself is unavailable, the system emits a structured security log, metric, and CID and does not falsely claim that a durable audit row exists.

## 8. Access-control registry and invariants

The resource/action registry includes domain-specific actions rather than only generic CRUD:

```text
cultivation.catalog.read|create|update|archive|validate
cultivation.ledger.read|adjust|reverse|rebuild
material.read|create|update|submit_review|review|publish|archive|rollback
user.read|create|update|suspend|restore|delete
profile_attribute.read|create|update|archive|migrate
profile.read_any|update_any
session.read_any|revoke_any
role.read|create|update|delete|set_policy
problem.read_admin|create|update|publish|archive
testset.read_hidden|create|update|publish
submission.read_any|rejudge
judge_job.read|retry|cancel
contest.create|update|publish|operate|score_override|rerate
notification.compose|read_delivery|broadcast_role|broadcast_all_active
audit.read
operations.read|execute
```

Mandatory invariants include:

- At least one active superadmin remains.
- An admin cannot remove their own final administrative path without another superadmin approving the resulting state.
- System roles and stable resource/action keys cannot be silently renamed or deleted.
- Hidden tests and contestant source have separate high-sensitivity permissions.
- Account suspension commits account/session revocation, audit, and outbox in one transaction; cache invalidation and WebSocket closure run only from the post-commit invalidation path. Rollback closes no connection, and replay repairs a crash after commit.
- Rejudge and rerating cannot run concurrently for the same contest unless the operation model explicitly serializes them.
- Catalog rows referenced by history are archived, never hard-deleted.

## 9. Admin workflows

### 9.1 Permission update

1. Load current role and policy revision.
2. Show a matrix diff and affected-user count.
3. Validate resource registry, system-role rules, superadmin/self-lockout invariants.
4. Build and test the complete candidate Casbin policy in memory.
5. Commit role/catalog, `casbin_rules`, authorization revision, audit, and outbox in one database transaction.
6. Atomically swap the runtime enforcer and revalidate affected sessions/sockets.

### 9.2 Rejudge

1. Filter eligible submissions and show count, contest impact, runtime/testset revisions, and estimated capacity.
2. Require reason and immutable target judge-plan revision.
3. Create a rejudge batch plus idempotent jobs; never mutate old attempts.
4. Track progress and failures.
5. Apply terminal results transactionally; reconcile standings and rating through versioned events.
6. Preserve prior results and audit the active-result pointer change.

### 9.3 Progression adjustment

1. Show current balance, recent ledger, and proposed deltas.
2. Validate nonnegative resulting projections and permission.
3. Commit immutable ledger, projections, audit, and outbox.
4. Rebuild affected leaderboard cache revision.
5. Reverse through a linked compensating entry when required.

### 9.4 Material publish

1. Preview the exact immutable raw revision, normalized tags, and selected render artifact.
2. Run completeness, link, tag, visibility, renderer/sanitizer-version, checksum, and sanitization checks.
3. Commit the matching revision/artifact pointers, lifecycle, audit, and event.
4. Revalidate public cache tags and canonical slug.

### 9.5 In-app notification campaign

`api/internal/notification` remains the business owner and the realtime/notification design remains the canonical campaign schema and delivery contract. Its authored revision uses the same shared source-parent contract as Material: the revision stores source ID, typed campaign-body kind, and exact normalized-source checksum; composite foreign keys bind those three values to `content_render_sources` and bind its selected artifact to that same source/kind/checksum. Admin only composes those application commands through `notification.compose`; launch campaigns create persisted in-app notifications only. Email, push, arbitrary webhook delivery, and a frontend-only “send succeeded” state are outside V1.

1. Create an immutable draft revision using a command idempotency key and the campaign's `expected_version`. Preview shows the exact sanitized render artifact, bounded plain-text title, internal allowlisted action path, and the revision that would be frozen.
2. Build the audience only from closed selectors: a configured-bounded explicit/imported user set, one authorized role, or all active accounts. Free-form SQL, arbitrary filter JSON, and client-supplied recipient counts are forbidden. Role and all-active selection additionally require `notification.broadcast_role` or `notification.broadcast_all_active`.
3. Dry-run authorizes the exact revision/selector and returns `estimated_at`, an authorization-safe recipient count, bounded sample, exclusions/skips by low-cardinality reason, and configured batch count/size. It performs no recipient or notification mutation and is labeled as an estimate rather than a delivery promise. After recent reauthentication, role/all-active confirmation yields a short-lived single-use opaque nonce bound to actor/session, campaign, exact revision/source/artifact, selector, expected version, and dry-run timestamp/count.
4. Scheduling requires the unchanged `expected_version`, a nonempty operator reason, a fresh logical-command idempotency key, and an explicit confirmation that repeats audience kind/count and scheduled time. A role/all-active command also requires the still-valid Identity recent-reauthentication window and consumes that exact confirmation nonce; changed inputs, expiry, or a different replay fail closed. Consumption records the canonical schedule request hash, idempotency key, and operation ID, so an exact retry after a lost response returns the already committed operation without reusing authority for another command. The command freezes the exact content/audience revision, commits audit, and starts one durable `platform/operation`.
5. The schedule workflow uses the shared UnitOfWork and Identity's narrow audience-snapshot participant to atomically create a hard-bounded recipient snapshot with the frozen campaign; Notification never imports Identity storage. Delivery then rechecks active/role eligibility through a transaction-bound Identity participant and creates notifications in configured bounded transactions. Durable checkpoints and per-recipient uniqueness make resume idempotent; transient failures remain pending with backoff, exhausted/nonretryable failures enter a bounded `failed` state, and the UI reads committed operation/recipient counts rather than treating request acceptance as completion.
6. Cancellation is idempotent, requires expected version and reason, prevents new batches, and cannot retract already committed notifications. An explicitly authorized expected-version/reasoned resume retries only safe failed/pending work and cannot duplicate `created` recipients. Pending, created, skipped, failed, and cancelled counts plus bounded safe failure summaries remain queryable and audited.

The composer disables scheduling after any edit until a new dry-run of that exact revision succeeds. It preserves entered content on validation/network failure, renders version conflicts with the canonical current revision, requires a second deliberate confirmation for broad audiences, and never exposes recipient private fields in samples or exports.

## 10. Migration strategy

Legacy solve rewards require an explicit allocation artifact before shadow-ledger cutover:

```text
legacy_reward_allocation_manifest
  id UUIDv7
  migration_run_id UUID
  high_water_marker string
  user_id UUID
  solve_aggregate_id UUID
  problem_id UUID
  total_exp_grant bigint >= 0
  element_exp_grants jsonb  # closed element-ID map; every value >= 0
  provenance historically_proven|operator_reviewed_policy
  historical_evidence_checksum string nullable
  allocation_policy_version string nullable
  reviewed_by UUID nullable
  review_reason string nullable
  row_checksum string
  unique (migration_run_id, solve_aggregate_id)

legacy_reward_allocation_residuals
  migration_run_id UUID
  user_id UUID
  total_exp_residual bigint >= 0
  element_exp_residuals jsonb  # every value >= 0
  classification historically_proven_non_solve|operator_reviewed_residual
  evidence_checksum string nullable
  reviewed_by UUID nullable
  review_reason string nullable
  snapshot_checksum string
  row_checksum string
  unique (migration_run_id, user_id)
```

`historically_proven` requires immutable evidence and its checksum. Otherwise an operator must explicitly approve one versioned allocation policy, actor, and nonempty reason; approval itself is audited and checksum-bound. A positive non-solve residual likewise requires proven non-solve evidence or explicit operator classification/actor/reason, so it cannot become an unexplained balancing bucket. Neither path evaluates the current mutable trait, profile, difficulty, mapping, or reward-formula rows. For each user and every total/element projection dimension, the sum of solved-pair vectors plus the one non-solve residual must equal the checksummed high-water snapshot exactly. A residual may be zero but never negative. A tampered checksum, missing solved pair, duplicate allocation, unexplained dimension, policy/classification without approval, or inability to satisfy those equations blocks that user and the global cutover.

1. Add registration trait-catalog/offer/offer-talent, ledger, solve-reward cursor, shadow ledger-derived projections, effective-dated trait/formula/profile revision history, legacy reward-allocation manifest/residual/correction tables, audit, raw material revision, normalized revision-tag, material-slug history, render-artifact, lifecycle, and projection-version tables without removing current fields.
2. Begin dual-writing immutable revisions and audit for new admin mutations. Seed and verify the first registration trait-catalog revision, deploy the CSRF/idempotent offer POST and frontend recovery flow, then remove the legacy roll GET only after its route consumer count is zero. Under the exclusive reward-timeline lock, seed one checksum-verified current trait-effect, reward-formula, and user reward-profile revision before solve-transition capture; all later catalog/profile changes append revisions. Gaps, overlaps, insufficient offer pools, or an unreconstructable active profile block activation.
3. Backfill raw material revision 1, normalized tag rows, one shared `content_render_sources` row, and one globally unique current slug per material, then render it through the shared `api/internal/platform/contentrender` migration policy into the common `content_render_artifacts` table. Set the paired published revision/source/kind and artifact/source pointers plus current slug only when their foreign-key ownership, checksums, visibility, tags, and sanitization pass; quarantine failures without exposing legacy HTML or reusing a conflicting slug.
4. Enable durable Submission solve-transition/outbox dual-write while the legacy reward path/live projections remain authoritative, then transactionally capture its high-water mark and checksummed user/element snapshots. Reconstruct every canonical solved pair and its stable baseline transition. Build and freeze the durable allocation manifest through historically checksummed evidence or the explicitly approved allocation policy; do not estimate a baseline with current reward inputs. For each pair, create a `legacy_reconstructed` reversible ledger entry with exactly its manifest total/element vector and set that solve cursor's `active_reward_ledger_entry_id` to the exact entry. Create `legacy_non_solve_opening_balance` ledger entries only for the manifest's nonnegative residual vector, with a non-solve source that no solve cursor can reverse. Recompute difficulty/tag solved projections from the canonical solved set; every legacy mismatch becomes a durable audited `legacy_solve_projection_correction` with before/after/run checksum rather than being hidden in EXP allocation. Verify all vector equations, manifest/correction checksums, active pointers, and canonical counts. Invalid/unallocatable/unapproved users block cutover.
5. Initialize users with no authoritative rating history to rating `1200`; preserve and verify rated users against their history. Reconcile every EXP/element/difficulty/tag/rating projection and cursor before enabling consumers.
6. With new live claims still disabled, a bounded migration replay invokes the same transactional transition handler against the shadow projections for every durable event after the captured high-water mark. Acquire a short reward-cutover gate, drain both paths, and replay to the final cursor. A declared legacy-versus-new rule delta is represented by an explicit audited `cutover_reconciliation` entry; any unexplained mismatch blocks. After every shadow/live balance and dimension agrees, atomically switch projection/read ownership, enable the outbox/inbox consumer, and disable Forge before releasing the gate; there is no interval in which both can award.
7. Replace raw user-stat admin routes with ledger commands and update admin UI.
8. Switch public material reads to the exact publication-aware revision/artifact projection.
9. Retire direct mutation and legacy compatibility paths after audit shows no clients use them.

## 11. Verification and acceptance

Automated verification must prove:

- Trait-offer catalog activation rejects insufficient/duplicate/nonpositive pools; the versioned integer sampler uses production CSPRNG entropy, returns one root bone plus six distinct talents, and deterministic injected-entropy vectors remain stable under `weighted-without-replacement-v1`.
- Same-pre-session/idempotency retries return the exact durable offer without spending quota; concurrent same/different-key rolls serialize into one contiguous sequence, only the highest committed offer remains issued, stale response order cannot reactivate it, configured offer/IP limits remain bounded, expiry/supersession is explicit, logs omit draws/form data, and the legacy unauthenticated roll GET is absent at the production route gate.
- Password and new-user OAuth onboarding accept only the offer's exact root bone plus three distinct offered talents and consume the offer in the same UnitOfWork as account/profile/reward-profile/session state; rollback consumes nothing, a different pre-session/account/key cannot reuse it, and exact session-issuance recovery converges on the already consumed offer/user.
- Duplicate first-solve events create one ledger entry and one reward; conflicting same-version provenance is rejected and alerted.
- Out-of-order solve events, missing-version gaps, replay recovery, and retained-history exhaustion never advance a cursor silently; audited reconciliation produces exact compensations.
- `unsolved -> solved -> unsolved -> solved` creates reward, linked reversal, and a new version-keyed reward without duplicate balance effects; `solved -> solved` pointer changes create none.
- Solve-reward provenance preserves aggregate/event versions, transition, active accepted submission/generation/problem revision, rule inputs, and source CID.
- Every legacy solved pair has one checksum-valid manifest allocation with proven evidence or an explicit versioned operator policy/actor/reason; pair vectors plus nonnegative non-solve residuals reproduce each total/element high-water dimension exactly without consulting current reward inputs.
- Every reconstructed solve cursor points to its exact manifest-backed baseline entry; unsolving it reverses only that vector while leaving non-solve residual entries untouched. Negative residuals, checksum drift, missing/duplicate pairs, unapproved policy, or unsatisfied equations block cutover.
- Difficulty/tag solved projections are recomputed from canonical solves and every legacy mismatch has an explicit checksummed correction record. Replay from the high-water mark has no holes, the gated ownership switch creates neither a double reward nor a lost transition, and unexplained/unreconciled users block cutover.
- Reward formulas produce golden values across trait combinations and rule versions.
- A solve replayed after trait, assignment, difficulty mapping, or formula edits selects the same effective revisions/checksums and produces the same delta as its original event; it never reads a mutable current row.
- Concurrent solve capture versus reward-catalog/profile publication is tested in both linearization orders under the shared/exclusive reward-timeline lock, and missing, overlapping, or checksum-invalid effective history blocks without advancing the cursor.
- Concurrent rewards and manual adjustments do not lose deltas.
- Ledger reversals exactly compensate the selected entry and preserve history.
- Invalid trait types, duplicates, inactive traits, malformed effects, and threshold overlaps are rejected.
- Anonymous and authenticated public queries cannot retrieve drafts, archived materials, or forbidden visibility classes.
- Raw material revisions, normalized revision-tag rows, render-source rows, and render artifacts remain immutable; database constraints reject cross-material revisions, wrong source kinds, and mismatched revision/source/artifact pairs, while publishing exposes one checksum-matched selection atomically and rollback/rerender changes pointers without rewriting history.
- Slug tests prove global uniqueness, same-material current-pointer ownership, retired-slug canonical redirects without chains, conflict-safe rename, and permanent non-reuse of historical slugs.
- Sanitization blocks script, event-handler, unsafe URL, and raw-HTML attack fixtures.
- Shared content-render golden/fuzz fixtures produce the same policy/checksum behavior for Problem, Contest, and Material; renderer-security upgrades fail closed on rejected/mismatched artifacts, preserve the prior artifact until the audited pointer switch, and public reads never render raw Markdown or select an implicit latest artifact.
- New and migrated users receive rating `1200` only when they have no authoritative history; rated users retain the history-derived value and inconsistencies block cutover.
- Admin optimistic conflicts return the canonical current version without partial mutation.
- Every successful sensitive mutation commits its redacted audit atomically; denied/validation outcomes are durable when PostgreSQL is available, and dependency failures always produce the specified structured security log/metric/CID fallback.
- Permission invariants prevent deletion of the final superadmin and self-lockout.
- Campaign tests cover exact-revision preview/dry-run, expected-version and recent-reauth conflicts, expired/replayed/wrong-revision broad-audience confirmation nonces, explicit/role/all-active permission scopes, bounded batching, crash/resume idempotency, mid-batch cancellation, recipient recheck/skip, no duplicate notification, and audit/operation counts; launch makes no email or push delivery.
- Cultivation and material query plans meet the production pagination and latency budgets.
