# Judge queue and worker capacity

## Symptoms and dashboard

Use `ops/observability/dashboard-judge.json`. Relevant signals are oldest-ready
age, state depth, claims-enabled/free capacity, lease retries, finalization
errors, disk safety, and worker readiness check names.

## Safe containment

1. Keep the API serving unrelated reads.
2. Let submission admission fail retryably when queue/disk/capability caps are
   reached; do not raise caps without load evidence.
3. Disable claims for the affected exact worker boot before restarting it.
4. Never manually mark a job completed or copy capability evidence between
   boots.

```sh
scripts/deploy/status.sh
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T worker wget --quiet --output-document=- http://127.0.0.1:9091/live
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T worker wget --quiet --output-document=- http://127.0.0.1:9091/ready
```

Expected worker readiness is 200 only when the current boot, claims gate,
exact capability evidence, rootless sandbox daemon, cgroup v2, database, and
disk checks pass.

## Escalation and recovery

Use the sandbox runbook for probe/cgroup failures and disk runbook for the
safety watermark. Roll back a worker only after its previous exact tuple
passes smoke against the current schema. Recovery requires queue age to trend
down, one terminal result per generation, no expired lease left unreconciled,
and reward/rating/notification projections converged. Record worker/boot IDs
only in the restricted incident record, not metric labels.
