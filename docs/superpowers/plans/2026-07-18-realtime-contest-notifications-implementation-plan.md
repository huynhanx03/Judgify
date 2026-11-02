# Judgify Realtime, Contest, and Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace SSE and polling with authorized replayable WebSocket delivery, persist notifications, and make contest lifecycle/scoring/freeze/rating correct under concurrency, retries, rejudge, and restart.

**Architecture:** Domain transactions append versioned PostgreSQL outbox rows. Serialized replay projection assigns committed contiguous cursors; the in-process WebSocket adapter only routes already authorized browser-safe events. Contest remains the owner of lifecycle, registration, scoring, standings, freeze, announcements, clarifications, and rating; Notification owns persisted inbox state and campaign delivery.

**Tech Stack:** Go/Ent/PostgreSQL, go-common WebSocket Hub, PostgreSQL outbox/inbox/leases/`LISTEN` wakeups, shared content renderer, native browser WebSocket, and existing Contest module.

## Global Constraints

- No final SSE endpoint or contest/submission polling path.
- One `/ws` endpoint supports explicit anonymous public topics and authenticated private topics.
- Present-invalid credentials reject the upgrade; they never become anonymous.
- PostgreSQL is the event source of truth; Hub delivery is best effort.
- Global replay cursor is serialized committed order, not raw PostgreSQL sequence allocation order.
- Each aggregate uses one monotonic `aggregate_version`; there is no separate frontend phase version.
- Client applies exactly local version + 1, ignores stale/repeated versions, and REST-resyncs on gaps.
- Every subscription is authorized by principal, topic, ownership, contest visibility/state, and capability.
- Slow clients cannot allocate unbounded memory; critical pressure closes 1013 and requires resync.
- Initial contest scoring is deterministic ICPC-style; rating starts at 1200 with `expected-rank-v1`.
- Rating application uses one global sequence and supports deterministic suffix rebuild/shadow comparison.
- Notification campaigns require durable exact preview confirmation and idempotent lost-response recovery.
- No commits.

---

## Required Execution Order

The numeric task ID is authoritative: **1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12**. Task sections are grouped in this document for domain review; an implementer must jump to and finish the next numeric ID before starting a later one.

### Task 1: Complete Durable Outbox Dispatch, Inbox, and Consumer Leases

**Files:**
- Create: `api/internal/ent/schema/consumer_inbox.go`
- Create: `api/internal/ent/schema/consumer_checkpoint.go`
- Modify: `api/internal/platform/eventing/outbox.go`
- Create: `api/internal/platform/eventing/inbox.go`
- Create: `api/internal/platform/eventing/dispatcher.go`
- Create: `api/internal/platform/eventing/dispatcher_test.go`
- Create: `api/internal/platform/eventing/integration_test.go`

**Interfaces:**

```go
type Handler interface {
    Name() string
    Handle(ctx context.Context, event eventing.Event) error
}
```

- [ ] **Step 1: Write transaction, duplicate, and lease tests**

Reuse the already tested transaction-bound outbox appender from the Identity foundation. Assert inbox exactly-once effect under at-least-once delivery, handler error retry/backoff/dead state, lease expiry/reclaim, checkpoint monotonicity, caller cancellation, panic containment, restart recovery, and that dispatcher mutation cannot change immutable event identity/payload.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/platform/eventing -count=1`

Expected: FAIL because eventing schemas/packages are absent.

- [ ] **Step 3: Implement transactional append and leased dispatch**

Appender requires transaction context for domain events. Dispatcher claims bounded batches with `FOR UPDATE SKIP LOCKED`, restores correlation/causation into context, executes handlers outside claim transaction, and commits inbox/effect/checkpoint through handler-owned UnitOfWork. PostgreSQL `NOTIFY` only wakes the poller; polling remains correctness path.

- [ ] **Step 4: Generate schema and verify**

Run:

```bash
cd api
go generate ./internal/ent
go test -tags=integration ./internal/platform/eventing -count=50
go test -race ./internal/platform/eventing -count=10
```

Expected: all exit 0. Do not commit.

### Task 6: Expand Contest Lifecycle, Problems, Registration, and Capacity

**Files:**
- Modify: `api/internal/ent/schema/contest.go`
- Modify: `api/internal/ent/schema/contest_registration.go`
- Create: `api/internal/ent/schema/contest_problem.go`
- Create: `api/internal/ent/schema/contest_transition.go`
- Create: `api/internal/ent/schema/contest_submission_sequence.go`
- Modify: `api/internal/contest/core/entity/contest.go`
- Modify: `api/internal/contest/core/entity/contest_registration.go`
- Modify: `api/internal/contest/ports/contest.go`
- Modify: `api/internal/contest/ports/registration.go`
- Modify: `api/internal/contest/core/service/contest.go`
- Modify: `api/internal/contest/core/service/registration.go`
- Modify: `api/internal/contest/adapters/driven/db/contest.go`
- Modify: `api/internal/contest/adapters/driven/db/registration.go`

**Interfaces:**

```go
type ContestState string

const (
    ContestDraft ContestState = "draft"
    ContestScheduled ContestState = "scheduled"
    ContestRunning ContestState = "running"
    ContestEnded ContestState = "ended"
    ContestCancelled ContestState = "cancelled"
)

type RegistrationCommand struct {
    ContestID uuid.UUID
    UserID uuid.UUID
    ExpectedContestVersion int64
    IdempotencyKey uuid.UUID
}
```

- [ ] **Step 1: Write lifecycle/capacity race tests**

Cover valid transition graph, database time, immutable contest problem/judge snapshots, scheduled start/end reconciler, manual emergency end/cancel, capacity under concurrent registrations, eligibility/status, unregister boundary, suspended user, duplicate idempotency, and compatible locks with submission intake/end/ban.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/contest/core/service -run 'Test(Contest|Registration)' -count=1`

Expected: new lifecycle/capacity cases fail.

- [ ] **Step 3: Implement locked transitions and registrations**

All commands lock contest first, use `SELECT clock_timestamp()` for authoritative time, verify optimistic version, insert immutable transition/audit/outbox, and update state. Registration capacity is reserved in the same transaction; no read-count-then-insert race.

- [ ] **Step 4: Replace blocking orchestrator**

Convert `contest_orchestrator.go` into a lifecycle component using caller context, bounded reconciliation batches, database advisory lease if needed, and idempotent state commands. It does not block API bootstrap synchronously.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go generate ./internal/ent
go test -tags=integration ./internal/contest/core/service -run 'Test(Contest|Registration)' -count=50
go test -race ./internal/contest/core/service -count=10
```

Expected: all exit 0. Do not commit.

### Task 7: Add Announcements and Clarifications with Immutable Rendering

**Files:**
- Create: `api/internal/ent/schema/contest_content_revision.go`
- Create: `api/internal/ent/schema/contest_announcement.go`
- Create: `api/internal/ent/schema/contest_clarification.go`
- Create: `api/internal/contest/core/entity/content.go`
- Create: `api/internal/contest/core/service/content.go`
- Create: `api/internal/contest/core/service/content_test.go`
- Create: `api/internal/contest/adapters/driven/db/content.go`
- Create: `api/internal/contest/adapters/driver/http/content.go`

**Interfaces:**

```go
type ContentKind string

const (
    ContestDescriptionKind ContentKind = "description"
    AnnouncementKind ContentKind = "announcement"
    ClarificationQuestionKind ContentKind = "clarification_question"
    ClarificationAnswerKind ContentKind = "clarification_answer"
)
```

- [ ] **Step 1: Write visibility/render tests**

Test exact render-source kind/revision/checksum FK, immutable revisions, participant/public/admin visibility, private clarification ownership, publish timing, edit creates new revision, unsafe content rejection, and one browser-safe event after commit.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/contest/core/service -run TestContent -count=1`

Expected: FAIL because content model is absent.

- [ ] **Step 3: Implement shared renderer integration**

Register normalized source and render artifact through `platform/contentrender`; store exact pointers on contest revision. Publication and outbox commit atomically. Public DTO returns only sanitized artifact and allowed metadata.

- [ ] **Step 4: Verify**

Run:

```bash
cd api
go test ./internal/contest/core/service -run TestContent -count=50
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 8: Recompute Deterministic ICPC Contributions and Standings

**Files:**
- Modify: `api/internal/ent/schema/contest_standing.go`
- Create: `api/internal/ent/schema/contest_problem_contribution.go`
- Create: `api/internal/ent/schema/contest_standing_projection.go`
- Modify: `api/internal/contest/core/entity/contest_standing.go`
- Modify: `api/internal/contest/core/service/standing.go`
- Modify: `api/internal/contest/adapters/driven/db/standing.go`
- Create: `api/internal/contest/core/service/scoring.go`
- Create: `api/internal/contest/core/service/scoring_test.go`
- Create: `api/internal/contest/core/service/freeze_test.go`

**Interfaces:**

```go
type Contribution struct {
    ContestID uuid.UUID
    UserID uuid.UUID
    ProblemID uuid.UUID
    Solved bool
    WrongAttempts int
    SolveElapsedMS *int64
    PenaltyMS int64
    Version int64
}

func Recompute(policy ScoringPolicy, submissions []SubmissionContribution) (Contribution, error)
```

- [ ] **Step 1: Write scoring/order/freeze tests**

Use ascending `(contest_submission_sequence,submission_id)`, snapshotted elapsed time, wrong attempts before first accepted, ignored after solve, rejudge changing verdict but not time/sequence, tie ordering, contribution version gap reload, public frozen projection, participant own live row policy, admin live view, unfreeze/end reconciliation, and duplicate event idempotency.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/contest/core/service -run 'Test(Scoring|Freeze)' -count=1`

Expected: new deterministic/rejudge/freeze tests fail.

- [ ] **Step 3: Implement recompute-from-canonical contributions**

Consumer locks one contestant/problem projection, loads canonical active judgement contributions in exact sequence, recomputes rather than incrementing uncertain state, advances projection/standing aggregate version, and appends one outbox event. Version gaps trigger full affected-key reload.

- [ ] **Step 4: Implement freeze-safe queries**

Persist enough projection data to return public frozen standings without leaking post-freeze verdict/timing through totals, ranks, diffs, notifications, or WebSocket payload. Admin override requires capability and audit.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/contest/core/service -run 'Test(Scoring|Freeze)' -count=100
go test -race ./internal/contest/core/service -count=10
```

Expected: all exit 0. Do not commit.

### Task 9: Implement Global Sequenced `expected-rank-v1` Rating

**Files:**
- Modify: `api/internal/ent/schema/rating_history.go`
- Create: `api/internal/ent/schema/rating_run.go`
- Create: `api/internal/ent/schema/rating_projection_generation.go`
- Modify: `api/internal/contest/core/entity/rating_history.go`
- Modify: `api/internal/contest/core/service/rating.go`
- Modify: `api/internal/contest/adapters/driven/db/rating_history.go`
- Create: `api/internal/contest/core/service/rating_algorithm.go`
- Create: `api/internal/contest/core/service/rating_algorithm_test.go`
- Create: `api/internal/contest/core/service/rerating.go`
- Create: `api/internal/contest/core/service/rerating_test.go`

**Interfaces:**

```go
const InitialRating = 1200
const RatingAlgorithmVersion = "expected-rank-v1"

type RatingInput struct {
    UserID uuid.UUID
    Rank int
    PreviousRating int
}

func Calculate(inputs []RatingInput) ([]RatingDelta, error)
```

- [ ] **Step 1: Write deterministic algorithm/sequence tests**

Golden vectors cover initial 1200, ties, stable ordering, conservation/bounds policy, missing user history, rerun equality, one global apply sequence, concurrent contest ends, rejudge invalidation, suffix shadow rebuild, pointer swap, rollback, and crash resume.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/contest/core/service -run 'Test(Rating|Rerating)' -count=1`

Expected: FAIL until versioned algorithm/run chain exists.

- [ ] **Step 3: Implement immutable run chain**

Lock global rating sequence, snapshot finalized contest standings, calculate with exact version, store immutable before/delta/after rows and input checksum, then advance active generation/run pointer atomically. Never default a missing previous value differently from 1200.

- [ ] **Step 4: Implement shadow suffix rebuild**

When historical result changes, find first affected run, build a new projection generation through the complete suffix without altering active reads, compare checksums/invariants, then atomically swap active pointer. Retain old generation for rollback.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/contest/core/service -run 'Test(Rating|Rerating)' -count=100
go test -race ./internal/contest/core/service -run 'Test(Rating|Rerating)' -count=10
```

Expected: all exit 0. Do not commit.

### Task 10: Expose Contest/Notification APIs and Closed Event Catalog

**Files:**
- Modify: `api/internal/contest/adapters/driver/http/contest.go`
- Modify: `api/internal/contest/adapters/driver/http/registration.go`
- Modify: `api/internal/contest/adapters/driver/http/standing.go`
- Modify: `api/internal/contest/adapters/driver/http/rating.go`
- Modify: `api/internal/contest/adapters/driver/http/handler.go`
- Create: `api/internal/contest/adapters/driver/http/http_test.go`
- Modify: `api/internal/platform/realtime/catalog.go`
- Create: `api/internal/platform/realtime/catalog_test.go`

**Interfaces:**
- Produces bounded REST snapshots with cursor and browser events such as contest lifecycle, own submission progress, standings projection, announcement, clarification, rating, and persisted notification updates.

- [ ] **Step 1: Write authorization and payload tests**

Assert public/authenticated/admin route matrices, registration requirements, freeze-safe fields, own/private clarification, rating history visibility, stable pagination, snapshot cursor, event type/schema/version, one source event→one topic projection, no hidden tests/source/policy/contact data, and CID errors.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/contest/adapters/driver/http ./internal/platform/realtime \
  -run 'Test(HTTP|Catalog)' -count=1
```

Expected: FAIL until approved APIs/catalog are implemented.

- [ ] **Step 3: Implement dedicated projections**

Handlers return explicit DTOs by audience. Snapshot queries and cursor share one repeatable-read transaction. Event catalog owns mapper per safe event type and rejects any unregistered outbox type.

- [ ] **Step 4: Verify**

Run:

```bash
cd api
go test ./internal/contest/... ./internal/notification/... ./internal/platform/realtime -count=50
go test -race ./internal/platform/realtime ./internal/contest/core/service -count=10
```

Expected: all exit 0. Do not commit.

### Task 11: Migrate Legacy Contest/SSE Data and Remove SSE

**Files:**
- Create: `api/internal/contest/migration/backfill.go`
- Create: `api/internal/contest/migration/backfill_test.go`
- Create: `api/internal/platform/realtime/migration/backfill.go`
- Create: `api/internal/platform/realtime/migration/backfill_test.go`
- Remove after switch: `api/internal/contest/store/hub.go`
- Modify: `api/internal/contest/adapters/driver/http/standing.go`
- Modify: `api/internal/contest/di/di.go`
- Modify: `api/internal/infrastructure/router.go`

**Interfaces:**
- Produces revision-one contest snapshots/projections/rating chain and replay starting cursor without inventing event history.

- [ ] **Step 1: Write idempotent backfill tests**

Seed legacy contests/registrations/standings/rating rows; assert exact state mapping, immutable problem snapshot, contribution reconstruction/quarantine, sequence-zero rating opening generation, checksum verification, replay cursor starts only from new committed safe events, and rerun creates no duplicates.

- [ ] **Step 2: Run migration tests**

Run:

```bash
cd api
go test -tags=integration ./internal/contest/migration ./internal/platform/realtime/migration -count=1
```

Expected: FAIL until migrations exist.

- [ ] **Step 3: Implement expand/backfill/switch**

Keep old read paths only during backfill. Install new active pointers transactionally, enable projectors/WebSocket, switch frontend contract, then remove SSE route and polling compatibility before production. Destructive table/column removal waits for a later contract release.

- [ ] **Step 4: Verify no SSE remains**

Run:

```bash
rg -n 'text/event-stream|new EventSource|use-contest-sse|/sse' api website
```

Expected after the frontend plan’s switch: no production match. Documentation/migration prose may mention historical SSE.

### Task 12: Run Realtime/Contest/Notification Crash and Load Gates

**Files:**
- Create: `api/internal/platform/realtime/integration/replay_test.go`
- Create: `api/internal/platform/realtime/integration/load_test.go`
- Create: `api/internal/contest/integration/scoring_test.go`
- Create: `api/internal/contest/integration/rating_test.go`
- Create: `api/internal/notification/integration/campaign_test.go`

**Interfaces:**
- Produces retained evidence for reconnect storms, cursor correctness, contest concurrency, rating rebuild, and campaign recovery.

- [ ] **Step 1: Inject commit/dispatch/project/broadcast crashes**

Kill after domain commit, outbox claim, inbox effect, replay projection, and before/after Hub broadcast. Assert canonical REST state plus replay recovers without duplicate business effects.

- [ ] **Step 2: Run concurrency and slow-client tests**

Use 1,000 simultaneous WebSockets, reconnect storm, anonymous/IP quotas, slow critical clients, policy/session revocation, 1013 resync, replay retention expiry, and bounded Hub/process memory.

- [ ] **Step 3: Run contest/rating/campaign races**

Concurrent end/submission/rejudge/registration; freeze leak probes; two contests ending together; historical rerate suffix rebuild; campaign schedule lost response; recipient eligibility changes; delivery retry/dead transition.

- [ ] **Step 4: Verify the complete plan**

Run:

```bash
cd api
go test ./internal/platform/eventing ./internal/platform/realtime \
  ./internal/notification/... ./internal/contest/...
go test -race ./internal/platform/eventing ./internal/platform/realtime \
  ./internal/notification/... ./internal/contest/core/service
go test -tags=integration ./internal/platform/realtime/integration \
  ./internal/contest/integration ./internal/notification/integration
go test ./...
git diff --check
```

Expected: all exit 0 and load metrics remain within configured bounds. Do not commit.

## Acceptance Criteria Reference

- [ ] Outbox/inbox survives retry/restart and effects are idempotent.
- [ ] Replay cursor reflects serialized committed projection order and is contiguous.
- [ ] Upgrade, replay, and every topic subscription are authorized.
- [ ] Anonymous clients can access only the explicit public catalog.
- [ ] Persisted notifications remain canonical without WebSocket delivery.
- [ ] Campaign confirmation binds exact actor/session/content/audience/version/schedule.
- [ ] Contest lifecycle/registration/intake locks close concurrency gaps.
- [ ] ICPC contributions recompute deterministically from canonical active judgements.
- [ ] Freeze leaks no post-freeze information.
- [ ] `expected-rank-v1` starts at 1200 and applies in one global sequence.
- [ ] Historical rerating rebuilds a verified suffix before pointer swap.
- [ ] No production SSE or polling path remains.
- [ ] No commit was created.

### Task 2: Build the Serialized Replay Projection

**Files:**
- Create: `api/internal/ent/schema/realtime_event.go`
- Create: `api/internal/ent/schema/realtime_cursor_state.go`
- Create: `api/internal/platform/realtime/catalog.go`
- Create: `api/internal/platform/realtime/projector.go`
- Create: `api/internal/platform/realtime/repository.go`
- Create: `api/internal/platform/realtime/projector_test.go`
- Create: `api/internal/platform/realtime/replay_test.go`

**Interfaces:**

```go
type BrowserEvent struct {
    Cursor           int64
    EventID          uuid.UUID
    Topic            string
    Type             string
    AggregateID      uuid.UUID
    AggregateVersion int64
    OccurredAt       time.Time
    Payload          json.RawMessage
}

type ReplayQuery struct {
    Principal authentication.Principal
    After     int64
    Limit     int
}
```

- [ ] **Step 1: Write commit-order and gap tests**

Simulate transactions allocating source sequence values then committing in reverse order. Assert replay cursors are contiguous in projector commit order, one source event produces at most one projection/topic mapping, duplicate handling is idempotent, retention reports `resync_required`, unauthorized rows are not returned, and snapshot+cursor fence has no subscribe gap.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/platform/realtime -count=1`

Expected: FAIL because replay projection does not exist.

- [ ] **Step 3: Implement one serialized writer**

Projector locks singleton cursor state, reads next eligible committed outbox event in deterministic order, validates it against a closed browser-safe catalog, inserts realtime row with cursor+1, marks source projected, and commits. Do not expose arbitrary outbox payloads.

- [ ] **Step 4: Implement authorized replay and snapshot fence**

Replay query first checks topic/object authorization against current canonical state, then returns bounded ordered events. Snapshot endpoints read domain snapshot plus current cursor in one repeatable-read transaction. Subscription registers behind the projector fence and then replays after the snapshot cursor.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/platform/realtime -count=100
go test -race ./internal/platform/realtime -count=10
```

Expected: cursors are contiguous and no authorized event is missed across the fence. Do not commit.

### Task 3: Integrate the Judgify WebSocket Gateway

**Files:**
- Create: `api/internal/platform/realtime/gateway.go`
- Create: `api/internal/platform/realtime/principal.go`
- Create: `api/internal/platform/realtime/topics.go`
- Create: `api/internal/platform/realtime/authorizer.go`
- Create: `api/internal/platform/realtime/gateway_test.go`
- Modify: `api/internal/infrastructure/router.go`
- Modify: `api/internal/infrastructure/run.go`
- Modify as a temporary compatibility adapter: `api/internal/contest/store/hub.go`

**Interfaces:**

```go
type TopicService interface {
    AuthorizeSubscribe(ctx context.Context, principal authentication.Principal, topic string) error
}

type Gateway interface {
    Handler() http.Handler
    Run(ctx context.Context) error
    Shutdown(ctx context.Context) error
    CloseSessions(ctx context.Context, sessionIDs []uuid.UUID, reason string) error
}
```

- [ ] **Step 1: Write upgrade/topic/revocation tests**

Cover cookie authentication, absent credentials explicit anonymous, invalid cookie reject, public closed catalog, private user/submission/contest/admin topics, ownership/registration/freeze rules, unauthorized topic probing, exact origin, one connection identity, session/account/policy committed close, rollback leaves socket open, reconnect/replay, and no direct Hub bypass from domain code.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/platform/realtime -run TestGateway -count=1`

Expected: FAIL because gateway is absent.

- [ ] **Step 3: Implement resolver and authorizer**

Resolver uses Identity session authenticator and returns anonymous only when auth cookies are absent. Authorizer parses topic through a closed grammar and delegates object checks to narrow module contracts. Anonymous can subscribe only to explicitly public active-contest topics; empty subject is never a target.

- [ ] **Step 4: Implement outbox-to-Hub delivery**

After replay row commit, offer its envelope to go-common Hub by topic/delivery class. Broadcast failure never rolls back domain/replay state. On reconnect the browser supplies cursor and receives replay or `resync_required`.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/platform/realtime -run TestGateway -count=100
go test -race ./internal/platform/realtime -count=20
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 4: Add Persisted Notifications

**Files:**
- Create: `api/internal/ent/schema/notification.go`
- Create: `api/internal/ent/schema/notification_preference.go`
- Create: `api/internal/notification/core/entity/notification.go`
- Create: `api/internal/notification/core/dto/notification.go`
- Create: `api/internal/notification/ports/notification.go`
- Create: `api/internal/notification/adapters/driven/db/notification.go`
- Create: `api/internal/notification/core/service/notification.go`
- Create: `api/internal/notification/core/service/consumer.go`
- Create: `api/internal/notification/core/service/notification_test.go`
- Create: `api/internal/notification/adapters/driver/http/notification.go`
- Create: `api/internal/notification/adapters/driver/http/http_test.go`
- Create: `api/internal/notification/di/di.go`

**Interfaces:**

```go
type CreateFromEvent struct {
    UserID       uuid.UUID
    SourceEventID uuid.UUID
    Type         string
    Title        string
    BodyArtifactID *uuid.UUID
    Target       Target
    OccurredAt   time.Time
}
```

- [ ] **Step 1: Write idempotency and read-state tests**

Assert unique `(user_id,source_event_id,type)`, bounded safe title/target, unread count, list pagination, read/read-all/archive optimistic behavior, user ownership, preferences only for supported channels, outbox on committed state, and WebSocket as acceleration only.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/notification/... -count=1`

Expected: FAIL because module does not exist.

- [ ] **Step 3: Implement event consumers and APIs**

Consumers re-authorize recipient eligibility from canonical state, insert notification and notification outbox atomically, and mark inbox in same transaction. REST is canonical for list/count/state; realtime sends `notification.created.v1`/count updates after commit.

- [ ] **Step 4: Verify**

Run:

```bash
cd api
go generate ./internal/ent
go test ./internal/notification/... -count=100
go test -race ./internal/notification/... -count=10
```

Expected: all exit 0. Do not commit.

### Task 5: Implement Durable Admin Notification Campaigns

**Files:**
- Create: `api/internal/ent/schema/notification_campaign.go`
- Create: `api/internal/ent/schema/notification_campaign_confirmation.go`
- Create: `api/internal/ent/schema/notification_campaign_recipient.go`
- Create: `api/internal/notification/core/entity/campaign.go`
- Create: `api/internal/notification/core/service/campaign.go`
- Create: `api/internal/notification/core/service/campaign_test.go`
- Create: `api/internal/application/workflow/notificationcampaign/workflow.go`
- Create: `api/internal/application/workflow/notificationcampaign/workflow_test.go`
- Create: `api/internal/notification/adapters/driver/http/campaign.go`

**Interfaces:**

```go
type PreviewCommand struct {
    ActorSessionID uuid.UUID
    ContentRevisionID uuid.UUID
    AudienceSelector AudienceSelector
    ScheduledAt *time.Time
}

type ScheduleCommand struct {
    ConfirmationToken string
    IdempotencyKey uuid.UUID
}
```

- [ ] **Step 1: Write confirmation/lost-response/failure tests**

Bind confirmation HMAC to actor/session/campaign/exact content/audience version/dry-run result/schedule. Test expiration, altered selector/content/schedule, exact bounded audience snapshot, statuses draft/scheduled/running/completed/failed/cancelled, recipient pending/created/skipped/failed, attempts/backoff, retry after lost HTTP response returning same operation, and no duplicate notifications.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/application/workflow/notificationcampaign ./internal/notification/... -run TestCampaign -count=1`

Expected: FAIL because campaign workflow is absent.

- [ ] **Step 3: Implement bounded scheduling UnitOfWork**

Lock exact campaign/confirmation, verify HMAC/expiry/idempotency, ask Identity’s narrow participant for a hard-bounded audience snapshot, freeze content/selector, insert unique recipients, audit, outbox, and operation row in one transaction. No raw confirmation nonce or network I/O enters transaction.

- [ ] **Step 4: Implement batch delivery**

For each bounded batch, Identity rechecks active/role eligibility immediately before Notification inserts persisted row/outbox. Mark skipped/failed with bounded reason and retry schedule. Exact retry uses operation request hash and returns the same operation ID.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/application/workflow/notificationcampaign \
  ./internal/notification/... -run TestCampaign -count=50
go test -race ./internal/application/workflow/notificationcampaign -count=10
```

Expected: all exit 0. Do not commit.

## Final Acceptance Gate

After Task 12, re-evaluate every checkbox in **Acceptance Criteria Reference**. The plan exits only when each item has direct test or retained operational evidence.
