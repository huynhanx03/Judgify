# Release rollout

## Preconditions

Use a reviewed immutable release directory with `release.env`, signed
artifacts/provenance, migration compatibility, exact judge manifest, and a
fresh verified off-host backup. The host deployment driver must implement the
command contract in `scripts/deploy/release.sh`. Keep the driver, state root,
release directory, and manifests owned by root or the deployment operator and
never writable by group/other users; symlinks are refused.

```sh
export JUDGIFY_DEPLOY_ACK=DEPLOY_JUDGIFY_RELEASE
export JUDGIFY_DEPLOY_DRIVER=/etc/judgify/deploy-driver
export JUDGIFY_DEPLOY_STATE_ROOT=/var/lib/judgify/deploy
scripts/deploy/release.sh /var/lib/judgify/releases/RELEASE_ID
```

The state machine is idempotent and records each completed transition. Every
step marker binds both the release-manifest and deployment-driver SHA-256, so a
resume cannot silently use mutated control code. It
applies expand-compatible migrations first, proves the new worker claims-off,
enables/drains workers, starts the API on a candidate listener, checks
readiness and exact capability activation, atomically switches ingress, then
drains the old API. The host driver must query the loopback-only role listeners
by executing the probe inside the candidate container/process network
namespace (`127.0.0.1:9090` and `127.0.0.1:9091`). A PID check, static `/ping`,
product listener, or public ingress response is not readiness.
When a current release exists, `previous_release_id` must equal it exactly.
The rollout records the release-manifest and driver checksums on its first
attempt and refuses to resume if either changes.
An immutable activation receipt also binds a release ID to that successful
activation. Never reuse a release ID after rollback; publish a new immutable
release ID even when the artifact content is unchanged.

## Stop and escalation conditions

Stop on missing signature/SBOM/config/backup evidence, incompatible schema,
stale worker heartbeat, missing exact tuple smoke, readiness failure, or
product smoke failure. `/metrics`, `/live`, `/ready`, and `/ping` remain
unreachable through the public `/api/` ingress path. Never skip a marker,
manually mark a step done, or run contract/destructive migrations during this
rollout.

## Completion

Retain rollout log, release state, smoke evidence, migration head, backup
receipt, tuple evidence, actor, and timestamps. Observe SLOs for one error
budget window before deleting N-1 artifacts.
