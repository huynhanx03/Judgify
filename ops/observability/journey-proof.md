# Observability journey: current evidence and operator path

Status: **implementation builds; running journey unmeasured**. This repository
does not contain a deployed staging environment, release images, credentials,
submission fixtures, or an external Alertmanager receiver. Do not treat a
successful build or a green readiness probe as evidence of an end-to-end judge
journey.

## Open one request and follow it

1. Open Grafana `Judgify / Service Overview`. Confirm each of the five scrape
   targets (`judgify-api`, `judgify-worker`, `judgify-envoy`,
   `judgify-website`, `judgify-alloy`) has a fresh `up = 1` sample. The
   **Missing scrape targets** panel must be empty; any series there is an
   unknown coverage state, not a healthy zero.
2. In Loki, select the `judgify-envoy` service and find an `HTTP request
   completed` JSON line. It carries `request_id`, `correlation_id`, optional
   sampled `trace_id`, numeric `duration_ms`, and one `metadata` object with
   method, route, status, upstream, and byte counts. Select **View trace** on
   the `trace_id` field to open Tempo.
3. In Tempo, inspect the gateway and API spans in time order. Check HTTP and
   database timings separately. Tempo's configured trace-to-logs query opens
   Loki in the span's time window. For an unsampled request, search Loki by
   `correlation_id`; an absent trace is expected under the 1% policy.
4. Submit a real staging job. Find the outbox and judge transition logs using
   the same `correlation_id`. The worker's `judge.attempt` is a new trace with
   a link to the sampled origin. Its top-level `duration_ms` measures attempt
   execution; `judgify_judge_oldest_ready_job_seconds` and job timestamps
   describe queue wait. Do not add queue wait to a single active HTTP span.
5. Inspect `judgify_outbox_events{state="dead"}` and the judge queue panels.
   Repeat after a safe retry and confirm each attempt gets its own linked
   worker span, while `request_id` appears only on the original live HTTP hop.

## Evidence still required before production claims

| Stage | State | Required evidence |
| --- | --- | --- |
| Static website and API build | Checked locally | `npm run build`, `go build ./...` |
| Compose structure | Checked locally | Both Compose files parse |
| Prometheus target health | Unmeasured | Fresh `/api/v1/targets` response from staging |
| Loki to Tempo and back | Unmeasured | One real sampled request opened in both data sources |
| Durable trace link and retry | Unmeasured | Real terminal submission plus retry fixture |
| Browser Web Vitals | Unmeasured | Sampled browser event in Loki, with bounded fields |
| Collector outage | Unmeasured | Staging product completion and bounded Alloy queue |
| Alert notification | Unmeasured | Approved external receiver and delivery receipt |

The provisioned receiver is `local-null`; firing alerts are visible in
Prometheus and Alertmanager but are not delivered to an operator. Choose an
approved receiver and secret path before claiming paging coverage.
