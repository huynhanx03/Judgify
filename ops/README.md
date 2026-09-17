# Judgify operations

This directory is intentionally independent of `api/`, `website/`, CI, and the runtime compose files. It is a safe first step for operating and benchmarking Judgify without changing application behavior.

## Included now

- `stack/`: a local Grafana, Prometheus, Loki, Tempo, and Grafana Alloy stack. Prometheus only scrapes itself; Alloy has no application source configured.
- `performance/`: k6 workload templates. Every scenario requires an explicit `TARGET_URL`; none can accidentally call a Judgify environment by default.
- `observability/`: existing SLOs, alert specifications, metric contract, and product dashboard specifications. They remain design contracts until the application exports the corresponding telemetry.
- `runbooks/` and `backup/`: existing operational documentation.

## Operating contracts

Before connecting an application, use these files as the source of truth:

- `contracts/performance-strategy.md`: what a valid capacity claim means.
- `contracts/workload-models/`: the exact behavior behind “1,000 CCU”.
- `contracts/metric-catalog.md`: metric names, units, and label rules.
- `contracts/alert-policy.md`: what pages an operator versus what becomes a ticket.
- `performance/report-template.md`: evidence required for every baseline.

The runnable Prometheus rules and Grafana service dashboard live under
`provisioning/`. They intentionally show no Judgify data until a later,
explicit instrumentation change exports the metric catalog.

## Local stack

```sh
make -C ops up
make -C ops status
make -C ops validate
```

Grafana is available at http://localhost:3300 (`admin` / `admin` for local development only). Prometheus is at http://localhost:19090. Loki and Tempo are not exposed outside the compose network.

Override the host ports when needed, for example `GRAFANA_PORT=4300 make -C ops up`.

Stop it with `make -C ops down`. Local data is stored under `ops/stack/data/` and deliberately ignored by Git.

## Load-test safety

Load tests are opt-in:

```sh
TARGET_URL=http://127.0.0.1:8080 make -C ops perf-smoke
```

Use a seeded staging environment for load, spike, and soak tests. Never point these templates at production until workload, SLO, rate-limit exceptions, and an incident owner are approved.

## Not connected yet

The stack has no scrape target, log source, OTLP endpoint use, or k6 target configured for Judgify. Connecting it later requires a separate application change: health/metrics endpoints, structured stdout logs, trace propagation, and explicit Prometheus/Alloy target configuration.
