# Judgify operations

This directory is intentionally independent of `api/`, `website/`, CI, and the runtime compose files. It is a safe first step for operating and benchmarking Judgify without changing application behavior.

## Included now

- `stack/`: Grafana, Prometheus, Loki, Tempo, and Grafana Alloy. Prometheus scrapes the application over the private `judgify_telemetry` Docker network; Alloy receives application logs and OTLP traces.
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
`provisioning/`. Product targets appear when the application Compose joins
`judgify_telemetry`.

## Local stack

```sh
make -C ops up
make -C ops status
make -C ops validate
```

Grafana is available at http://localhost:3300 (user `admin`, password from
`GRAFANA_ADMIN_PASSWORD`). Prometheus is at http://localhost:19090. Loki and
Tempo are not exposed outside the compose network. Alertmanager currently
routes to `local-null`; configure an approved receiver before relying on pages.

Override the host ports when needed, for example `GRAFANA_PORT=4300 make -C ops up`.

Stop it with `make -C ops down`. Local data is stored under `ops/stack/data/` and deliberately ignored by Git.

Start this stack before the production application. It creates the private
`judgify_telemetry` network; application containers then connect directly to
`prometheus` scrape targets and Alloy OTLP receivers without host-loopback
hairpin routing. Stopping this stack while the application uses the network
will interrupt telemetry but does not stop business traffic.

Root request traces are sampled at 1% by Envoy, Next.js, and Go. A sampled
request's durable judge or outbox work starts a separate trace linked to that
origin. Unsampled journeys remain searchable by `correlation_id` in Loki.
Alloy's trace export queue is bounded to 256 batches and retries for at most
two minutes; Tempo retains local blocks for seven days. See
`observability/journey-proof.md` for the operator click path and evidence that
still requires a running staging environment.

## Load-test safety

Load tests are opt-in:

```sh
TARGET_URL=http://127.0.0.1:8080 make -C ops perf-smoke
```

Use a seeded staging environment for load, spike, and soak tests. Never point these templates at production until workload, SLO, rate-limit exceptions, and an incident owner are approved.

The load-test target remains opt-in. The stack does not store production
credentials in this repository.
