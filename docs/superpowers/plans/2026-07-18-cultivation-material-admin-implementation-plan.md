# Judgify Cultivation, Material, and Admin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve Judgify’s cultivation identity while making onboarding randomness, rewards, rankings, material publication, and every administrative mutation deterministic, auditable, idempotent, and recoverable.

**Architecture:** Cultivation owns immutable catalogs, trait offers, reward ledgers, progression and rankings. Material owns stable articles with immutable revisions and shared sanitized render artifacts. Admin is a capability-controlled composition surface over domain commands plus shared audit/operation records; it never writes foreign repositories directly.

**Tech Stack:** Go/Ent/PostgreSQL, crypto/rand, UnitOfWork, PostgreSQL outbox/inbox, shared content renderer, Casbin capability registry, and Next.js admin product.

## Global Constraints

- Registration trait offers use `weighted-without-replacement-v1`.
- One offer contains exactly one root and six distinct talents; the user selects exactly three offered talents.
- Production default allows three offers per pre-session including the initial offer.
- Exact idempotent retry consumes no extra offer; a newer logical offer supersedes the prior unconsumed offer.
- Client-submitted trait IDs are never entitlement proof.
- Every EXP/rating/manual adjustment has an immutable ledger row and stable source/idempotency identity.
- First-solve and unsolve effects derive from versioned canonical solve transitions.
- Legacy reward migration uses a checksummed exact allocation manifest; it never guesses from current formulas.
- Material authored revisions and render artifacts are immutable.
- Raw HTML and unsafe authored content never reach production surfaces.
- Admin bulk/destructive operations require preview, exact confirmation, reason, optimistic version, audit, and idempotency.
- Admin is not a god-domain and imports no foreign repository/Ent type.
- No commits.

---

## Required Execution Order

The numeric task ID is authoritative: **1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11**. Task sections are grouped in this document for domain review; an implementer must jump to and finish the next numeric ID before starting a later one.

### Task 1: Version the Cultivation Catalog and Generate Durable Trait Offers

**Files:**
- Modify: `api/internal/ent/schema/trait.go`
- Modify: `api/internal/ent/schema/rarity.go`
- Create: `api/internal/ent/schema/cultivation_catalog_revision.go`
- Create: `api/internal/ent/schema/onboarding_trait_offer.go`
- Create: `api/internal/ent/schema/onboarding_trait_offer_talent.go`
- Create: `api/internal/cultivation/core/entity/catalog.go`
- Create: `api/internal/cultivation/core/entity/trait_offer.go`
- Create: `api/internal/cultivation/ports/trait_offer.go`
- Create: `api/internal/cultivation/adapters/driven/db/trait_offer.go`
- Create: `api/internal/cultivation/core/service/trait_offer.go`
- Create: `api/internal/cultivation/core/service/trait_offer_test.go`
- Modify: `api/internal/cultivation/adapters/driver/http/gacha.go`

**Interfaces:**

```go
const OfferAlgorithmVersion = "weighted-without-replacement-v1"

type CreateOfferCommand struct {
    PreSessionID uuid.UUID
    IdempotencyKey uuid.UUID
}

type TraitOffer struct {
    ID uuid.UUID
    Sequence int64
    CatalogRevisionID uuid.UUID
    CatalogChecksum string
    Root TraitSummary
    Talents []TraitSummary
    Remaining int
    ExpiresAt time.Time
}
```

- [ ] **Step 1: Write algorithm, quota, race, and entropy tests**

Use injected deterministic integer entropy to prove weighted selection without replacement, exact one root/six distinct talents, no modulo bias, closed active catalog, stable catalog checksum, contiguous sequence, three-offer quota, same-key exact replay, concurrent reroll serialization, newer superseding older, late lower-sequence client response ignored by contract, expiry, and no arbitrary trait consumption.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/cultivation/core/service -run TestTraitOffer -count=1`

Expected: FAIL because durable offers do not exist.

- [ ] **Step 3: Implement cryptographic weighted draws**

Use rejection sampling over `crypto/rand.Reader` for unbiased bounded integers. Snapshot one immutable catalog revision and checksum. Inside a pre-session-keyed transaction lock, recheck idempotency/quota, mark prior issued offer superseded for a new logical key, assign next contiguous sequence, and insert root/talent rows.

- [ ] **Step 4: Replace public roll endpoint**

Expose only CSRF/pre-session-protected `POST /api/onboarding/trait-offers` with client UUID. Keep trait codex as read-only catalog. Remove unauthenticated `GET /traits/roll` after the frontend switches.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go generate ./internal/ent
go test -tags=integration ./internal/cultivation/core/service -run TestTraitOffer -count=100
go test -race ./internal/cultivation/core/service -run TestTraitOffer -count=20
```

Expected: all exit 0 and no process-global pseudo-random state remains. Do not commit.

### Task 2: Implement the Onboarding Trait-Consumption Participant

**Files:**
- Create: `api/internal/cultivation/ports/onboarding.go`
- Create: `api/internal/cultivation/core/service/onboarding.go`
- Create: `api/internal/cultivation/core/service/onboarding_test.go`
- Modify: `api/internal/application/workflow/onboarding/register.go`
- Modify: `api/internal/application/workflow/onboarding/register_test.go`
- Modify: `api/internal/cultivation/di/di.go`

**Interfaces:**

```go
type ConsumeOfferCommand struct {
    OfferID       uuid.UUID
    PreSessionID  uuid.UUID
    ExpectedSequence int64
    RootID        uuid.UUID
    TalentIDs     []uuid.UUID
    UserID        uuid.UUID
}

type OnboardingParticipant interface {
    ConsumeOffer(ctx context.Context, command ConsumeOfferCommand) (OnboardingFacts, error)
}
```

- [ ] **Step 1: Write atomic onboarding tests**

Assert exact pre-session/current issued offer, root equals offered root, exactly three distinct talents drawn from offered six, catalog/revision active at issue not re-rolled at consume, one account consumes once, superseded/expired/other-account rejection, exact issuance-receipt recovery after consume, and rollback with Identity/profile/session/recovery-contact on any failure.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/cultivation/core/service ./internal/application/workflow/onboarding \
  -run TestOnboarding -count=1
```

Expected: FAIL until participant is wired.

- [ ] **Step 3: Implement transaction-scoped consumption**

Lock offer, validate immutable rows and command, mark consumed with user/receipt binding, create initial user stats/root/talents/progression rows, and return facts for issuance fingerprint/audit. Do not create a nested transaction or import Identity repositories.

- [ ] **Step 4: Enable registration routes**

Replace the temporary contract fake from the identity plan with the real participant. Registration and new-user OAuth finalize become enabled only after this integration suite passes.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/application/workflow/onboarding \
  ./internal/cultivation/core/service -run TestOnboarding -count=50
go test ./...
```

Expected: all exit 0 and no partial user/cultivation state exists. Do not commit.

### Task 3: Add the Immutable Progression Ledger and Versioned Reward Rules

**Files:**
- Create: `api/internal/ent/schema/progression_ledger.go`
- Create: `api/internal/ent/schema/reward_rule_revision.go`
- Modify: `api/internal/ent/schema/user_stats.go`
- Modify: `api/internal/ent/schema/user_element_exp.go`
- Create: `api/internal/cultivation/core/entity/ledger.go`
- Create: `api/internal/cultivation/core/entity/reward_rule.go`
- Create: `api/internal/cultivation/ports/ledger.go`
- Create: `api/internal/cultivation/adapters/driven/db/ledger.go`
- Create: `api/internal/cultivation/core/service/reward.go`
- Create: `api/internal/cultivation/core/service/reward_test.go`

**Interfaces:**

```go
type RewardSource struct {
    SourceType string
    SourceID uuid.UUID
    SourceEventID *uuid.UUID
    AggregateVersion *int64
    RuleVersion string
}

type LedgerEntry struct {
    ID uuid.UUID
    UserID uuid.UUID
    AdjustmentType string
    Delta int64
    BalanceAfter int64
    Source RewardSource
    CorrelationID string
    CreatedAt time.Time
}
```

- [ ] **Step 1: Write ledger/idempotency/overflow tests**

Test unique source event, fallback compound uniqueness, non-zero delta, checked balance arithmetic, immutable rows, deterministic rule snapshot, duplicate event no-op, concurrent events serial balance, first solve reward once, unsolve exact reversal, re-solve new transition, and no direct judge mutation.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/cultivation/core/service -run TestReward -count=1`

Expected: FAIL because ledger/reward consumer is absent.

- [ ] **Step 3: Implement event-driven reward transaction**

Consume `user_problem_solve.changed.v1`, verify event against canonical transition, lock user progression, compute from snapshotted difficulty/tag/rule facts, insert ledger rows and derived balances, append cultivation outbox, and record inbox atomically.

- [ ] **Step 4: Add manual adjustment command**

Require capability, non-empty incident/reason, preview, expected balance/version, bounded delta, idempotency UUID, audit, and exact ledger source. Never update balance without a ledger entry.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/cultivation/core/service -run TestReward -count=100
go test -race ./internal/cultivation/core/service -run TestReward -count=20
```

Expected: all exit 0. Do not commit.

### Task 6: Add Material Search, Navigation, Related Content, and Bounded Views

**Files:**
- Create: `api/internal/ent/schema/material_view_daily.go`
- Create: `api/internal/material/core/service/search.go`
- Create: `api/internal/material/core/service/search_test.go`
- Create: `api/internal/material/core/service/view.go`
- Create: `api/internal/material/core/service/view_test.go`
- Modify: `api/internal/material/adapters/driven/db/material.go`
- Modify: `api/internal/material/adapters/driver/http/material.go`

**Interfaces:**

```go
type SearchQuery struct {
    Text string
    CategoryID *uuid.UUID
    Tags []string
    Difficulty *string
    Cursor string
    Limit int
}
```

- [ ] **Step 1: Write query/visibility/view tests**

Cover published-only public predicates, draft editor/admin predicates, allowlisted sort/filter, bounded full-text input, stable keyset cursor, related-content deterministic scoring, no hidden revision leak, anonymous/session daily view dedupe using privacy-preserving HMAC, and view write failure not breaking article read.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/material/core/service -run 'Test(Search|View)' -count=1`

Expected: FAIL until search/view services exist.

- [ ] **Step 3: Implement PostgreSQL-backed search and counters**

Use normalized text/search vector/indexes already available in PostgreSQL; do not add Elasticsearch for single-host launch. Related content uses bounded shared tags/category/difficulty and stable tie-break. View intents are batched/deduped durably or written idempotently; raw IP is never stored.

- [ ] **Step 4: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/material/core/service -run 'Test(Search|View)' -count=50
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 7: Add the Durable Bulk Operation Control Plane

**Files:**
- Create: `api/internal/ent/schema/operation.go`
- Create: `api/internal/ent/schema/operation_batch.go`
- Create: `api/internal/platform/operation/operation.go`
- Create: `api/internal/platform/operation/repository.go`
- Create: `api/internal/platform/operation/runner.go`
- Create: `api/internal/platform/operation/runner_test.go`
- Create: `api/internal/platform/operation/http.go`

**Interfaces:**

```go
type Status string

const (
    Pending Status = "pending"
    Running Status = "running"
    Paused Status = "paused"
    Completed Status = "completed"
    Failed Status = "failed"
    Cancelled Status = "cancelled"
)

type Operation struct {
    ID uuid.UUID
    Type string
    RequestHash string
    Status Status
    Total int64
    Processed int64
    Succeeded int64
    Failed int64
}
```

- [ ] **Step 1: Write resume/idempotency/state tests**

Test unique operation type+idempotency/request hash, changed request conflict, bounded batches, progress monotonicity, pause/cancel eligibility, crash/lease reclaim, exact resume, per-item failure summary bounds, audit linkage, and no raw arbitrary job callback persisted.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/platform/operation -count=1`

Expected: FAIL because operation plane is absent.

- [ ] **Step 3: Implement generic control only**

Platform owns identity/state/lease/progress. Each domain registers a typed bounded batch executor in application wiring; platform never imports domain repositories or decides selection/mutation semantics.

- [ ] **Step 4: Verify**

Run:

```bash
cd api
go generate ./internal/ent
go test -tags=integration ./internal/platform/operation -count=100
go test -race ./internal/platform/operation -count=10
```

Expected: all exit 0. Do not commit.

### Task 8: Build Admin Query Composition and Audited Command Contracts

**Files:**
- Create: `api/internal/application/query/adminoverview/query.go`
- Create: `api/internal/application/query/adminoverview/query_test.go`
- Create: `api/internal/application/workflow/admincommand/command.go`
- Create: `api/internal/application/workflow/admincommand/confirmation.go`
- Create: `api/internal/application/workflow/admincommand/command_test.go`
- Create: `api/internal/platform/audit/query.go`
- Create: `api/internal/platform/audit/http.go`
- Create: `api/internal/platform/audit/query_test.go`
- Modify: `api/internal/infrastructure/router.go`

**Interfaces:**

```go
type CommandMeta struct {
    Actor authentication.Principal
    ExpectedVersion int64
    IdempotencyKey uuid.UUID
    Reason string
    ConfirmationToken string
}

type AdminOverview struct {
    Users Summary
    Judge Summary
    Contests Summary
    Operations Summary
    Alerts []Alert
    AsOf time.Time
}
```

- [ ] **Step 1: Write capability/confirmation/audit tests**

Test each admin area requires an explicit capability, preview tokens are HMAC-bound to actor/session/action/target/version/request hash/expiry, changed input conflicts, destructive action requires exact confirmation, reason non-empty, optimistic version, idempotent lost response, mutation+audit atomicity, and redacted before/after query.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/application/query/adminoverview \
  ./internal/application/workflow/admincommand ./internal/platform/audit -count=1
```

Expected: FAIL because composition/contracts are absent.

- [ ] **Step 3: Implement narrow query and command composition**

Overview calls read-only module contracts concurrently with bounded fan-out and one deadline; it never writes. Admin command workflow validates meta then invokes one typed domain command under its ownership. It does not accept repository names, table names, raw SQL, or arbitrary payload maps.

- [ ] **Step 4: Implement audit search**

Allowlisted filters include actor, action, resource, target, CID, time range, and operation ID with bounded pagination. Sensitive payload fields remain redacted and large diffs are summarized/checksummed.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/application/query/adminoverview \
  ./internal/application/workflow/admincommand ./internal/platform/audit -count=50
go test -race ./internal/application/query/adminoverview -count=10
```

Expected: all exit 0. Do not commit.

### Task 9: Implement Cross-Domain Admin Workflows

**Files:**
- Create: `api/internal/application/workflow/admincommand/rejudge.go`
- Create: `api/internal/application/workflow/admincommand/progression.go`
- Create: `api/internal/application/workflow/admincommand/material.go`
- Create: `api/internal/application/workflow/admincommand/access.go`
- Create: `api/internal/application/workflow/admincommand/workflows_test.go`
- Modify: `api/internal/identity/adapters/driver/http/role.go`
- Modify: `api/internal/submission/adapters/driver/http/admin.go`
- Modify: `api/internal/cultivation/adapters/driver/http/user_stats.go`
- Modify: `api/internal/material/adapters/driver/http/material.go`
- Modify: `api/internal/notification/adapters/driver/http/campaign.go`

**Interfaces:**
- Consumes Identity policy replacement, Submission rejudge, Cultivation adjustment, Material publication, Notification campaign, platform operation/audit.
- Produces uniform preview/apply/operation responses without bypassing domain rules.

- [ ] **Step 1: Write workflow contract tests**

For permission update, rejudge, progression correction, material publish, and notification campaign assert preview/apply parity, exact expected revision, actor capability/recent-reauth where required, self-lockout/domain invariant enforcement, idempotent retry, rollback, audit/outbox/operation linkage, and safe response.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/application/workflow/admincommand -run TestWorkflow -count=1`

Expected: FAIL until workflow adapters exist.

- [ ] **Step 3: Implement typed composition**

Each file depends on the smallest domain interface and maps the uniform `CommandMeta` to a domain command. Permission policy remains Identity-owned; rejudge Submission-owned; ledger Cultivation-owned; publication Material-owned; campaign Notification-owned. No workflow imports an adapter, repository, Ent type, or another module’s broad ports package.

- [ ] **Step 4: Verify**

Run:

```bash
cd api
go test ./internal/application/workflow/admincommand -count=100
go test -race ./internal/application/workflow/admincommand -count=10
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 10: Backfill Exact Legacy Rewards and Material Revisions

**Files:**
- Create: `api/internal/ent/schema/legacy_reward_allocation_manifest.go`
- Create: `api/internal/cultivation/migration/backfill.go`
- Create: `api/internal/cultivation/migration/backfill_test.go`
- Create: `api/internal/material/migration/backfill.go`
- Create: `api/internal/material/migration/backfill_test.go`
- Create: `docs/migrations/legacy-reward-allocation.md`

**Interfaces:**
- Produces checksummed exact allocation provenance for current balances and revision-one materials.

- [ ] **Step 1: Write migration fixture tests**

For each legacy user, provide exact solve allocation vectors and explicit non-solve residual. Assert manifest checksum, sum equals legacy balance, each baseline solve transition gets exact reward provenance, unsolve reverses exact allocation, no current formula inference, rerun idempotency, mismatch quarantine, and rollback.

Material tests create stable identity/revision one/source/artifact/slug pointers, quarantine unsafe or checksum-mismatched content, and retain legacy IDs only in migration mapping.

- [ ] **Step 2: Run migration tests**

Run:

```bash
cd api
go test -tags=integration ./internal/cultivation/migration ./internal/material/migration -count=1
```

Expected: FAIL until manifests/backfills exist.

- [ ] **Step 3: Implement evidence-required migration**

Migration refuses to run without a complete checksummed allocation manifest. It records solver/source vectors, residual reason, source dataset checksum, tool version, and reviewer metadata. It never calculates historical allocations from today’s reward rule.

- [ ] **Step 4: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/cultivation/migration ./internal/material/migration -count=20
go test ./...
```

Expected: all exit 0 for complete fixtures and deliberate refusal for incomplete evidence. Do not commit.

### Task 11: Expose Complete Domain/Admin APIs and Run Acceptance Gates

**Files:**
- Modify: `api/internal/cultivation/adapters/driver/http/element.go`
- Modify: `api/internal/cultivation/adapters/driver/http/gacha.go`
- Modify: `api/internal/cultivation/adapters/driver/http/handler.go`
- Modify: `api/internal/cultivation/adapters/driver/http/level.go`
- Modify: `api/internal/cultivation/adapters/driver/http/rank.go`
- Modify: `api/internal/cultivation/adapters/driver/http/rarity.go`
- Modify: `api/internal/cultivation/adapters/driver/http/ranking.go`
- Modify: `api/internal/cultivation/adapters/driver/http/trait.go`
- Modify: `api/internal/cultivation/adapters/driver/http/user_element_exp.go`
- Modify: `api/internal/cultivation/adapters/driver/http/user_stats.go`
- Modify: `api/internal/cultivation/adapters/driver/http/user_trait.go`
- Modify: `api/internal/cultivation/di/di.go`
- Modify: `api/internal/material/adapters/driver/http/handler.go`
- Modify: `api/internal/material/adapters/driver/http/material.go`
- Modify: `api/internal/material/adapters/driver/http/material_category.go`
- Modify: `api/internal/material/di/di.go`
- Create: `api/internal/cultivation/adapters/driver/http/http_test.go`
- Create: `api/internal/material/adapters/driver/http/http_test.go`
- Create: `api/internal/application/integration/admin_test.go`
- Modify: `docs/BACKEND_GUIDE.md`

**Interfaces:**
- Produces real APIs for onboarding offers, cultivation history/rankings, material catalog/article/authoring, admin overview/audit/operations, and every typed admin workflow.

- [ ] **Step 1: Write route/data/capability tests**

Enumerate public/authenticated/admin routes, UUID types, pagination, visibility, CSRF, capability, recent reauth, optimistic conflicts, stable errors/CID, offer quota, ledger history, safe material artifact, audit redaction, and operation progress.

- [ ] **Step 2: Run HTTP/integration tests**

Run:

```bash
cd api
go test ./internal/cultivation/adapters/driver/http \
  ./internal/material/adapters/driver/http ./internal/application/integration -count=1
```

Expected: FAIL until routes are complete.

- [ ] **Step 3: Register final handlers and remove mock/legacy contracts**

Handlers use typed DTOs/services and shared middleware. Remove `GET /traits/roll`, mutable material-body response assumptions, direct balance mutation endpoints, and any admin handler writing foreign repositories.

- [ ] **Step 4: Run reconciliation and failure gates**

Rebuild progression/rankings from ledger/history, crash onboarding consume/issuance boundaries, retry reward events, interrupt material publish, resume bulk operations, and verify audit/outbox invariants.

- [ ] **Step 5: Verify complete plan**

Run:

```bash
cd api
go test ./internal/cultivation/... ./internal/material/... \
  ./internal/platform/operation ./internal/application/query/... \
  ./internal/application/workflow/admincommand
go test -race ./internal/cultivation/... ./internal/material/... \
  ./internal/platform/operation
go test -tags=integration ./internal/cultivation/migration \
  ./internal/material/migration ./internal/application/integration
go test ./...
git diff --check
```

Expected: all exit 0. Do not commit.

## Acceptance Criteria Reference

- [ ] Trait offers are durable, versioned, fair, bounded, and cannot be forged by the client.
- [ ] Registration consumes exactly one current offer in the same account/session transaction.
- [ ] Every progression change has immutable exact-source ledger provenance.
- [ ] Legacy rewards are reconstructed only from a checksummed exact allocation manifest.
- [ ] Rankings are deterministic, reconcilable, and keyset-paginated.
- [ ] Materials use immutable revisions and shared sanitized artifacts.
- [ ] Search/view behavior remains PostgreSQL-backed and privacy-bounded.
- [ ] Admin overview is read composition, not a god-domain.
- [ ] Bulk operations are durable/resumable/idempotent.
- [ ] Every admin mutation enforces capability, preview/confirmation, reason, version, audit, and outbox.
- [ ] No legacy roll/direct-balance/mutable-body production route remains.
- [ ] No commit was created.

### Task 4: Rebuild Rankings from Versioned Cultivation and Rating Projections

**Files:**
- Create: `api/internal/ent/schema/cultivation_ranking_projection.go`
- Modify: `api/internal/cultivation/core/dto/ranking.go`
- Modify: `api/internal/cultivation/ports/ranking.go`
- Modify: `api/internal/cultivation/adapters/driven/db/user_stats.go`
- Modify: `api/internal/cultivation/core/service/ranking.go`
- Modify: `api/internal/cultivation/core/service/ranking_test.go`

**Interfaces:**

```go
type RankingCursor struct {
    ProjectionVersion int64
    Score int64
    UserID uuid.UUID
}
```

- [ ] **Step 1: Write stable pagination and reconciliation tests**

Cover deterministic tie-break, UUID string IDs, versioned cursor, public privacy mapping, element/root/tag filters, invalid cursor, concurrent projection update, full reconciliation from ledger/rating history, and cache invalidation.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/cultivation/core/service -run TestRanking -count=1`

Expected: new cursor/reconciliation tests fail.

- [ ] **Step 3: Implement projection and keyset queries**

Consume ledger/rating events idempotently, advance per-user/global projection versions, and query with allowlisted keyset ordering. Cache only bounded derived pages and invalidate by projection version; PostgreSQL remains canonical.

- [ ] **Step 4: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/cultivation/core/service -run TestRanking -count=50
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 5: Version Material Identity, Revisions, Slugs, and Render Artifacts

**Files:**
- Modify: `api/internal/ent/schema/material.go`
- Modify: `api/internal/ent/schema/material_category.go`
- Create: `api/internal/ent/schema/material_revision.go`
- Create: `api/internal/ent/schema/material_slug.go`
- Create: `api/internal/ent/schema/material_tag.go`
- Create: `api/internal/material/core/entity/revision.go`
- Modify: `api/internal/material/core/entity/material.go`
- Modify: `api/internal/material/core/dto/material.go`
- Modify: `api/internal/material/ports/material.go`
- Modify: `api/internal/material/adapters/driven/db/material.go`
- Modify: `api/internal/material/core/service/material.go`
- Modify: `api/internal/material/core/service/material_test.go`

**Interfaces:**

```go
type MaterialState string

const (
    MaterialDraft MaterialState = "draft"
    MaterialPublished MaterialState = "published"
    MaterialArchived MaterialState = "archived"
)

type PublishMaterialCommand struct {
    MaterialID uuid.UUID
    DraftRevisionID uuid.UUID
    ExpectedVersion int64
    Reason string
}
```

- [ ] **Step 1: Write revision/slug/visibility tests**

Test immutable raw revision, exact material render-source FK/checksum, deterministic safe artifact, unique normalized slug with retained redirect history, category/tag normalization, draft/editor/public state, scheduled publish if supported, optimistic conflict, archive, rollback, and old published revision preserved.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/material/core/service -run TestMaterial -count=1`

Expected: new revision/publication tests fail.

- [ ] **Step 3: Implement shared rendering and pointer switch**

Editing normalizes bounded Markdown once, hashes exact bytes, registers `material_body` source, renders immutable artifact, and stores composite FK on revision. Publishing locks stable material and advances active pointer with slug/audit/outbox in one transaction.

- [ ] **Step 4: Generate and verify**

Run:

```bash
cd api
go generate ./internal/ent
go test ./internal/material/... -run TestMaterial -count=50
go test ./...
```

Expected: all exit 0. Do not commit.

## Final Acceptance Gate

After Task 11, re-evaluate every checkbox in **Acceptance Criteria Reference**. The plan exits only when each item has direct test or retained migration/operational evidence.
