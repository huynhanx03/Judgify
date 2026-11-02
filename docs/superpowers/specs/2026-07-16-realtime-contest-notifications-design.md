# Judgify Realtime, Contest, and Notifications Design

**Date:** 2026-07-16  
**Status:** Draft for written review  
**Scope:** Single-tenant production deployment on one host, with contracts that remain valid when API instances are added later

## 1. Outcome

Judgify will replace contest SSE and submission polling with one WebSocket transport backed by committed PostgreSQL state. The same `/ws` endpoint supports authenticated principals and an explicit anonymous principal restricted to closed public-contest topics. PostgreSQL outbox rows are the durable delivery source; the in-process WebSocket hub is only a low-latency connection manager. Contest state, scoring, rating, freeze behavior, and notifications remain correct under retries, duplicate events, slow clients, worker crashes, and process restarts.

This design establishes these invariants:

- No realtime event is visible before the database transaction that produced it commits.
- REST responses remain the canonical snapshots; WebSocket events accelerate convergence.
- A reconnecting client can replay retained events or receive an explicit `resync_required` response.
- A slow client cannot create unbounded memory, goroutines, or latency for other clients.
- Contest scoring is based on contest-relative time and records every scoring verdict exactly once.
- Public frozen standings and official standings are distinct projections of the same committed attempts.
- Notifications are persisted before they are delivered and can always be recovered from the inbox API.

## 2. Ownership and boundaries

### 2.1 go-common owns the transport primitive

The reusable transport lives at:

```text
go-common/pkg/common/websocket
```

It uses `github.com/coder/websocket` as a private implementation detail. No public type, option, close code wrapper, or handler signature exposes a `coder/websocket` type. A future core swap must not change Judgify call sites.

The package owns:

- HTTP upgrade and exact-origin validation hooks.
- Connection lifecycle, read pump, single writer pump, ping/pong, deadlines, and graceful close.
- A local `Hub` with users, connections, rooms/topics, and bounded mailboxes.
- Subscribe/unsubscribe plus public Hub `Broadcast`, connection-targeted send, and principal-subject-targeted send operations.
- Typed inbound handler registration.
- A versioned envelope and protocol errors.
- Backpressure policy and connection/subscription quotas.
- Framework-neutral `http.Handler`; Gin integrates through `gin.WrapH`.
- A private local fan-out boundary that can later receive a package-owned cross-process adapter and is explicitly non-durable.

The package does not own Judgify topics, database outbox rows, contest authorization rules, notification state, or replay retention policy.

### 2.2 Judgify owns durable events and product policy

Judgify places its adapter at:

```text
api/internal/platform/realtime
```

It owns:

- PostgreSQL outbox claiming and publication.
- Product event-to-envelope mapping.
- Topic parsing and authorization.
- Replay queries and retention decisions.
- Contest, submission, notification, and admin telemetry event catalogs.
- Selecting and closing affected sockets after committed account, permission, or audience changes; reconnect performs v1 reauthorization/replay.

Business modules create outbox events in their own transaction. The realtime adapter never reconstructs business events by observing tables after the fact.

Persisted notification state is owned by `api/internal/notification`, not by realtime. Notification consumes domain events idempotently, commits notification state plus its outbox event, and exposes REST commands/queries. Realtime only authorizes and delivers the committed browser-safe projection.

## 3. go-common WebSocket contract

### 3.1 Public API shape

The stable package reuses `security/authentication.Principal` and exposes the following package-owned transport contract:

```go
type PrincipalResolver func(context.Context, *http.Request) (authentication.Principal, error)
type TopicAuthorizer func(context.Context, authentication.Principal, string) error

type Options struct {
    AllowedOrigins                   []string
    ReadLimit                        int64
    WriteQueueCapacity               int
    WriteQueueByteCapacity           int64
    MaxHubQueuedBytes                int64
    MaxOutboundMessageBytes          int64
    InboundMessagesPerSecond         int
    InboundBurst                     int
    MaxInFlightHandlersPerConnection int
    MaxInFlightHandlers              int
    HandlerTimeout                   time.Duration
    PingInterval                     time.Duration
    PongTimeout                      time.Duration
    WriteTimeout                     time.Duration
    ShutdownTimeout                  time.Duration
    MaxConnections                   int
    MaxConnectionsPerPrincipal       int
    MaxConnectionsPerRemoteIP        int
    UpgradeRatePerSecondPerRemoteIP  int
    UpgradeBurstPerRemoteIP          int
    MaxSubscriptions                 int
}

func NewHub(options Options) (*Hub, error)
func (h *Hub) Broadcast(ctx context.Context, topic string, message Message) error
func (h *Hub) SendToConnection(ctx context.Context, connectionID string, message Message) error
func (h *Hub) SendToPrincipal(ctx context.Context, subject string, message Message) error
func (h *Hub) CloseConnections(ctx context.Context, selector ConnectionSelector, options CloseOptions) (CloseResult, error)
```

`Message`, `DeliveryClass`, `Hub`, `Conn`, `Envelope`, `ConnectionSelector`, close types, and generic `Handler[T] func(context.Context, *Conn, *T) error` are the package-owned types defined in the go-common design. The signatures above are the canonical v1 design contract. Naming-only changes require a spec amendment; the following semantics are fixed:

- One read pump and exactly one writer pump per connection.
- All potentially blocking operations accept `context.Context`.
- Every package-owned pump/scheduler goroutine is owned by the hub and ends during shutdown. Application handlers are tracked and must honor context cancellation; Go cannot forcibly terminate a callback that violates that contract.
- `Hub.Shutdown(ctx)` stops upgrades, closes connections with code `1001`, cancels handlers, drains bounded work, and returns on deadline. A still-running noncompliant callback yields the typed undrained-handler result/count for lifecycle reporting rather than a false success.
- `Hub.Handler(resolve, authorize)` resolves the same typed principal used by HTTP and authorizes every subscription through the service hook.
- Server-side delivery uses only the Hub methods above; Judgify neither implements nor retains a common broker/fan-out object. Because those methods are trusted transport primitives rather than domain authorizers, only Judgify's authorized application/replay adapters receive the Hub and they validate the topic/target before calling it.
- Browser credential resolution is performed during upgrade using the same `security/authentication.Verifier` used by HTTP middleware; an intentionally anonymous result is a typed principal, not an authentication failure fallback.
- Compression is disabled by default because most events are small and compression adds memory and CPU amplification risk.
- The default inbound and outbound message limits are 64 KiB each.
- Control operations and Judgify application messages are serial per connection; any future keyed parallel handler must use the bounded opt-in contract from the go-common design.
- Per-connection rate/burst and per-connection/global in-flight limits are validated at startup, and overload never spawns an unowned handler goroutine.
- A valid `Broadcast` to a topic with no current subscribers succeeds as an expected no-op. A missing connection/principal target may return the package's typed not-found result.
- One slow destination's coalesce, drop, or close outcome is measured per destination and does not turn an otherwise valid broadcast into a global partial-failure error. Only validation/topic, oversize, Hub shutdown/closure, or caller-context failures fail the call.

### 3.2 Envelope

Every message uses the same wire envelope:

```json
{
  "v": 1,
  "op": "event",
  "id": "019f...",
  "cursor": "019f...",
  "topic": "submission:019f...",
  "type": "submission.judged.v1",
  "aggregate_version": 4,
  "occurred_at": "2026-07-16T10:30:00.000Z",
  "data": {}
}
```

Rules:

- IDs and cursors are opaque strings; clients never parse them as numbers.
- `v` versions the transport envelope. Event schema versions remain in `type`.
- `aggregate_version` supplies per-aggregate ordering and deduplication.
- `occurred_at` is RFC 3339 UTC.
- `data` contains only a browser-safe projection, never an internal outbox payload by default.
- Protocol errors use `op: "error"` with a stable code, bounded typed parameters, request ID when applicable, and `retryable` flag. The frontend maps the code through its text catalog; the wire does not supply canonical UI copy.

Initial client operations are `subscribe`, `unsubscribe`, `ping`, and optional delivery telemetry `ack`. Domain commands continue through REST so HTTP idempotency, validation, audit, and status codes remain authoritative.

### 3.3 Connection authentication and authorization

Production traffic is same-origin:

- `/api/*` routes to Go HTTP.
- `/ws` routes directly to Go WebSocket.
- Browser credentials use secure HttpOnly cookies.
- The upgrade validates an exact configured Origin allowlist; it never reflects arbitrary origins.
- When the credential cookie is absent, the resolver may return the explicit `Authenticated=false` principal. When a cookie is present, it must be a purpose-`access` token with a live session, active user, correct issuer/audience, and recognized key ID; malformed, expired, wrong-purpose, revoked, or otherwise invalid credentials reject the upgrade and never downgrade to anonymous.
- An authenticated connection cannot outlive `Principal.AuthenticatedUntil`. At access-token expiry the hub closes it with application code `4001 authentication_expired`; the frontend runs the single refresh coordinator and reconnects. Ping/pong never extends authentication. Anonymous principals have no authentication expiry/session semantics.
- Anonymous upgrades and connections are bounded by the Hub-global limit plus stricter per-remote-IP connection and upgrade token-bucket limits. Remote IP is taken from the socket peer or forwarding headers only through the configured exact trusted-proxy chain; an arbitrary `X-Forwarded-For` value is never trusted.

Each subscribe request is authorized independently. Topic names are parsed into a closed registry, not treated as arbitrary room strings.

| Topic | Access rule |
|---|---|
| `contest:{contestID}:lifecycle:public` | Anonymous or authenticated only when contest visibility is public |
| `contest:{contestID}:standings:public` | Anonymous or authenticated only when public standings are enabled; freeze projection applies |
| `contest:{contestID}:announcements:public` | Anonymous or authenticated only when contest visibility is public; contains only explicitly public announcements |
| `user:{userID}` | Same user or privileged support/admin permission |
| `submission:{submissionID}` | Submission owner or `submission.read_any` |
| `contest:{contestID}:standings:participants` | Authenticated registered participant for restricted contests; admin always allowed |
| `contest:{contestID}:announcements:participants` | Authenticated registered participant; contains no jury announcements |
| `contest:{contestID}:admin` | Contest operations permission |
| `notifications:{userID}` | Same user only |
| `admin:operations` | Explicit operations permission |

The closed topic registry defaults to deny. Anonymous principals may subscribe/replay only the three `:public` topic forms above, receive no user/submission/private-contest/admin/notification replay, and cannot invoke domain commands over WebSocket. A public projection is generated separately from participant/jury projections, so an audience change cannot leak through a shared room.

Identity first commits account/session/policy state, audit, monotonic access revision, and its outbox event in PostgreSQL; Contest visibility/audience changes do the same for their scope. No socket I/O occurs inside either transaction. After commit, an idempotent local effect keyed by event ID/revision invalidates cached topic decisions, fences new Hub admissions for the affected selector, and calls `Hub.CloseConnections`. That method marks matches closing before routing resumes, rejects new enqueue/subscription work, drops already queued frames with exact charge release, and closes best-effort. The outbox consumer repairs the effect after a crash or duplicate delivery; on additional API nodes, each node must apply committed access revisions before its local replay tailer may route later domain frames.

V1 deliberately closes and reconnects rather than reauthorizing a live subscription in place. Suspended/deleted users and revoked sessions select matching authenticated subject/session connections. A role change selects that authenticated subject. A global policy/catalog change explicitly selects all authenticated connections. A contest visibility/audience change selects its affected public/participant/admin topics across authenticated and anonymous classes. Reconnect repeats credential and topic authorization, then REST snapshot/replay restores continuity. This avoids silently retaining queued old-policy frames or inventing a cursor handoff that the generic Hub cannot prove. A future in-place reauthorization optimization requires an explicit pause, queued-frame discard, authorization-revision fence, and `resync_required` cursor contract before it can replace close/reconnect. Account/session expiry/revocation selectors do not target anonymous connections, while public visibility selectors do. Effects are idempotent and expose committed/applied revision, matched/marked/already-closing counts, and CID in bounded logs/metrics.

### 3.4 Backpressure

Each outbound message is assigned one delivery class:

1. **Latest state:** standings snapshots, queue counts, capacity summaries. Coalesce by topic and aggregate key; only the newest pending value is retained.
2. **Ephemeral:** typing/presence-style telemetry and nonessential operational samples. Drop when the mailbox is full and increment a metric.
3. **Critical durable:** verdicts, authorization changes, persistent notifications, contest lifecycle changes. Never block the hub. If the bounded mailbox cannot accept the event, send a best-effort `resync_required` reason and close with `1013 Try Again Later`.

Critical events are safe to miss at the socket layer because replay and REST snapshots are durable. No code path creates an unbounded queue to avoid disconnecting a slow client.

Every encoded envelope is rejected before routing if it exceeds `MaxOutboundMessageBytes`. Each connection mailbox enforces both `WriteQueueCapacity` and `WriteQueueByteCapacity`, while the Hub atomically enforces `MaxHubQueuedBytes` across every destination queue. Coalescing, dequeue, drop, disconnect, and shutdown release per-connection and global logical-byte charges exactly once. An immutable encoded frame may be shared physically across recipients, but every recipient is charged its full logical size. Load tests therefore assert the configured Hub/process memory ceiling, not merely a bounded number of messages.

Inbound rate/burst limits cover every decoded frame, including subscribe, unsubscribe, ping, ack, and application messages, so a client cannot CPU-flood the server with legal 64 KiB messages. Handler execution consumes both connection and Hub-global slots, has a bounded deadline, recovers panics, and is serial per connection for all launch operations. Repeated rate/overload abuse receives a stable protocol error and closes without affecting other clients.

## 4. Durable outbox, inbox, and replay

### 4.1 Outbox record

Judgify uses a PostgreSQL outbox with these logical fields:

```text
id UUIDv7 primary key
event_type string
schema_version integer
aggregate_type string
aggregate_id UUID
aggregate_version bigint
payload jsonb
realtime_projection jsonb nullable
audience jsonb nullable
consumer_set_version integer
occurred_at timestamptz
available_at timestamptz
attempts integer
lease_owner string nullable
lease_token UUID nullable
lease_until timestamptz nullable
published_at timestamptz nullable
dead_at timestamptz nullable
last_error_class string nullable
last_error_message string nullable
```

The domain write and outbox insert share one Ent transaction. An outbox dispatcher claims rows with `FOR UPDATE SKIP LOCKED`, a lease token, bounded batches, retry backoff, and dead-letter visibility. A code-owned consumer registry maps each event type/schema version and `consumer_set_version` to an immutable required-consumer set. `published_at` is set only after every required consumer reports `committed` or `duplicate`; it never means every browser received the event. Adding a consumer creates a new consumer-set version and an explicit backfill job instead of retroactively changing completion semantics for old rows.

### 4.2 Consumer inbox

Every side-effecting consumer records:

```text
event_id UUID
consumer_name string
processed_at timestamptz
result_version integer
primary key (event_id, consumer_name)
```

The side effect and inbox marker commit in the same transaction. Delivery is at-least-once; the business effect is idempotent. The realtime projector uses the cursor-bearing specialized inbox in section 4.3 instead of duplicating this generic row, because its idempotent result must retain the assigned `cursor_offset`.

### 4.3 Realtime replay

Only explicitly browser-safe projections enter the replay stream:

```text
realtime_cursor_state
  stream_key text primary key             # `global` in v1
  committed_cursor_offset bigint not null
  minimum_retained_cursor_offset bigint not null
  updated_at timestamptz

realtime_events
  cursor_offset bigint primary key
  source_event_id UUID not null unique
  topic string
  event_type string
  aggregate_id UUID
  aggregate_version bigint
  envelope jsonb
  occurred_at timestamptz
  expires_at timestamptz

realtime_projection_inbox
  source_event_id UUID primary key
  cursor_offset bigint not null unique
  processed_at timestamptz
```

Each source outbox event may carry **exactly one** browser-safe realtime projection with one canonical topic in v1. If one semantic change needs different topic/audience projections, the originating transaction or an idempotent durable derivation consumer emits a separate outbox event for each projection, with a distinct event ID and the original ID as causation metadata. The realtime projector never expands one `source_event_id` into an implicit topic list. This keeps `source_event_id` unique, makes the inbox key exactly `source_event_id`, and ensures a retry has one unambiguous cursor offset.

The realtime projector is a serialized replay writer. It processes either one source event or a bounded claimed batch in one short transaction. New events inside a batch are de-duplicated and sorted deterministically by `(occurred_at, source_event_id)`. A fast inbox precheck may avoid entering the writer, but the correctness path locks `realtime_cursor_state['global'] FOR UPDATE` **before** its authoritative `realtime_projection_inbox` recheck. Only then does it assign the next contiguous `cursor_offset` range to still-new items, insert each replay row and specialized inbox marker, advance `committed_cursor_offset` once to the last inserted row, and commit. A writer that waited for the lock must recheck; it cannot rely on a pre-lock miss and discover a uniqueness race after allocation. A duplicate reads its previously recorded `cursor_offset` without allocating another one, including after its retained replay row has expired. Batch item/byte count and lock duration are bounded; the one-event path remains the fallback under light load or contention.

The row lock is held through commit, and allocation/state advancement are transactional, so committed replay cursor offsets are contiguous and numeric order is replay-writer commit order. A PostgreSQL sequence, UUID timestamp, application clock, or independently committed counter is forbidden for this cursor because allocation order alone is not commit order. After commit the durable tailer broadcasts the committed rows in `cursor_offset` order; no row is broadcast from the uncommitted batch.

The client cursor is a bounded, integrity-protected opaque encoding of the committed cursor offset plus protocol version; clients never construct it. `occurred_at` is display metadata and is never the ordering authority. The initial retention target is seven days for verdicts, contest lifecycle, standings-change markers, and notifications; snapshot-heavy operational telemetry may have shorter retention. Retention transactionally advances `minimum_retained_cursor_offset`, never moves the committed watermark backward, preserves the durable projection inbox for the outbox redelivery horizon, and makes an older cursor return `resync_required`.

Every API instance owns a durable replay tailer. It reads exactly the next cursor offset through the committed watermark, publishes rows through the Hub in order, and advances its local delivered cursor offset only after routing finishes under the documented backpressure policy. PostgreSQL `NOTIFY` or a future fan-out adapter is only a wake-up hint; bounded polling repairs a lost hint. The tailer never skips a missing cursor offset inside the retained range. A missing row between `minimum_retained_cursor_offset` and `committed_cursor_offset` is an invariant failure that makes realtime readiness false instead of silently advancing clients.

Reconnect and gap-free handoff flow:

1. Client fetches the canonical REST snapshot and receives its aggregate version/cursor. The endpoint reads the domain projection and `realtime_cursor_state.committed_cursor_offset` in one PostgreSQL `REPEATABLE READ` transaction (or a separately proven cursor-first protocol); two autocommit reads are forbidden.
2. Client opens `/ws` and subscribes with the cursor.
3. Under the topic subscription lock, the server captures a replay-store high-water cursor and registers the connection in a buffering state.
4. The server replays authorized events after the client cursor through the captured high-water cursor, flushes any buffered events newer than that fence in cursor order, and only then marks the subscription live.
5. Client ignores duplicate or older aggregate versions.
6. If the cursor is invalid, unauthorized, or outside retention, server responds `resync_required`; the client refetches REST and subscribes again.

The snapshot's state/cursor pair comes from one consistent database snapshot, so it may conservatively return new state with an older cursor but can never return stale state with a cursor already past that state's update event. The subscription high-water value is read from `realtime_cursor_state.committed_cursor_offset`; therefore every cursor offset at or below the fence is already committed and queryable. The replay query and live registration share this explicit fence. The sequential database tailer routes every later cursor offset either into the subscription buffer or, after handoff, into the live mailbox. The per-subscription pending buffer is bounded by items and bytes; overflow returns `resync_required` and closes the subscription rather than risking a silent gap. A naive “query, then join the room,” two independent snapshot/cursor reads, and a sequence value that can become visible out of commit order are all forbidden.

On one host the committed replay writer wakes the local durable tailer, which calls the Hub's public `Broadcast`/targeted-send API. When API instances are added, each node tails the same contiguous committed stream; `LISTEN/NOTIFY` or another package-owned fan-out implementation may reduce poll latency but is never the source of truth.

## 5. Contest model and lifecycle

### 5.1 State machine

Contest lifecycle is command-driven and time-aware:

```text
draft -> upcoming -> running -> ended
  |          |          |
  +----------+----------+-> cancelled
```

- `draft`: editable and invisible to ordinary users.
- `upcoming`: published; registration policy applies; scoring configuration and problem revisions are locked except through audited republish.
- `running`: submissions accepted only from eligible participants and only for snapshotted contest problems.
- `ended`: no new official submissions; scoring is finalized; rating may run.
- `cancelled`: terminal and unrated; historical data remains inspectable.

Admin commands are `publish`, `start`, `end`, and `cancel`. Scheduled transitions use the same idempotent command handlers as manual actions. Publishing requires valid times, at least one published problem revision, a scoring policy, and a consistent freeze configuration. API startup reconciles missed scheduled transitions before readiness.

The legacy `DELETE /contests/:id` hard-delete route is retired. It is not silently translated because delete and audited cancellation have different preconditions. The admin UI switches to the versioned cancel command with reason; the coordinated release removes the legacy route, and historical contests remain inspectable.

### 5.2 Contest problem snapshot

A contest does not point only at mutable problems. `contest_problems` records:

- Contest ID and immutable problem revision ID.
- Display order and contest alias such as A/B/C.
- Optional points and penalty policy.
- Whether the problem is visible before start.
- Checker/testset revision used for official judging.

Changing a source problem after publish does not change an active or historical contest.

### 5.3 Registration and capacity

Published contests snapshot this participation policy:

```text
contests
  registration_mode registered|open
  registration_opens_at timestamptz nullable
  registration_closes_at timestamptz nullable
  max_participants integer nullable       # null means unlimited; zero is invalid
  next_submission_sequence bigint not null default 1

contest_registrations
  id UUIDv7 primary key
  contest_id UUID
  user_id UUID
  status registered|withdrawn|banned
  source explicit|implicit_open|admin
  version bigint
  registered_at timestamptz nullable
  withdrawn_at timestamptz nullable
  banned_at timestamptz nullable
  reason string nullable
  unique (contest_id, user_id)
```

For `registered` mode, publishing requires non-null `registration_opens_at <= registration_closes_at <= start_at`; explicit register and withdraw commands are idempotent, versioned, and allowed only inside that window. For `open` mode those fields are null, explicit preregistration is disabled, and the official running window is the implicit participation window. Withdrawal after an official contest submission is forbidden. A ban is an audited contest-operation command, remains effective regardless of registration mode, and never disappears through re-registration.

Every register, withdraw, ban, start, end, cancel, and participant-reservation command locks the contest row `FOR UPDATE` before checking state. Registration counts `status = registered` under that lock and rejects the command with stable `contest_full` when the configured maximum is reached. The unique key prevents duplicates, while the common contest lock prevents two concurrent final slots from both succeeding. Cached participant counts are projections only.

For `open` contests, the first successfully admitted official submission command atomically creates or reactivates an `implicit_open` registration under the same lock and consumes capacity. Subsequent submissions reuse that row. A publish command may combine open participation with a capacity limit because this implicit reservation is atomic; it may not rely on an eventually consistent count.

Public APIs provide contest policy/capacity and current registration state; explicit register/withdraw commands are exposed only for `registered` mode, while `open` mode reports that participation is reserved by the first admitted official submission. Admin APIs list bounded participant pages and provide audited register, withdraw, and ban commands with expected version and reason. Participant identity is not exposed by public capacity APIs. Each committed change emits `contest.registration.changed.v1`; browser-safe projections include no ban reason.

### 5.4 Submission eligibility

Submission intake orchestrates one platform `UnitOfWork`; Contest does not read or own Account state. The top-level intake first calls Identity's transaction-bound eligibility check, which locks/protects the account eligibility revision through commit and rejects inactive/suspended/deleted accounts. Submission then performs its own idempotency, rate, and concurrent-job checks. Only after those module-owned checks does it call Contest's transaction-bound `ReserveContestSubmission`. This Identity → Submission → Contest lock order is shared by every cross-module intake path to avoid deadlocks.

`ReserveContestSubmission` locks the contest, contest-registration, and required contest-problem rows in their canonical order. After all potentially blocking Contest locks have been acquired, it reads `clock_timestamp()` exactly once and uses that value for every time decision and persisted submission timestamp. Contest validates only its own invariants:

- Contest is `running` and `start_at <= db_now < end_at`; exact start is admissible and exact end is not.
- User is registered unless the contest explicitly permits open participation, has not withdrawn, and is not contest-banned.
- Requested contest-problem ID belongs to the contest and resolves to its exact immutable problem, testset, checker, and runtime-policy snapshot.
- Capacity and the command's contest reservation/sequence invariants still hold under the contest lock.

For open participation the reservation first performs the capacity-safe implicit registration described above. It then takes the current `next_submission_sequence`, increments the contest counter transactionally, sets `submitted_at = db_now`, and computes `submission_elapsed_ms = max(0, floor((db_now - start_at) / 1 millisecond))`. It returns those values with the immutable judge snapshot. Submission persists them without rereading the mutable Problem publication pointer, then creates the submission, judge specification, job, and outbox event in the same transaction. If any later intake write fails, the registration/counter update and all module writes roll back together.

The resulting submission stores contest ID, contest-problem ID, contestant ID, database/server `submitted_at`, derived `submission_elapsed_ms`, and the returned `contest_submission_sequence`. A database unique constraint on `(contest_id, contest_submission_sequence)` and a positive check enforce the contract. Sequence is server-assigned, stable, and the sole canonical order for contest attempts. Client timestamps, current Problem publication state, event delivery time, and judge completion time are never used for scoring order.

### 5.5 Announcements, clarifications, and safe render artifacts

Contest-authored and participant-authored content is revisioned and rendered through one closed pipeline:

```text
contest_announcements
  id UUIDv7 primary key
  contest_id UUID
  status draft|published|withdrawn
  audience public|participants|jury
  current_draft_revision_id UUID
  published_revision_id UUID nullable
  published_content_kind announcement_body nullable
  published_render_source_id UUID nullable
  published_render_source_kind typed enum nullable
  published_normalized_source_checksum string nullable
  published_render_artifact_id UUID nullable
  version bigint
  scheduled_for timestamptz nullable
  published_at, withdrawn_at timestamptz nullable

contest_clarifications
  id UUIDv7 primary key
  contest_id UUID
  requester_id UUID
  contest_problem_id UUID nullable
  status open|answered|closed|withdrawn
  answer_audience requester|participants
  question_revision_id UUID
  question_content_kind question
  question_render_source_id UUID
  question_render_source_kind typed enum
  question_normalized_source_checksum string
  question_render_artifact_id UUID
  current_public_question_revision_id UUID nullable
  current_public_question_content_kind public_question_summary nullable
  current_public_question_render_source_id UUID nullable
  current_public_question_render_source_kind typed enum nullable
  current_public_question_normalized_source_checksum string nullable
  current_public_question_render_artifact_id UUID nullable
  current_answer_revision_id UUID nullable
  current_answer_content_kind answer nullable
  current_answer_render_source_id UUID nullable
  current_answer_render_source_kind typed enum nullable
  current_answer_normalized_source_checksum string nullable
  current_answer_render_artifact_id UUID nullable
  version bigint
  created_at, answered_at, closed_at timestamptz nullable

contest_content_revisions
  id UUIDv7 primary key
  owner_type announcement|clarification
  owner_id UUID
  content_kind announcement_body|question|public_question_summary|answer
  revision_number bigint
  title string nullable
  markdown text
  author_id UUID
  created_at timestamptz
  render_source_kind typed enum
  normalized_source_checksum string
  render_source_id UUID unique
  unique (owner_type, owner_id, content_kind, revision_number)
  unique (id, content_kind, render_source_id, render_source_kind, normalized_source_checksum)

platform.content_render_sources
  id UUIDv7 primary key
  owner_revision_id UUID
  source_kind typed enum
  source_checksum string
  created_by UUID nullable
  created_at timestamptz
  unique (source_kind, owner_revision_id)
  unique (id, source_kind, source_checksum)
  unique (id, owner_revision_id, source_kind, source_checksum)

platform.content_render_artifacts
  id UUIDv7 primary key
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
  unique (render_source_id, renderer_version, sanitizer_policy_version)
  unique (id, render_source_id, render_source_kind, source_checksum)
  foreign key (render_source_id, render_source_kind, source_checksum)
    references content_render_sources (id, source_kind, source_checksum)
```

Contest owns its raw `contest_content_revisions`, lifecycle, audience, and exact revision/source/artifact pointers. It does **not** own another renderer table or pipeline: `api/internal/platform/contentrender` owns the shared immutable render-source and `content_render_artifacts` contracts used by problems, materials, announcements, and clarifications. The shared render-source parent makes the polymorphic relationship relationally enforceable instead of pretending a UUID plus owner string is a foreign key. Every raw Contest revision stores the typed source kind and checksum of the exact normalized Markdown bytes. Its composite foreign key `(render_source_id, id, render_source_kind, normalized_source_checksum)` binds its own revision ID to the shared `(id, owner_revision_id, source_kind, source_checksum)` tuple. Artifacts carry and composite-reference `(render_source_id, render_source_kind, source_checksum)`; a `ready` row has non-null sanitized HTML/checksum, while `rejected` has only bounded safe diagnostics and cannot be selected. Checked aggregate pointer-kind columns and composite foreign keys then bind revision-plus-source to the Contest revision and artifact-plus-the-same-source to the shared artifact; the source primary key transitively fixes its one owner revision. Thus the database, not only application validation, prevents an announcement body, raw question, approved summary, or answer from selecting an artifact from another kind/revision/checksum. The renderer enables the approved GFM, math, and code subset; raw HTML is disabled, URL schemes and embeds use a closed allowlist, external links receive safe attributes, and script/event/style injection is removed. Publish/submit/answer validates bounded source and rendered sizes. A renderer security upgrade creates a new shared artifact and an audited Contest command atomically changes the exact entity pointer; it never rewrites a content revision or old artifact. A pointer whose renderer policy is revoked is not served until a safe replacement has committed.

Announcement authors require `contest.announcement.manage` within their contest scope. Draft edits create content revisions. Publish validates contest state/audience and a matching `ready` shared render artifact, advances both published revision/artifact pointers, writes audit/outbox, and commits atomically. A scheduled publication is a durable command using the same idempotent handler. Withdrawal preserves all revisions and emits a tombstone event; it does not hard-delete browser history.

An active registered participant may submit a clarification while the contest is running; jury members with `contest.clarification.answer` may read all, answer, revise an answer, create/revise its shared question summary, change its audience, or close it. The requester may read their own raw item and withdraw it only while open. Other participants can read only answered items whose audience is `participants`. A participant-audience answer requires a separately authored, rendered, and explicitly approved `public_question_summary` revision plus the answer revision; the system never copies or automatically promotes the participant's raw question. Participant list/detail/replay projections return only that approved summary and answer and never return requester identity, the raw private question, or private operational metadata. Public anonymous users never receive participant clarifications. Jury-only announcement content is never placed in a public/participant replay topic.

Clarification questions are immutable after submission. Each answer, answer correction, or shared-summary correction creates a new raw revision and shared render artifact plus an audit entry. The answer command atomically advances the exact summary/answer revision-and-artifact pointers and status, then emits `contest.clarification.answered.v1`. Announcement and clarification events carry IDs, revision, audience, contest/problem references, and preview metadata derived only from the approved shared summary/answer artifact; they never derive preview text from the raw requester question. Clients fetch authorized canonical content through REST instead of receiving arbitrary Markdown or oversized HTML in the WebSocket envelope.

REST APIs provide cursor-paginated authorized announcement/clarification feeds, own clarification detail, jury inbox filters, and revision history for authorized operators. List and detail queries enforce audience/ownership in SQL and use separate public, participant, requester, and jury response mappers.

## 6. Scoring, standings, freeze, and rating

### 6.1 Normalized score state

The current JSONB-only problem result is replaced by normalized rows:

```text
contest_standings
  contest_id, user_id, solved_count, penalty_seconds, version, updated_at

contest_problem_results
  contest_id, user_id, contest_problem_id
  accepted, accepted_elapsed_seconds, wrong_attempts_before_accept
  best_score, last_scoring_submission_id, version

contest_scoring_attempts
  contest_id, user_id, contest_problem_id, submission_id
  submitted_at, submission_elapsed_ms, contest_submission_sequence
  active_generation, terminal_verdict
  judgement_version, source_event_id, updated_at
```

`contest_scoring_attempts` is the canonical ordered scoring input and is unique by submission. A newer rejudge generation/version supersedes that submission's prior verdict; duplicate or stale events are ignored. Unique keys and the consumer inbox prevent the same terminal event from applying twice.

### 6.2 ICPC scoring rule

For the initial ICPC mode:

- Every terminal non-accepted scoring submission whose canonical order precedes the earliest active AC increments `wrong_attempts_before_accept` once.
- The earliest active AC by `contest_submission_sequence` marks the problem solved once.
- `accepted_elapsed_seconds = floor(accepted_attempt.submission_elapsed_ms / 1000)`; elapsed time is never rounded to the nearest second.
- Problem penalty is `accepted_elapsed_seconds + wrong_attempts_before_accept * 1200` (20 minutes per counted wrong attempt).
- Verdicts after the first AC do not alter score.
- Compilation errors count or do not count according to an explicit contest setting snapshotted at publish; the default is to count them as wrong attempts.
- System errors never count against the contestant.
- Ranking is solved descending, penalty ascending, then last accepted time ascending, then stable user ID for deterministic display. Equal scored results receive competition rank.

For every event that advances a submission's active judgement version, the scorer locks the affected contestant/problem projection, upserts the canonical scoring-attempt row, and deterministically recomputes that problem result from all active attempts ordered by `contest_submission_sequence`. A cancelled or infrastructure-failed rejudge that did not become active is recorded operationally but cannot replace the scoring attempt. The scorer then recomputes the contestant aggregate, advances projection versions, writes the inbox marker and outbox event, and commits in one transaction. Correctness therefore does not depend on judge completion, event-delivery order, timestamp equality, or wall-clock adjustment; concurrent WA/AC events cannot double-increment solves, lose penalties, or choose a later AC. `submitted_at` remains authoritative for freeze visibility and elapsed-time display, not relative attempt order.

### 6.3 Freeze

Freeze is an independent published configuration with `freeze_at` and optional automatic `unfreeze_at`:

- Admin/jury queries always return official standings.
- Public standings exclude scoring attempts whose server-recorded submission time is at or after `freeze_at`; judge completion and event-delivery time do not determine visibility.
- Frozen rows show pending attempt indicators without revealing verdicts when the policy permits.
- At contest end, official standings continue to update privately.
- Unfreeze is an idempotent projection rebuild followed by `contest.standings.unfrozen.v1` and a latest-state standings event.

The public API explicitly reports `view: frozen|official`, projection version, and last cursor. It never silently mixes the two views.

### 6.4 Rating

Contests have a snapshotted `rated` flag and rating algorithm version. Rating runs only when:

- Contest is ended, not cancelled, and rated.
- Official standings have reached a final projection version.
- No unresolved judge system errors or active rejudge batch affects scoring.

Rating execution is a durable globally serialized job, not an unmanaged background goroutine. Contest owns the canonical rating decision history and current-rating projection; Cultivation mirrors committed current values for profile/ranking reads through the rating event.

```text
rating_state
  id text primary key                         # `global`
  next_sequence bigint
  applied_sequence bigint
  projection_revision bigint
  active_projection_generation_id UUID
  active_run_chain_id UUID

rating_sequence_slots
  rating_sequence bigint primary key
  contest_id UUID unique
  effective_end_at timestamptz
  assigned_at timestamptz
  unique (rating_sequence, contest_id)

rating_projection_generations
  id UUIDv7 primary key
  status building|verified|active|retired
  based_on_projection_revision bigint
  through_sequence bigint
  checksum string nullable
  created_at, verified_at, activated_at timestamptz nullable

rating_run_chains
  id UUIDv7 primary key
  status building|verified|active|retired
  based_on_chain_id UUID nullable
  through_sequence bigint
  checksum string nullable
  created_at, verified_at, activated_at timestamptz nullable

rating_run_chain_entries
  chain_id UUID
  rating_sequence bigint
  rating_run_id UUID
  primary key (chain_id, rating_sequence)
  unique (chain_id, rating_run_id)

contest_rating_runs
  id UUIDv7 primary key
  contest_id UUID
  rating_sequence bigint
  run_version bigint
  standings_projection_version bigint
  algorithm_version string                    # `expected-rank-v1`
  status waiting|running|applied|skipped|superseded|failed
  skip_code string nullable
  actor_id UUID nullable
  reason string nullable
  created_at, applied_at timestamptz nullable
  unique (contest_id, run_version)
  unique (rating_sequence, run_version)
  foreign key (rating_sequence, contest_id) references rating_sequence_slots

contest_rating_results
  rating_run_id UUID
  user_id UUID
  prior_rating integer
  prior_rated_contest_count integer
  actual_rank decimal
  expected_rank decimal
  delta integer
  new_rating integer
  primary key (rating_run_id, user_id)

contest_user_ratings
  projection_generation_id UUID
  user_id UUID
  rating integer
  rated_contest_count integer
  applied_sequence bigint
  revision bigint
  primary key (projection_generation_id, user_id)
```

When a rated contest enters `ended`, the command locks `rating_state['global'] FOR UPDATE`, assigns the next durable `rating_sequence`, and inserts its immutable sequence slot in the same transaction. Runs composite-reference that slot, so every version/skip at one sequence belongs to the same contest. Concurrent end commands are therefore given one persisted total order. Missed scheduled ends are reconciled in `(effective_end_at, contest_id)` order before allocating their sequences. A run may apply only when every smaller rating sequence has an active terminal chain entry (`applied` or `skipped`). An earlier contest waiting on final scoring blocks later ratings visibly.

Resolving, cancelling, or explicitly marking an already-sequenced contest unrated cannot delete its sequence or leave a hole. A separately authorized operator command records actor, non-empty reason, expected version, and a closed `skip_code`, creates an immutable terminal `skipped` run with no participant results/rating changes, appends it to the active run chain, and atomically advances `rating_state.applied_sequence/projection_revision`. It emits `contest.rating.skipped.v1`. Retry returns the same skip decision; changing it later requires the historical shadow-rerating flow from that sequence. Without this audited skip entry, later rating sequences remain blocked.

The normal apply transaction locks `rating_state`, verifies the expected global projection revision and standings version, resolves and locks the active projection generation/run chain, snapshots every participant's canonical prior rating row, writes one immutable result row per `(rating_run_id, user_id)`, updates only the active generation's `contest_user_ratings`, appends the run to the active chain, marks the run applied, advances the generation/chain/state sequence and projection revision, and emits `contest.rating_calculated.v1`. A user created after the active generation was built legitimately has no row; that absence is the canonical virtual prior `(rating = 1200, rated_contest_count = 0, applied_sequence = 0)` and the first rated run upserts the physical row. Absence is never interpreted as legacy zero, while a missing row for a user with authoritative rating history is an invariant failure. Public current-rating reads always resolve through `rating_state.active_projection_generation_id` with the same virtual-new-user rule; public canonical run/history reads resolve through `active_run_chain_id`, never a partially updated cached contest pointer. Retry of the same run returns its canonical result. Two contests can never snapshot the same stale global baseline and apply out of order.

#### 6.4.1 `expected-rank-v1`

Version `expected-rank-v1` intentionally changes a never-rated user's initial rating from the legacy zero default to `1200`. The rated participant set is every non-disqualified contestant with at least one acknowledged, eligible official contest submission persisted by the intake transaction, regardless of verdict or whether any problem was solved. A registered no-show has no such submission and is not rated. Let `N` be participant count, `R_i` the prior rating, and `C_i` the number of previously applied rated contests. If `N <= 1`, delta is zero.

Official results are ordered by the ICPC rule in section 6.2. Contestants with identical solved count and penalty share an actual rank equal to the arithmetic mean of their one-based occupied positions. For participant `i`:

```text
P(i beats j) = 1 / (1 + 10 ^ ((R_j - R_i) / 400))
E_i          = 1 + sum(j != i, 1 - P(i beats j))
K_i          = 40 when C_i < 5, otherwise 20
delta_i      = round_half_away_from_zero(K_i * (E_i - actual_rank_i) / (N - 1))
new_rating_i = max(0, R_i + delta_i)
```

For `N <= 2000`, v1 evaluates every pair exactly using the pinned Go `float64` reference implementation. For `N > 2000`, it uses the existing deterministic approximation: group nonnegative ratings into width-100 buckets by `floor(rating / 100)`, visit bucket IDs ascending, use integer `floor(sum_rating / count)` as each representative, compute `E_i = 1 + sum(bucket_count * P(bucket beats i)) - 0.5`, and apply the same K, rounding, and floor. Algorithm constants, the exact/approximation threshold, bucket width, arithmetic order, and rounding belong to the version; changing any one requires a new algorithm version and rerating run.

Mandatory golden vectors are:

| Prior ratings / results | Prior counts | Expected new ratings |
|---|---|---|
| one participant at `1200` | `0` | `1200` |
| `1000` beats `1500` | `0, 0` | `1038, 1462` |
| three `1200` users finish first/second/third | `0, 0, 0` | `1220, 1200, 1180` |
| two tied `1200` users with identical result | `5, 5` | `1200, 1200` |
| two `0` users, first/second | `5, 5` | `10, 0` after floor |
| 2001 users at `1200`, uniquely placed first through last | all `0` | first `1220`, position 1001 `1200`, last `1180` |

CI stores the complete input/output golden artifact for the 2000/2001 threshold and fails on any changed result under the same algorithm version.

#### 6.4.2 Rerating and migration

An admin rerating command creates a new audited run and never rewrites historical rows. If any later rating sequence was applied from the old baseline, an isolated historical rerate is forbidden. The globally serialized rating worker creates a durable **shadow** projection generation and shadow run chain, reconstructs the opening state through `target_sequence - 1`, then recomputes the target and every later active sequence into new immutable runs/results. Checkpoints make restart idempotent; no shadow row is reachable from a public pointer while status is `building`.

After the shadow generation and chain both reach `rating_state.applied_sequence`, verification checks the sequence has no gap, every run/result cardinality, the final per-user projection, and independently computed deterministic checksums. One short transaction then locks `rating_state`, proves its based-on revision and active pointers have not changed, marks both shadows verified/active, atomically switches `active_projection_generation_id` and `active_run_chain_id`, increments `projection_revision`, and records the audited activation/outbox event. Only after that switch may the old generation/chain be marked retired. Public reads therefore see the entire old complete view or the entire rebuilt complete view, never a suffix of new ratings paired with old run history. Cancel/failure leaves the old pointers untouched and cleanup may delete only unreachable shadow data.

Migration initializes users with zero previously rated contests to `1200`, even when the legacy stats row contains its default `0`. Users with rating history retain their current canonical rating only after an audit proves it matches the last applied legacy history; mismatches are quarantined for reconciliation. The migration creates the sequence-zero opening projection generation and run chain, records and independently verifies their checksums, then atomically installs both active pointers before rating jobs are enabled. This is an explicit `expected-rank-v1` behavior change, not accidental preservation of the legacy missing-user default.

## 7. Persisted notifications

### 7.1 Data model

```text
notifications
  id UUIDv7
  user_id UUID
  type string
  content_kind catalog|campaign
  title_key string nullable
  body_key string nullable
  params jsonb nullable
  campaign_revision_id UUID nullable
  action_path string nullable
  source_event_id UUID
  created_at timestamptz
  read_at timestamptz nullable
  archived_at timestamptz nullable
  expires_at timestamptz nullable
  unique (user_id, source_event_id, type)
  check (exactly one of catalog keys or campaign_revision_id is selected)

notification_preferences
  user_id UUID
  category string
  in_app bool
  primary key (user_id, category)
```

The launch schema models only the supported in-app channel. Email, push, and quiet-hours fields are not pre-created before those delivery channels exist; they require a later versioned channel design. Stored system-notification content uses stable message keys plus typed parameters so the frontend controls Vietnamese copy and later language additions do not require rewriting historical rows. Security notifications cannot be disabled in-app. User-authored contest announcements remain sanitized domain content, not UI catalog literals.

`action_path` is an internal relative route produced by a closed server-side route builder. Absolute URLs, schemes, protocol-relative paths, and arbitrary redirects are rejected.

### 7.2 Admin in-app notification campaigns

The launch admin composer is backed by a real durable in-app campaign model; it is not a frontend-only send button:

```text
notification_campaigns
  id UUIDv7 primary key
  status draft|scheduled|sending|completed|failed|cancelled
  current_draft_revision_id UUID
  scheduled_revision_id UUID nullable
  scheduled_render_source_id UUID nullable
  scheduled_render_source_kind typed enum nullable
  scheduled_normalized_source_checksum string nullable
  scheduled_render_artifact_id UUID nullable
  operation_id UUID nullable
  version bigint
  scheduled_for timestamptz nullable
  last_failure_code string nullable
  created_by UUID
  scheduled_by, cancelled_by UUID nullable
  schedule_reason, cancel_reason string nullable
  created_at, scheduled_at, started_at, completed_at, cancelled_at timestamptz nullable

notification_campaign_revisions
  id UUIDv7 primary key
  campaign_id UUID
  revision_number bigint
  title string
  body_markdown text
  body_render_source_id UUID
  body_render_source_kind typed enum
  body_render_artifact_id UUID
  action_path string nullable
  audience_kind explicit_users|role|all_active
  audience_role_id UUID nullable
  normalized_source_checksum string
  author_id UUID
  created_at timestamptz
  unique (campaign_id, revision_number)
  unique (id, body_render_source_id, body_render_source_kind, normalized_source_checksum)

notification_campaign_explicit_users
  campaign_revision_id UUID
  user_id UUID
  primary key (campaign_revision_id, user_id)

notification_campaign_confirmations
  nonce_hash bytes primary key
  actor_id UUID
  session_id UUID
  campaign_id UUID
  campaign_revision_id UUID
  render_source_id UUID
  render_source_kind typed enum
  normalized_source_checksum string
  render_artifact_id UUID
  audience_fingerprint bytes
  expected_campaign_version bigint
  dry_run_estimated_at timestamptz
  dry_run_recipient_count bigint
  issued_at, expires_at timestamptz
  consumed_at timestamptz nullable
  consumed_by_idempotency_key UUID nullable
  schedule_request_hash bytes nullable
  operation_id UUID nullable

notification_campaign_recipients
  campaign_id UUID
  user_id UUID
  campaign_revision_id UUID
  status pending|created|skipped|failed
  notification_id UUID nullable
  skip_code string nullable
  failure_code string nullable
  attempt_count integer
  next_attempt_at timestamptz nullable
  created_at, last_attempt_at, processed_at timestamptz nullable
  primary key (campaign_id, user_id)
```

Campaign revisions are immutable and use the same shared `platform/contentrender` source/artifact contract. Each raw campaign revision composite-references `(body_render_source_id, id, body_render_source_kind, normalized_source_checksum)` so its own ID is the source owner revision; its artifact composite-references `(body_render_source_id, body_render_source_kind, normalized_source_checksum)`. The campaign's scheduled revision/source/artifact pointers are also composite-bound, so scheduling cannot swap in an artifact from another revision or checksum. The title is bounded plain text and the body is sanitized authored domain content; neither is a frontend catalog key. `action_path` uses the same closed internal route builder as system notifications. The audience is a closed selector: an explicit bounded/imported user set, one authorized role, or all active accounts. Arbitrary SQL/filter JSON is forbidden.

The canonical capability registry is:

- `notification.compose`: create/edit/preview/dry-run and schedule to an explicit bounded user set;
- `notification.broadcast_role`: additionally select one authorized role;
- `notification.broadcast_all_active`: additionally select all active accounts;
- `notification.read_delivery`: inspect recipient/delivery details and bounded failure samples.

There is no launch `notification.campaign.manage` alias; any temporary alias is removed before the final authorization seed/migration. Role/all-active scheduling requires the selector-specific capability, a server-verified recent reauthentication, and a short-lived single-use confirmation nonce. Only the nonce's hash is durable; the raw opaque value exists only in client memory. Its row binds actor/session, campaign, exact preview revision/source/kind/checksum/artifact, a canonical audience fingerprint, expected campaign version, dry-run timestamp/count, and reviewed schedule time. The schedule transaction locks and consumes the unexpired row atomically with campaign freezing, recipient snapshot creation, audit, and operation creation. It records the schedule idempotency UUID, canonical request hash, and resulting operation ID on that consumed row. An exact retry after commit/lost response reauthorizes the caller, matches that full tuple/hash, and returns the same campaign/operation without consuming again; a different key/hash or any changed input, session, version, recipient count, expired reauthentication, expiry, or consumed hash fails closed and requires a new dry-run/confirmation. Logs and audit never contain the raw nonce.

Preview renders the exact draft revision. Dry-run returns an authorization-filtered count and bounded sample with an `estimated_at` marker, but does not promise delivery. `api/internal/application/workflow/notificationcampaign` owns the immediate-consistency schedule transaction; Notification never imports Identity repositories or Ent entities. Through the shared `platform/transaction.UnitOfWork`, Notification locks the campaign/confirmation, Identity's narrow `SnapshotCampaignAudience` participant resolves the exact closed selector and active membership into a bounded UUID set, and Notification inserts the unique recipient rows, freezes content/audience revision, writes audit, and starts the durable `platform/operation`. A hard configured `max_campaign_recipients` bounds the transaction and any selector above it is rejected before mutation. The count must still equal the confirmed dry-run count; otherwise scheduling rolls back and requires reconfirmation. Role/all-active membership is therefore the committed schedule snapshot; later additions are not silently appended.

Delivery runs in bounded top-level `notificationcampaign` UnitOfWork batches. Identity's narrow transaction-bound participant locks/rechecks each selected account's active status and, for a role campaign, current role membership immediately before Notification creates its row. A suspension/removal may therefore linearize before delivery and produce a bounded `skip_code`; Notification never reads Identity tables directly. For each eligible recipient, the `notifications` row, recipient `created` transition, and `notification.created.v1` outbox event commit together. The unique `(campaign_id, user_id)` key, operation checkpoint, and state machine make retry/resume idempotent.

A retryable provider/database boundary leaves the recipient `pending`, increments its bounded attempt metadata, and schedules backoff; it does not count as terminal failure. A closed nonretryable error or exhausted recipient retry budget transitions that recipient to `failed` with a low-cardinality `failure_code`. If the operation cannot continue or any terminal failures remain, the campaign becomes `failed` rather than falsely `completed`. An explicitly authorized, expected-version, reasoned resume command can move eligible failed recipients back to `pending` without changing their unique identity or duplicating already-created notifications; exhausted/nonretryable invariants that are not safely retryable remain failed. `completed` means no pending or failed recipient remains. Cancel is idempotent, prevents new batches, allows the current bounded transaction to finish, and preserves already-created notifications and audit history. Status/count queries read durable operation progress. V1 sends no email or push, and cancellation never pretends to recall a notification already committed.

### 7.3 Initial categories and events

- Submission terminal verdict and system-error recovery.
- Contest registration, start reminder, lifecycle change, announcement, clarification answer, final rank, and rating change.
- Role/account/session/security changes.
- Admin operational alerts for dead judge jobs, outbox backlog, migration mismatch, and sandbox capacity.
- Material publication events only for users who explicitly follow that category after launch.

Consumers create notifications from committed domain events using inbox idempotency. Inserting the notification and its `notification.created.v1` outbox event is one transaction.

### 7.4 API and realtime behavior

User REST endpoints provide cursor pagination, unread count, detail-safe action links, `mark-read`, `mark-all-read`, archive, and preferences. Ownership is enforced in the repository query, not only in handlers. Admin endpoints provide campaign draft/revision CRUD, preview, dry-run, schedule, status, idempotent resume/cancel, and bounded recipient failure summaries. Every mutation enforces its canonical capability, expected version, idempotency key, actor, reason, and shared audit in its application command; recipient inspection additionally requires `notification.read_delivery` and redacts fields outside that permission's scope.

WebSocket sends `notification.created.v1`, `notification.read.v1`, and an unread-count latest-state event on `notifications:{userID}`. If the socket is absent or slow, the notification remains in PostgreSQL. Reconnect reads the REST inbox and converges from its cursor.

## 8. Event catalog

Initial stable event names are:

```text
submission.queued.v1
submission.state_changed.v1
submission.judged.v1
submission.system_error.v1
problem.solve_status_changed.v1
contest.published.v1
contest.started.v1
contest.ended.v1
contest.cancelled.v1
contest.registration.changed.v1
contest.standings.changed.v1
contest.standings.unfrozen.v1
contest.rating_calculated.v1
contest.rating.skipped.v1
contest.rating_rebuilt.v1
contest.announcement.published.v1
contest.announcement.withdrawn.v1
contest.clarification.submitted.v1
contest.clarification.answered.v1
contest.clarification.closed.v1
contest.clarification.withdrawn.v1
notification.created.v1
notification.read.v1
notification.campaign.status_changed.v1
authorization.changed.v1
account.status_changed.v1
```

Event schemas are additive within a version. Removing or changing meaning requires a new event version. Internal payloads can contain more fields than realtime projections, but the projection is explicitly constructed and security-reviewed.

## 9. Migration sequence

1. Fix UUID, contest lifecycle, and submission ownership defects while the existing delivery paths remain active; capture characterization fixtures before changing semantics.
2. Add the shared transaction/audit/content-render prerequisites, Contest registration/content tables, exact revision/source/artifact pointer constraints, and compatibility reads. Backfill and quarantine unsafe authored HTML before exposing new reads.
3. Add contest submission sequence/counter fields and normalized attempt/result/standings rows. Backfill deterministic sequences, enforce the per-contest unique constraint only after audit, then dual-read and compare old/new scoring projections.
4. Add rating state, immutable runs/results, projection generations/run chains, and the audited `1200` opening migration. Quarantine history mismatches, verify golden/checksum artifacts, and do not enable rating workers until active pointers are valid.
5. Add notification/preferences/campaign/revision/explicit-audience/hashed-confirmation/recipient tables plus durable operation checkpoints. Keep email/push absent and keep the composer disabled until its permission, audit, preview, dry-run, atomic audience snapshot, failure, resume, and cancel commands are live.
6. Add outbox/general inbox plus `realtime_cursor_state`, `realtime_events`, and `realtime_projection_inbox`. Initialize the global committed/minimum-retained cursor state and deploy the serialized projector/tailer dark with cursor-contiguity checks.
7. Add `go-common/pkg/common/websocket` with contract, race, origin, quota, byte-budget, inbound-overload, and cancellation/undrained-handler tests.
8. Start writing outbox events in submission, contest, authorization, notification, and campaign transactions; compare idempotent consumer results while SSE/polling remain authoritative.
9. Deploy the dispatcher, replay writer/tailer, post-commit authorization socket effects, and WebSocket endpoint without removing SSE/polling. Kill/restart and backlog drills must pass before traffic ramps.
10. Add the single frontend WebSocket provider, REST snapshot/cursor handoff, theme-safe status UI, and observable fallback; then move submission verdicts, standings, notifications, campaigns, and admin telemetry by feature flag.
11. Remove contest SSE and fixed-interval submission polling only after production metrics show replay convergence and no active legacy clients for the agreed window.
12. Remove compatibility tables/fields and old active-rating/scoring reads only in a later contract migration after checksum comparisons remain clean.

## 10. Verification and acceptance

The design is accepted only when automated tests demonstrate:

- Autobahn-compatible core behavior inherited from the pinned library plus package-level protocol tests.
- Race-clean hub join/leave/broadcast/shutdown under concurrent churn.
- Exact origin, token-purpose, session, and topic ACL enforcement; absent credential deliberately yields anonymous, any present-invalid credential rejects, anonymous defaults deny outside the three public topic forms, and outbound Hub access exists only in trusted service adapters.
- Anonymous global/trusted-remote-IP connection and upgrade-rate quotas resist spoofed forwarding headers; public/participant/jury projections cannot cross audience rooms or replay queries.
- Committed suspension/session/role/policy/visibility changes fence stale delivery and idempotently mark/close every selected existing connection before routing resumes, including crash recovery through outbox replay and anonymous public-topic selectors.
- Mailbox item/byte and global logical-byte accounting remain exact under fan-out/coalesce/drop/dequeue/close; inbound rate/handler limits bound CPU/goroutines, and slow clients are coalesced, dropped, or disconnected by class.
- Zero-subscriber broadcast is a success, a missing targeted destination is typed not-found, and one destination's pressure outcome does not become a global partial-failure error.
- A DB commit followed immediately by process kill still yields replayable events after restart.
- Bounded replay batches allocate one contiguous `cursor_offset` range under one committed-watermark lock; a waiter rechecks its projection inbox after acquiring that lock, concurrent duplicate writers cannot create a uniqueness/allocation race or expose commit inversion, and no event is broadcast before its transaction commits.
- Duplicate outbox delivery, including after replay-row retention expiry, reuses its projection inbox offset and does not duplicate replay, standings, rating, notifications, or campaign recipients.
- One source outbox ID can create only one canonical topic projection; multi-topic cases use separately identified causal outbox events and remain deterministic under retry.
- A lost `NOTIFY`, process restart, and an event committed during snapshot/subscribe handoff are repaired by polling/fencing without a replay gap; an expired cursor returns `resync_required`.
- REST snapshot race tests prove the domain projection and committed cursor come from one repeatable-read snapshot: state-first/cursor-later interleavings cannot return stale state with a cursor already past its update.
- Concurrent contenders for the final registration slot admit exactly one participant. Open-mode implicit registration, sequence reservation, submission/spec/job/outbox creation, and rollback are atomic with no over-capacity or orphaned reservation.
- Contest end/cancel versus submission admission serializes through the contest lock; an admitted submission carries the exact contest problem/testset/checker/runtime snapshot and a unique server sequence.
- Intake lock-order tests prove Identity eligibility is protected first and Contest never reads Account state. A deliberately blocked contest lock proves the single `clock_timestamp()` is captured only after lock acquisition.
- Boundary goldens accept exact `start_at`, reject exact `end_at`, hide submissions at exact `freeze_at`, produce elapsed second `0` at `999ms` and `1` at `1000ms`, and add exactly `1200` seconds per counted wrong attempt.
- Concurrent WA/AC verdicts produce one solve and exact ICPC penalty.
- AC delivery before an earlier-submitted WA, equal-timestamp submissions, duplicates, and rejudge reversals all produce the same deterministic score as canonical submission order.
- Freeze exposes no post-freeze verdict while official standings continue updating.
- Rated-participant tests include WA/CE/other eligible official attempts regardless of verdict, exclude registered no-shows/disqualified users, and verify initial rating `1200`, K thresholds, floor zero, tie rounding, and complete 2000/2001 golden vectors.
- Concurrent contest ends allocate/apply one global rating order. Historical rerating builds a restartable shadow projection/run chain, checksum-verifies it, and makes public readers observe only the old or new complete pointer pair at activation.
- Marking a blocking contest unrated/cancelled creates one audited idempotent `skipped` chain entry with no rating delta, advances the sequence exactly once, and never lets a later run cross an unrecorded gap.
- Migration tests initialize zero-history users to `1200`, preserve only audited consistent legacy histories, and quarantine mismatches before rating enablement.
- Raw clarification questions/requester identity never appear in participant SQL mappers, REST, previews, logs, or replay. Participant publication requires an explicit jury-authored safe summary, and XSS/unsafe-link/render-policy-revocation plus cross-revision artifact-pointer tests fail closed.
- Campaign capability separation (`compose`, role/all-active broadcast, delivery inspection), closed selectors, recent-reauth/single-use hashed confirmation binding/atomic consumption, exact schedule-idempotency recovery after commit/lost response, different-key/hash replay rejection, dry-run count drift, expected-version/revision replay rejection, exact immutable source/artifact, hard-bounded atomic audience snapshot, eligibility recheck, transient-pending versus terminal-failed semantics, bounded recipient batches, resume/cancel races, `(campaign_id,user_id)` idempotency, safe action paths, audit actor/reason, nonce redaction, and absence of email/push side effects are covered.
- The final production backend and frontend contain no SSE route, `EventSource`, legacy SSE hook, or fixed-interval submission-status polling.
- A 1,000-socket authenticated/anonymous mix plus the agreed HTTP workload meets the production SLOs defined in the operations design without allowing anonymous traffic to starve authenticated control paths.
