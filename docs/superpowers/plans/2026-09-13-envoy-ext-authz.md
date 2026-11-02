# Envoy External Authorization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce every protected HTTP route at Envoy's `ext_authz` boundary using Judgify's existing credential verifier and Casbin policy, without creating a direct-API bypass.

**Architecture:** A private gRPC `Check` endpoint runs in the existing Go API process. A single route policy manifest maps normalized method/path to public, optional, protected, WebSocket, or operational behavior. On Allow, Check issues a short-lived signed admission assertion bound to the request; the API verifies it before constructing principal context. Gin permission checks stay authoritative in comparison mode and are removed route group by route group only after parity tests pass.

**Tech Stack:** Envoy gRPC `envoy.service.auth.v3.Authorization/Check`, Go 1.27.1, Gin, existing `authentication.Verifier` and `authorization.Authorizer`, Casbin policy snapshot, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-09-13-envoy-gateway-observability-authz-design.md`

## Global Constraints

- Requires gateway plan `docs/superpowers/plans/2026-09-13-envoy-gateway-identity.md`; observability plan `docs/superpowers/plans/2026-09-13-judgify-observability.md` supplies Check metrics/traces but is not an authorization dependency.
- Never trust client-supplied `X-User-ID`, admission assertion, forwarding headers, correlation, trace-sampling request, or route-policy hint.
- Check does not inspect request bodies or cache user permission decisions. Credential/session verification and Casbin policy revision remain authoritative.
- Invalid/missing credential is 401; authenticated denial is 403; Check error, timeout, or dependency failure is 503 and fail closed.
- Domain ownership, contest state, CSRF, recent reauthentication, OAuth/refresh boundaries, and WebSocket ticket/topic checks remain in API.
- Enforced cutover requires route-manifest coverage, direct API denial, assertion binding, policy-revocation parity, outage behavior, and measured p50/p95/p99/CPU/memory.

## File map and boundaries

- `api/internal/infrastructure/authz/route_policy.go` defines `RoutePolicy{Method, Pattern, Class, AnyOf}` and a deterministic resolver; `route_policy_test.go` compares it with the live Gin `Routes()` catalog.
- `api/internal/infrastructure/authz/check_server.go` implements Envoy's gRPC Check interface and adapts to existing verifier/authorizer ports; `check_server_test.go` covers policy and status mapping.
- `api/internal/infrastructure/authz/admission.go` signs/verifies a purpose-specific short-lived request assertion; `admission_test.go` covers tampering, binding, expiry, and key rotation.
- `api/internal/infrastructure/router.go` installs admission verification and later removes only route-level `RequirePermission`; identity route files retain domain and CSRF checks.
- `deploy/envoy/envoy.yaml.tmpl` invokes Check over mTLS, strips/injects the admission header, and has a separate private mTLS listener for service-to-service calls; `deploy/compose.production.yml` mounts gateway/Check certificates and a distinct admission signing key with narrow read access. The API gRPC Check listener is not published.
- `scripts/verify/ext_authz_test.sh`, `api/internal/architecture/authorization_catalog_test.go`, and `ops/runbooks/authorization-stale.md` supply contract, drift, and rollback proof.

---

### Task 1: Complete route policy inventory and drift gate

**Files:** Create `api/internal/infrastructure/authz/route_policy.go`, `api/internal/infrastructure/authz/route_policy_test.go`; modify `api/internal/architecture/authorization_catalog_test.go` and the route files under `api/internal/*/adapters/driver/http/routes.go` only where an existing route lacks a stable explicit classification.

**Interfaces:** `Resolve(method, normalizedPath string) (RoutePolicy, bool)` returns an explicit policy or false. `Class` is one of `public`, `optional_identity`, `protected`, `refresh`, `oauth_callback`, `websocket`, `operational`. `AnyOf` contains `{Resource, Action}` pairs from `api/pkg/permissions`; false never means public.

- [ ] **Step 1 — red coverage test:** Build the real Gin engine in a test, enumerate `Routes()`, normalize Gin `:param` and Envoy path matching identically, and assert every `method + full path` resolves to exactly one policy. Add a fake route and assert the test fails for an unclassified route.
- [ ] **Step 2 — manifest:** Encode the actual public, optional, protected, refresh, OAuth callback, WebSocket, and operational route catalog. For routes with alternate permissions, use an explicit `AnyOf` list; reject duplicate/shadowed patterns and URL-encoded path ambiguity. Example table entry:

  ```go
  {Method: "POST", Pattern: "/problems", Class: "protected",
   AnyOf: []Capability{{Resource: permissions.ResourceKeyProblem,
                        Action: permissions.PermissionScopeCreate}}}
  ```

  Resolve missing/ambiguous routes as deny. Use the existing permission catalog constants rather than inventing resource strings.
- [ ] **Step 3 — verify:** Run `cd api && go test ./internal/infrastructure/authz ./internal/architecture -count=1`. Assert route count, class count, and the exact protected set are recorded as test output for the PR.
- [ ] **Step 4 — commit:** Commit manifest and drift tests only after they pass; a newly registered route must cause CI to fail until classified.

### Task 2: Private Check service with correct failure semantics

**Files:** Create `api/internal/infrastructure/authz/check_server.go`, `api/internal/infrastructure/authz/check_server_test.go`; modify `api/internal/infrastructure/api.go`, `api/internal/di/dependencies.go`, `api/go.mod`, `api/go.sum`, `deploy/compose.production.yml`, `deploy/envoy/envoy.yaml.tmpl`, `ops/runbooks/deploy.md`.

**Interfaces:** `Check(context.Context, *authv3.CheckRequest) (*authv3.CheckResponse, error)` accepts only allowlisted method, normalized path, authority, bounded bearer/cookie credential, gateway IDs, and trace context. It calls existing `authentication.Verifier` and `authorization.Authorizer.Authorize(ctx, authorization.Request{Principal, Resource, Action})`. The gRPC listener has no published host port and verifies a gateway-only mTLS client certificate SAN; production certificates and CA are operator-managed secrets outside Git. An untrusted container on the same network has no Check authority.

- [ ] **Step 1 — red matrix:** In `check_server_test.go`, cover public no-credential Allow; optional identity with valid credential Allow and invalid presented credential 401; protected missing/invalid credential 401; valid user insufficient permission 403; authorized user Allow; authorizer error 503; unknown route 403; malformed path/oversized credential fail closed. Add a transport test: a client without the gateway certificate, with an untrusted CA, or with the wrong SAN cannot call Check even from the private network. Confirm tests fail before the server exists.
- [ ] **Step 2 — Check adapter:** Reuse the already constructed verifier and authorization service from DI; do not instantiate a second Casbin runtime. Bind the gRPC Check listener on the private network with client-certificate verification and an exact gateway SAN allowlist. Bound credential bytes and reject unexpected authority/method/path. Map Check-denial HTTP status and JSON error envelope with current CID; do not return raw verifier/Casbin errors. Document certificate issuance/rotation and the mounted secret paths in `ops/runbooks/deploy.md`.
- [ ] **Step 3 — Envoy filter:** Configure an mTLS gRPC cluster and the `ext_authz` filter on every API route classified by the manifest (including public/optional identity, excluding gateway-local operational denials) with a bounded timeout, `failure_mode_allow: false`, explicit allowlisted headers, and no request-body buffering. In enforcement mode a Check timeout/dependency error produces 503 and ordinary lack of permission produces 403. In comparison mode, Check evaluates and records its candidate decision but returns Allow plus request-bound gateway admission for catalogued routes, leaving Gin authoritative; infrastructure errors still fail closed and unknown routes never get comparison Allow.
- [ ] **Step 4 — running proof:** Run focused Go tests and `bash scripts/verify/ext_authz_test.sh` against an isolated Compose stack; stop Check and assert protected requests fail 503 while public health behavior remains as specified.
- [ ] **Step 5 — commit:** Commit Check wiring, generated module sums, Envoy filter, and tests after Go and running failure tests pass.

### Task 3: Signed admission and authenticated internal propagation

**Files:** Create `api/internal/infrastructure/authz/admission.go`, `api/internal/infrastructure/authz/admission_test.go`; modify `api/internal/infrastructure/router.go`, `api/internal/infrastructure/gateway_identity.go`, `deploy/envoy/envoy.yaml.tmpl`, `deploy/compose.production.yml`, `deploy/env.production.example`, `scripts/verify/ext_authz_test.sh`.

**Interfaces:** Allow result carries a signed assertion with purpose, request ID, normalized method/path, principal/session ID, issuance/expiry, and policy revision. The API validates signature, purpose, time, and exact request binding before setting principal context. Gateway strips any incoming assertion before Check. The internal Envoy listener requires operator-issued mTLS client identities for `website` and `worker`; their exact client SAN acts as the service credential and is passed to Check through an Envoy-generated allowlisted metadata/header value. Public clients cannot reach this listener or supply that metadata. Browser credentials and service certificates are separate trust roots.

- [ ] **Step 1 — red assertion tests:** Cover valid assertion, wrong method/path/request ID, expired/not-yet-valid assertion, altered principal, altered policy revision, wrong purpose, unknown key ID, and replay on a second request. Assert none creates principal context. Cover direct `api:8000` call without a Check-issued assertion.
- [ ] **Step 2 — signer/verifier:** Use a dedicated secret mounted only in the API process that hosts Check and assertion verification, constant-time MAC verification, bounded token size, short expiry no longer than the request budget, and key-ID support for rotation. A browser credential key must not validate an admission token. Preserve the verified principal's existing authentication context type.
- [ ] **Step 3 — internal credential:** Configure the internal listener to require valid client certificates with the exact website/worker SAN allowlist; only Envoy-generated verified service identity reaches Check. Check authorizes that service identity for the target route before allowing caller-supplied `X-Correlation-ID`; validate length/format, overwrite `X-Request-ID`, and forward only that verified inherited correlation. Strip identity metadata, assertion, trace-force, and forwarding headers on the public listener. An unauthenticated internal attempt fails TLS/Check and cannot preserve correlation.
- [ ] **Step 4 — bypass proof:** Run direct API, forged assertion, wrong route, changed request ID, stale session, service-credential spoof, and public-correlation-spoof tests. Confirm an allowed website-to-API hop has `request_id=R2, correlation_id=R1` and both belong to the same trace where propagated.
- [ ] **Step 5 — commit:** Commit admission and internal-boundary changes only after focused Go tests and running bypass tests pass.

### Task 4: Compare and enforce route groups

**Files:** Modify `api/internal/infrastructure/router.go`, affected `api/internal/*/adapters/driver/http/routes.go`, `api/internal/architecture/authorization_catalog_test.go`, `deploy/envoy/envoy.yaml.tmpl`, `scripts/verify/ext_authz_test.sh`, `ops/runbooks/authorization-stale.md`, `ops/runbooks/rollback.md`.

**Interfaces:** Comparison mode evaluates Check but returns Allow for catalogued routes, records the candidate decision, and lets legacy Gin decide the user-visible result. The isolated parity matrix pairs candidate decision with the observed Gin decision using the request ID. Enforcement switches route groups with a versioned config flag; the API still verifies admission and retains domain/CSRF/WebSocket-specific authorization.

- [ ] **Step 1 — red parity matrix:** Exercise anonymous, normal user, admin, revoked user, policy revision change, access-token expiry, refresh, OAuth callback, CSRF failure, recent reauthentication, and WebSocket topic denial; compare candidate and legacy decisions for every protected route group. Any mismatch blocks cutover.
- [ ] **Step 2 — comparison mode:** Keep `RequirePermission` attached; Check returns Allow in comparison mode after candidate evaluation and emits bounded `authz.candidate` diagnostics. The parity harness compares each candidate with the legacy Gin result and emits `authz.parity_mismatch` only for disagreements, using route pattern, policy revision, and correlation ID. Count all comparisons in metrics. Do not put credential/body values in logs.
- [ ] **Step 3 — incremental enforcement:** For each route group with zero mismatches, remove only its route-level `RequirePermission` adapter while retaining Gin principal/domain checks; run that group's matrix immediately. Keep new/unclassified routes denied by Check and the router catalog gate.
- [ ] **Step 4 — failure proof:** Repeat 401/403/503, Check timeout, Check service down, forged headers, direct API, CSRF, OAuth/refresh, and WebSocket topic cases after each switch. Verify a Check outage fails closed on protected routes and does not turn into 403.
- [ ] **Step 5 — commit:** Commit each reviewed route group as a separate change with its parity output and direct-access proof. A staged same-release toggle may re-enable Gin enforcement only if it was built and tested in that release; the production rollback path uses `scripts/deploy/rollback.sh` against the immediately previous immutable release, whose ingress and API policy wiring are a compatible pair.

### Task 5: Performance, release, and rollback acceptance

**Files:** Create `scripts/verify/ext_authz_load.sh`; modify `ops/runbooks/deploy.md`, `ops/runbooks/rollback.md`, `ops/provisioning/prometheus-rules/judgify.yml`, `.github/workflows/ci.yml`.

**Interfaces:** A release is allowed only when source contracts, API/website suites, Envoy config validation, topology, live auth matrix, and measured overhead pass. CI keeps API and website tracks separate and a final summary gate depends on both, as in the existing workflow.

- [ ] **Step 1 — benchmark harness:** Run representative authenticated reads, mutations, and bursts through Envoy with Check enabled and disabled in an isolated environment. Record p50/p95/p99 request and Check latency plus CPU/memory for both modes; do not set an acceptable limit by guesswork.
- [ ] **Step 2 — release tests:** Run `bash scripts/verify/go.sh`, `bash scripts/verify/frontend.sh source`, `bash scripts/verify/frontend.sh unit`, `bash scripts/verify/topology.sh`, Envoy validation, `bash scripts/verify/ext_authz_test.sh`, and `JUDGIFY_E2E_INGRESS=envoy bash scripts/verify/frontend_e2e.sh`. Run `make verify-release` only from the clean, immutable release checkout with the required supply-chain evidence environment and digest inventory; a dev checkout with local `go-common` replacement cannot satisfy this gate. Add the static authz catalog to `cross-stack-contracts`, live matrix to `cross-stack-e2e`, and image/packaging checks to `cross-stack-packaging`; keep separate API/website jobs and the existing `final-gate`.
- [ ] **Step 3 — rollback drill:** Rehearse `scripts/deploy/rollback.sh /absolute/path/to/immediately-previous-release` under the trusted host `JUDGIFY_DEPLOY_DRIVER`. The N-1 artifact brings back its own ingress (Nginx at the first gateway cutover, usually Envoy during later authz waves), API binary, and matching Gin/Check policy wiring after `verify-current-schema-compatible`; the script does not reverse database migrations. Re-run `scripts/deploy/smoke.sh` and the anonymous/user/admin matrix, and record transaction ID, step results, elapsed time, and active release identity in `ops/runbooks/rollback.md`. Abort cutover if N-1 cannot read the expanded schema.
- [ ] **Step 4 — alert proof:** Trigger a bounded test Check timeout/error alert through Alertmanager; verify delivered notification and linked runbook. Confirm API/worker/gateway scrape targets stay private.
- [ ] **Step 5 — final commit:** Commit benchmark harness, CI, alerts, and runbook changes after all release tests and rollback drill pass.

## Completion gate

Every live protected HTTP route has an explicit manifest policy, a verified Check decision, and a valid request-bound admission assertion before API principal construction. The direct API path, forged headers, and WebSocket topic route do not bypass authorization. 401/403/503, revocation, and timeout behavior are proven. Gateway and Check overhead is measured, the alert is delivered, and rollback is rehearsed.
