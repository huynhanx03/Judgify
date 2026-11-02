# Envoy Gateway, Observability, and Authorization Execution Map

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement the three linked plans task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the public Nginx ingress with Envoy, connect logs/metrics/traces, and move protected-route authorization to Envoy `ext_authz` without losing API defense or rollback.

**Architecture:** Keep one public Envoy gateway and the existing Next.js, Go API, worker, and database services. Envoy owns public request identity; the API owns durable correlation and policy decisions through a private Check endpoint. Alloy/Loki/Tempo/Prometheus/Grafana consume telemetry over private paths. Rollout uses the existing immutable release driver and N-1 rollback state machine.

**Tech Stack:** Envoy v3, Go 1.27.1, Gin/Casbin, Next.js/Node 22.22.0, Docker Compose, Grafana Alloy/Loki/Tempo/Prometheus/Alertmanager.

**Spec:** `docs/superpowers/specs/2026-09-13-envoy-gateway-observability-authz-design.md`

## Global Constraints

- Implementation units are the three linked plans below; this map establishes dependency order and release gates, not another implementation of their interfaces.
- Only Envoy publishes product HTTPS after cutover. No BFF or additional public proxy is introduced.
- `X-Request-ID` identifies an HTTP attempt; `X-Correlation-ID` carries the durable cause; `trace_id`/`span_id` belong to OpenTelemetry. The browser does not mint the first two.
- Loki labels begin with trusted `service` and `environment`; both are absent from JSON logs. `version` stays in production JSON; IDs never become index labels.
- Preserve existing database `origin_cid` and `data.cid` meanings, CSRF, ownership, contest, recent-reauthentication, and WebSocket topic checks.
- CI keeps separate API, website, and cross-stack tracks and their `final-gate`. Production release/rollback uses the trusted host driver.
- This document authorizes planning only. Tell the user when code implementation starts so they can switch model as requested.

---

## Documents and ownership

| Document | Owns | Entry gate | Exit evidence |
| --- | --- | --- | --- |
| [Gateway and identity](2026-09-13-envoy-gateway-identity.md) | TLS ingress, routes, headers, browser identity, `go-common` HTTP behavior | Current route baseline, immutable image selection | Envoy route/identity contracts, browser E2E through Envoy, N-1 rollback drill |
| [Observability](2026-09-13-judgify-observability.md) | JSON schema, trusted labels, traces, durable trace link, metrics, retention, alerts | Gateway IDs and private telemetry network | Cross-process correlation search, Tempo trace, Prometheus targets, alert delivery, collector outage proof |
| [External authorization](2026-09-13-envoy-ext-authz.md) | Route manifest, Check, signed admission, parity and enforcement | Gateway routes and identity; private Check wiring | Router-manifest coverage, 401/403/503 and bypass tests, parity per group, load and rollback evidence |

The spec and these three documents are the source of truth. Each task's files, interfaces, red test, implementation, verification, and scoped commit are in its owned plan. First capture passing old-route behavior, then build the new path, then compare auth decisions before removing Gin route permissions.

## Dependency order and cutover points

1. **Baseline before edits:** Record `git status`, existing `make verify-topology`, API/website CI status, `test/e2e/stack.sh` baseline, production ingress routes, and the immediately previous release identity. The workspace has unrelated changes; isolate execution and do not stage them.
2. **Gateway tasks 1–2:** Add digest-pinned Envoy config and renderer, validate route/TLS/header parity, install strict API gateway identity. Keep Gin permission middleware authoritative. Add an Envoy E2E fixture: the existing `test/e2e/stack.sh` creates a Node reverse proxy and cannot prove Envoy by itself.
3. **Gateway tasks 3–4:** Remove browser-generated correlation, prove idempotent replay semantics, update `go-common` HTTP correlation. Commit `go-common`; use `GO_COMMON_REF` only during transition, then publish an immutable module version, update `api/go.mod`/`api/go.sum`, remove the sibling replacement and obsolete CI checkout/variable. Add Envoy image to image inventory and SBOM/CVE scans.
4. **Gateway cutover G1:** Static Envoy validation, live route/identity E2E, source/supply-chain gates, candidate readiness, and N-1 schema compatibility pass. The trusted driver runs `scripts/deploy/release.sh /var/lib/judgify/releases/RELEASE_ID`; `switch-ingress-atomically` follows readiness. Run `scripts/deploy/smoke.sh` and browser smoke after activation. Internal correlation preservation can remain disabled; Gin still authorizes.
5. **Observability tasks 1–4:** Prove Docker syslog → Alloy → Loki labels before removing Go `service`/`environment`; add OTel and nullable trace columns; connect private metric targets; enforce retention and alert proof. Alloy/Tempo outage must leave traffic and authorization unchanged. Prove N-1 can read expanded schema.
6. **Authorization tasks 1–3:** Derive a complete manifest from live Gin routes; add private gRPC Check and signed admission; reject forged/direct access. Keep internal correlation preservation disabled until Check verifies service authority. Retain Gin `RequirePermission`.
7. **Comparison G2:** Check evaluates candidate decisions, Gin decides user-visible protected requests. Compare anonymous/user/admin/revoked/expired/refresh/OAuth/CSRF/recent-reauth/WebSocket cases per route group. Infrastructure failures fail closed; unknown routes never receive comparison Allow. Any mismatch blocks enforcement.
8. **Enforcement G3:** Switch one verified route group per reviewed release/config step. Remove only its route-level `RequirePermission`; keep domain, CSRF, session, and WebSocket topic checks. Rerun its matrix, direct API denial, forged assertion, timeout, and revocation tests after each switch.
9. **Final G4:** Record p50/p95/p99 gateway/Check latency and CPU/memory under representative and burst load, query real dashboard targets, deliver an alert, and rehearse N-1 rollback. Complete only when every protected route has explicit policy and the final PR gate and production smoke pass.

## CI and release evidence matrix

| Gate | Existing place to extend | Required evidence and failure signal |
| --- | --- | --- |
| API/database | `.github/workflows/ci.yml` API jobs; `scripts/verify/go.sh`, `scripts/verify/migrations.sh --integration` | Check/admission tests, router-manifest drift, nullable migration replay, N-1 compatibility; failure blocks `api-gate` |
| Website | Existing website jobs; `scripts/verify/frontend.sh source` and `scripts/verify/frontend.sh unit` | No browser identity pair, response/error and replay tests; failure blocks `website-gate` |
| Static cross-stack | `cross-stack-contracts`; `scripts/verify/topology.sh`, renderer tests, Envoy `--mode validate` | Route/image/config invariants; reject tag-only/zero release digest, public telemetry/admin port, or manifest drift |
| Live cross-stack | `cross-stack-e2e`; `JUDGIFY_E2E_INGRESS=envoy bash scripts/verify/frontend_e2e.sh` plus gateway/authz contracts | Real Envoy route/TLS/WS/identity, anonymous/user/admin, 401/403/503, bypass, replay; old Node-proxy run is insufficient |
| Packaging/security | `cross-stack-packaging`; `scripts/verify/supply_chain.sh` | Build/scan existing five product images and Envoy with SBOM/CVE evidence and Gitleaks. Pin and scan all six production ops images (Prometheus, Alertmanager, Loki, Tempo, Alloy, Grafana) in the approved release image inventory; Grype's existing `--fail-on high` threshold applies without silent exceptions |
| PR summary | Existing `api-gate`, `website-gate`, `cross-stack-gate` → `final-gate` | All three independent results equal success; retain this DAG |
| Release checkout | `make verify-release` (includes `scripts/verify/supply_chain.sh --check` and `--evidence`) | Clean immutable tree; no sibling `go-common` replace or `v0.0.0`; approved digest-pinned inventory; set `JUDGIFY_RELEASE_EVIDENCE_ACK=GENERATE_UNSIGNED_RELEASE_EVIDENCE`, `JUDGIFY_RELEASE_EVIDENCE_DIR` to an empty secure directory outside source, and `JUDGIFY_RELEASE_IMAGES_FILE` to approved inventory. Run once per empty evidence directory. Record source SHA, immutable `go-common` module version and source commit, image digests, SBOM/CVE, migration version |

`deploy/env.production.example` intentionally uses invalid zero digests. The operator's private environment supplies real digests. `JUDGIFY_DEPLOY_DRIVER` is a protected host executable outside release and writable state directories. Define its Envoy candidate/readiness/switch/smoke behavior and test with a fixture; never bypass `scripts/deploy/release.sh` with direct Compose commands. The release command requires `JUDGIFY_DEPLOY_ACK=DEPLOY_JUDGIFY_RELEASE`, `JUDGIFY_DEPLOY_DRIVER=/etc/judgify/deploy-driver`, and `JUDGIFY_DEPLOY_STATE_ROOT=/var/lib/judgify/deploy` as documented in `ops/runbooks/deploy.md`.

The driver must preserve the script's exact order: artifact/provenance/config/migration/backup verification → expand migration → new worker claims-off and exact-tuple smoke → worker claims switch/drain → API candidate and loopback readiness → exact capability activation → `switch-ingress-atomically` → old API drain → product smoke. Envoy config validation and TLS secret checks belong in `verify-config`; no early traffic switch is allowed. The driver is host-owned and remains outside this repo's mutable release directory.

## Production rollback contract

Before G1, G2, or G3, identify the immediately previous immutable release and verify it can read the current expanded database schema. On failed smoke or security regression, set `JUDGIFY_ROLLBACK_ACK=ROLL_BACK_JUDGIFY_BINARY_ONLY` with the same trusted driver/state root, then run `scripts/deploy/rollback.sh /var/lib/judgify/releases/PREVIOUS_RELEASE_ID`. The N-1 release carries its own ingress config and matching API identity/permission mode; it is Nginx at G1 and may be Envoy in later waves. Database migrations are not reversed. If schema compatibility fails, stop before traffic switch. Record active release identity and `scripts/deploy/smoke.sh` result after rollback.

## Review checklist before implementation handoff

- [ ] Reconfirm the live route catalog, CI job names, and release-driver step list; update exact file names if they changed.
- [ ] Confirm candidate Envoy digest and image scan policy against release inventory; keep example zero digest invalid.
- [ ] Confirm Envoy E2E fixture exercises same-origin, TLS, CSRF origin, WebSocket, and API rewrite.
- [ ] Confirm immutable `go-common` module release and supply-chain evidence run from a clean checkout.
- [ ] Confirm each wave has red/green tests, independent review, production smoke, and N-1 rollback exercise before the next cutover.

No code, CI workflow, Compose stack, or production service is changed by writing this plan.
