# Metric catalog

Metrics use stable, bounded labels only. Never use `user_id`, `submission_id`, `job_id`, email, source code, raw query text, or trace ID as a metric label.

| Metric | Type | Labels | Unit | Meaning |
| --- | --- | --- | --- | --- |
| `judgify_http_requests_total` | counter | role, route, method, status_class | requests | API traffic and errors |
| `judgify_http_request_duration_seconds` | histogram | role, route, method, status_class | seconds | API latency |
| `judgify_database_pool_connections` | gauge | state | connections | DB-pool saturation |
| `judgify_database_query_duration_seconds` | histogram | operation, outcome | seconds | DB operation latency |
| `judgify_judge_jobs` | gauge | state | jobs | durable judge queue depth |
| `judgify_judge_oldest_ready_job_seconds` | gauge | - | seconds | oldest claimable job age |
| `judgify_judge_worker_capacity` | gauge | kind | slots | total/free/claims-enabled worker capacity |
| `judgify_judge_finalizations_total` | counter | outcome | jobs | terminal judge finalization outcome |
| `judgify_sandbox_cgroup_events_total` | counter | event | events | timeout, OOM, and cgroup events |
| `judgify_websocket_connections` | gauge | - | connections | active realtime sessions |
| `judgify_process_resident_memory_bytes` | gauge | role | bytes | process memory pressure |

Every metric implementation must document its owner, update point, label cardinality bound, and whether it feeds an SLO or an alert.

