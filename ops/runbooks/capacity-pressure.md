# Process and host capacity pressure

## Symptoms

Memory, goroutine, file-descriptor, database-pool, request latency, or queue
metrics grow outside the measured soak envelope.

## Containment

Rate-limit the responsible route template or operation kind, pause optional
bulk work, and preserve capacity for authentication, submission
acknowledgement, health, and drain. Do not increase concurrency, connection
pools, or OS limits before identifying the bounded resource owner.

```sh
scripts/deploy/status.sh
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/metrics
```

Metrics must be read from the private listener. Compare against the retained
four-hour soak evidence for the same release and host shape.

## Recovery

Roll back regressions or lower code-owned concurrency to the last validated
setting. Recovery requires thirty minutes of flat memory/goroutine/FD curves,
database waiters near zero, error and latency budgets restored, queue age
falling, and a product smoke. Escalate if the process approaches OOM or FD
exhaustion; preserve profiles only through the audited time-bounded debug
control. Record release, thresholds, samples, containment, and CID examples.
