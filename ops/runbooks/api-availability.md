# API availability and latency

## Symptoms and dashboard

Use `ops/observability/dashboard-api.json`. Trigger conditions are elevated
`5xx`, p95 latency over 500 ms, failed readiness checks, or a rapid increase in
goroutines/memory/file descriptors. Preserve the alert start time and release
ID before changing anything.

## Safe containment

1. If `/live` fails, keep the instance out of ingress and restart only that
   process.
2. If `/ready` fails, inspect the named safe health code; do not bypass the
   readiness gate.
3. If one route template is responsible, rate-limit that route at ingress.
4. Do not restart PostgreSQL or the worker merely because API latency rose.

```sh
scripts/deploy/status.sh
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/live
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/ready
```

Expected output is HTTP 200. A 503 readiness response identifies a bounded
check name/code without a raw dependency error.

## Escalation and recovery

Rollback the API binary when the regression began with the current release and
the prior binary is schema compatible. Escalate to the database runbook for a
database check failure; use the authorization or outbox runbook for those
specific checks. Recovery requires normal readiness for ten minutes, error
rate inside budget, stable resource curves, and one authenticated product
smoke. Record CID examples, affected route templates, release, containment,
and verification in the incident log.
