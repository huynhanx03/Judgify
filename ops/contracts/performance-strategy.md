# Judgify performance strategy

## Purpose

A performance result is valid only when it declares the workload, environment, dataset, pass criteria, and observed outcome. A virtual-user count alone is not a capacity claim.

## Initial capacity objectives

| Profile | Target | Decision criteria |
| --- | --- | --- |
| Practice normal | 1,000 concurrent users | API p95 under 300 ms; 5xx under 0.5% |
| Contest steady state | 1,000 concurrent users | submit acknowledgement p95 under 500 ms; queue-age p95 under 10 s |
| Contest final spike | 50 submissions/s for 10 s | no lost or duplicate submission; queue drains in under 5 min |
| Soak | 200 concurrent users for 4 h | no sustained latency growth, resource leak, or connection leak |

These are starting hypotheses. A release may claim a target only after a report records measured evidence from an environment with production-like data and capacity.

## Required measurements

- Client: LCP, INP, CLS, browser errors, and successful user journeys.
- API: RPS, p50/p95/p99 latency, 4xx/429/5xx, active requests.
- Judge: incoming and completion rate, queue depth/age, queue wait, verdict latency, worker capacity, sandbox failures, and recovery time.
- Database: connection-pool pressure, query latency, lock wait, CPU, I/O, disk.
- Runtime: CPU throttling, memory/OOM, restarts, file descriptors, goroutines, and heap/GC.

## Evidence and release gate

Every report uses `performance/report-template.md`. A failed threshold, lost submission, unexplained queue growth, or missing environment metadata is a failed benchmark. The report must name a decision owner and a follow-up action.

