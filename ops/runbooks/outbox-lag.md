# Durable outbox lag or dead events

## Symptoms

The product dashboard shows pending/dead counts, oldest pending age, and replay
lag. API readiness may fail only when the critical dispatcher itself cannot
make progress; a bounded optional projection can report degraded.

## Containment and diagnosis

Keep PostgreSQL authoritative. Do not delete, mutate payloads, or publish
directly to WebSockets. Pause the affected dispatcher through its supported
operation when repeated attempts amplify load. Correlate application logs by
the event's origin CID; keep event IDs out of metric labels.

```sh
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/ready
scripts/deploy/status.sh
```

Inspect dead/replay state through the admin operations UI or supported
operator command. Expected output identifies a stable error code and handler,
not raw payload data.

## Solve-reward Forge queue

The solve-reward projection consumes a local Forge queue whose backlog is
independent of the PostgreSQL outbox. `judgify_solve_reward_forge_consumer_lag`
counts records the consumer has not acknowledged; `judgify_solve_reward_forge_dlq_records`
counts records the consumer rejected. A non-zero DLQ means reward progression is
silently behind for the affected users, so treat it as data loss until replayed.
`judgify_solve_reward_forge_segments` and `judgify_solve_reward_forge_storage_bytes`
show whether the queue is approaching its retention or storage ceiling; a rising
segment count with flat lag means retention is not reclaiming space.

Diagnose by comparing the reward reconciler against the ledger before replaying;
the queue is an accelerator, PostgreSQL stays authoritative.

## Recovery

Fix or roll back the deterministic handler, then use the fenced replay
operation. Unknown event types remain dead until a reviewed route exists.
Recovery requires pending age below 60 seconds, no unexpected dead events, and
reconciler equality for reward, material-view, notification, contest, and
rating projections. Record replay actor, reason, CID, event count, release,
and post-reconcile evidence.
