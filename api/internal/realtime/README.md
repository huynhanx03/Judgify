# Realtime integration contract

Realtime is a live-update channel, not a second source of truth. Clients fetch
the REST snapshot before subscribing and fetch it again after reconnecting.
Every browser-safe event is projected into PostgreSQL under one serialized,
opaque global cursor before Hub fan-out. A reconnect issues bounded per-topic
replay requests; an expired/unknown cursor receives `resync_required` and the
client reconciles through REST. No event is recovered from process memory.

## HTTP and WebSocket boundaries

- `POST /api/v1/realtime/tickets` is protected by the live-session access-token
  middleware. It calls `TicketStore.IssueFromContext`, returns
  `{ "ticket": "...", "expires_at": "..." }`, and sets
  `Cache-Control: no-store`. The endpoint must not automatically retry because
  each successful call creates a different short-lived credential.
- `GET /ws` is anonymous and can subscribe only to public contest topics.
- `GET /ws?ticket=<opaque>` atomically consumes a single-use ticket. Query
  strings must be redacted from HTTP access logs. Refresh cookies and bearer
  credentials are never accepted by the WebSocket resolver.
- Construct the handler with a dependency-backed `TopicAuthorizer`. Exact
  origin checks, connection limits, ping/pong, backpressure, and graceful drain
  remain owned by go-common's WebSocket hub.

## Topics and events

- Public: `contest:<contest UUID>:standings`, event
  `contest.standings.changed.v1`. This is a coalescible invalidation; the client
  refetches the standings REST snapshot.
- Public discovery: `contest:lifecycle`. Public lifecycle changes are also sent
  to `contest:<contest UUID>:public` so detail views can invalidate without a
  global subscription.
- Active participant: `contest:<contest UUID>:participants`. Subscription
  checks the canonical registration projection on every subscribe.
- Requester-private: `contest:<contest UUID>:user:<own user UUID>`. Only the
  exact ticket principal can subscribe.
- Staff: `contest:<contest UUID>:staff`. Subscription requires the current
  Casbin `contest:read` capability.
- Contest content and lifecycle messages contain only
  `{contest_id, resource, resource_id, status, version, changed_at}`. They are
  latest-state invalidations; authored questions, answers, Markdown, requester
  identity, and render artifacts never enter a browser frame.
- Private: `user:<own user UUID>:submissions`, event
  `submission.judged.v1`. It carries immutable routing/verdict facts from the
  committed Submission outbox event; the browser reloads the canonical REST
  submission snapshot for mutable presentation fields. Delivery is critical
  and never coalesces or silently drops. A client whose bounded mailbox cannot
  accept it is closed with `1013`; reconnect always refetches the authoritative
  REST cursor page. No open-ended topic grammar is accepted.
- Authenticated: `authorization:capabilities`, event
  `authorization.capabilities.changed.v1`. The event contains only the latest
  committed authorization revision. Every client refetches its own
  `/authorization/me` projection and ignores revisions it already holds.

Events contain summary fields only. Source code, testcase data, diagnostics,
judge leases, and reward internals are never selected from the database.

## Composition

Create `db.NewAuthorizationSource(entClient)`, the dependency-backed topic
authorizer, and one `realtime.NewHubPublisher(hub, replayProjection)`. Submission, contest,
notification, and campaign browser events are delivered only from the
transaction-bound PostgreSQL outbox by the API-owned durable dispatcher;
production never table-polls submissions. The authorization revision tailer is
the fail-safe socket-revocation fence for a committed policy change. The API
process never opens the worker's local queue. A failed publish never marks an
outbox row delivered.

Register lifecycle components in this order so reverse shutdown is safe:

1. ticket store
2. WebSocket hub
3. authorization revision tailer
4. durable event dispatcher
5. HTTP server

Shutdown then stops new HTTP upgrades, stops producing changes, drains the hub,
and finally destroys all outstanding tickets.
