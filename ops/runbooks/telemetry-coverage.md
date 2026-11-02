# Telemetry coverage

Owner: Judgify operator. Dashboard: `/d/judgify-service-overview`.

The `JudgifyApplicationTargetUnavailable` alert means Prometheus still knows the
target but cannot scrape it. `JudgifyApplicationTargetMissing` means the target
has disappeared from service configuration or discovery. Treat a missing series
as **unknown**, never as a healthy zero. The local Alertmanager receiver is
`local-null`; it records alerts but does not notify anyone externally.

1. Open Prometheus `/targets` and locate the API, worker, Envoy, website, and
   Alloy jobs. Note the last scrape error and scrape timestamp.
2. Inspect the private `judgify_telemetry` network and verify that both Compose
   projects are attached. Check process readiness and the service's metrics
   endpoint from an authorized container on the private network.
3. If Alloy is down, inspect its own `/metrics`, component health and exporter
   queue. Check Tempo and Loki separately; do not assume an application outage.
4. Once the target returns, confirm fresh samples in Prometheus and a recent
   trace and log in Grafana. An old trace is not evidence of current coverage.

Do not put a public route or public host port in front of private metrics or
the Envoy admin endpoint to resolve a scrape failure.
