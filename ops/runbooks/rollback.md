# Binary rollback

## Decision

Rollback when the current binary regresses and the retained previous release
declares compatibility with the already-applied schema. Database rollback is
forbidden. If the old binary cannot read the current expanded schema, contain
the incident and fix forward.

```sh
export JUDGIFY_ROLLBACK_ACK=ROLL_BACK_JUDGIFY_BINARY_ONLY
export JUDGIFY_DEPLOY_DRIVER=/etc/judgify/deploy-driver
export JUDGIFY_DEPLOY_STATE_ROOT=/var/lib/judgify/deploy
scripts/deploy/rollback.sh /var/lib/judgify/releases/PREVIOUS_RELEASE_ID
```

The script verifies artifacts/provenance/schema, proves the prior worker exact
tuples claims-off, drains the current worker, readies the prior API on a
candidate listener, switches ingress atomically, and drains the current API.
Worker and API gates execute against loopback `9091` and `9090` from inside the
candidate process namespace; process existence or the compatibility `/ping`
route is not rollback evidence. Only the exact `previous_release_id` (N-1) in
current state is accepted. Rollback state and every step marker bind the target
manifest and deployment-driver checksums and refuse mutation on retry.
The activation receipt remains immutable after rollback, so the rolled-back
release ID cannot later be reused for a fresh rollout. Issue a new release ID.

## Validation and evidence

Run `scripts/deploy/smoke.sh`, then verify readiness, API error/latency, queue
age/finalization, outbox lag, authorization revision, WebSocket reconnect, and
security email age. Escalate rather than restoring a database backup solely to
undo an application release. Record decision owner, current/target release,
schema head, exact tuple evidence, reason, state-machine log, and recovery time.
