# Performance test templates

These are deliberately **not yet Judgify API tests**. They model concurrency, request pressure, test safety, thresholds, and report conventions without embedding credentials, endpoint assumptions, or application behavior.

## Profiles

- `smoke`: one virtual user for 15 seconds; validates a chosen safe endpoint.
- `normal-1k-ccu`: 1,000 virtual users over 40 minutes, browsing with a ten-second think time by default.
- `contest-final-spike`: 1,000 readers plus a 50 arrivals/s spike template. Until submission auth and payload contracts are attached, its spike safely calls the configured readiness path rather than submitting code.
- `soak`: 200 users for four hours to reveal leak and degradation signals.

## Run

```sh
TARGET_URL=http://127.0.0.1:8080 \
SMOKE_PATH=/healthz \
make -C ops perf-smoke
```

Enable k6's local live dashboard and persist a portable HTML report:

```sh
TARGET_URL=https://staging.example.invalid \
K6_WEB_DASHBOARD=true \
K6_WEB_DASHBOARD_EXPORT=ops/performance/results/smoke.html \
k6 run ops/performance/k6/scenarios/smoke.js
```

The `results/` directory is ignored. CI should upload HTML/JSON output as an artifact; approved summaries belong in `baselines/`.

## Later integration contract

The future submission scenario must add dedicated custom metrics for durable acknowledgement, queue wait, verdict completion, and judge failures. It must use a non-production test account and seed data, and it must never log source code, hidden test cases, credentials, or tokens.

