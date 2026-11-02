# Envoy Gateway and Request Identity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Judgify's public Nginx ingress with Envoy while preserving routes and security behavior and making Envoy the owner of public request IDs.

**Architecture:** A single Envoy process serves HTTPS and routes to the existing API and Next.js containers. It overwrites public request/correlation headers, passes an authenticated internal correlation value only on the internal path, and leaves Gin authorization authoritative until the authorization plan is complete. Deployment cutover waits for route, identity, and public-route proofs.

**Tech Stack:** Docker Compose, Envoy v3 static configuration, Go 1.27.1, Gin, Next.js/Node 22.22.0, shell/Python contract tests.

**Spec:** `docs/superpowers/specs/2026-09-13-envoy-gateway-observability-authz-design.md`

## Global Constraints

- Public path is `browser -> Envoy -> API or website`; no second public proxy or BFF.
- Preserve `/api/*` prefix removal, `/api` 308 redirect, `/ws` upgrade, public operational-path 404s, exact hostname/TLS, security headers, 4 MiB body limit, and no retries for mutations.
- Envoy owns `X-Request-ID`; public `X-Correlation-ID` equals the newly assigned request ID; all gateway-local responses return both.
- Preserve database `origin_cid` and `data.cid` semantics; never reinterpret them as the current request ID.
- Only `ingress` publishes the HTTPS port; all admin, API, worker, database, and telemetry listeners stay private.
- Existing Nginx config and Gin permission middleware remain usable until the final acceptance gate; do not discard unrelated working-tree changes. Before editing Next.js code, read the relevant `website/node_modules/next/dist/docs/` guide as required by `website/AGENTS.md`.

## File map and boundaries

- `deploy/envoy/envoy.yaml.tmpl` owns public listeners, clusters, route order, request-ID generation, security headers, response envelope, and access-log format.
- `scripts/deploy/render-envoy.py` validates the hostname and renders only allowlisted deployment values into the Envoy template; `deploy/compose.production.yml` mounts the rendered config and TLS secrets. The existing immutable N-1 release contains its own Nginx topology and is the production rollback artifact.
- `scripts/verify/envoy_gateway_test.sh` exercises public route/identity behavior against a local Compose stack; `scripts/verify/topology.sh` checks immutable image, network, listener, secret, and publication rules.
- `api/internal/infrastructure/gateway_identity.go` validates trusted gateway headers and stores current request ID separately from durable correlation context; `api/internal/infrastructure/router.go` installs this before request logging; `api/pkg/settings/config.go` and `api/internal/infrastructure/configuration/config.go` define and validate `gateway_identity_mode=envoy|legacy` for cutover and rollback.
- `website/lib/api/client.ts` consumes response IDs without generating public IDs; `website/lib/api/error.ts` permits an absent CID only for transport errors without a response; `website/lib/api/client.test.ts` proves refresh/replay and network-error behavior.
- The `go-common` changes needed to stop minting HTTP correlation values are scoped to `../go-common/pkg/common/http/middlewares/correlation.go` and its tests. Temporarily pin their committed SHA via `GO_COMMON_REF`; the final API module graph uses an immutable released version and drops the sibling `replace` and redundant CI checkout.

---

### Task 1: Envoy route and topology parity

**Files:** Create `deploy/envoy/envoy.yaml.tmpl`, `scripts/deploy/render-envoy.py`, `scripts/verify/envoy_gateway_test.sh`; modify `deploy/compose.production.yml`, `deploy/env.production.example`, `scripts/verify/topology.sh`, `scripts/verify/topology_test.sh`.

**Interfaces:** Render input is `JUDGIFY_PUBLIC_HOST` plus existing TLS file mounts; output is an absolute Envoy YAML file selected by `JUDGIFY_ENVOY_CONFIG_FILE`. The Compose service remains named `ingress`, so existing deployment commands keep their target.

- [ ] **Step 1 — red test:** Extend `topology_test.sh` with a fixture that expects the `ingress` image to be digest-pinned Envoy, `8443` as the sole published listener, a private admin listener, and no `reverse_proxy` config. Run `bash scripts/verify/topology_test.sh`; confirm it fails against the Nginx topology.
- [ ] **Step 2 — render contract:** Add a renderer test rejecting a hostname containing whitespace, slash, quote, or newline; assert a valid hostname is used only in the allowed SNI/Host match and not interpreted as configuration syntax. Run the test before the renderer exists to confirm failure.
- [ ] **Step 3 — minimal config:** Render a v3 config with exact-host TLS listener, private admin/health listener, `api:8000` and `website:3000` clusters, route order below, body limit, security headers, and disabled mutation retries:

  ```text
  /api/{live,ready,ping,metrics} -> local 404
  /api                         -> local 308 /api/
  /api/*                       -> strip /api/ -> api:8000
  /ws                          -> api:8000 with WebSocket upgrade
  /_next/static/*              -> website:3000
  /*                            -> website:3000
  ```

  Add `JUDGIFY_ENVOY_IMAGE` to `deploy/env.production.example` using the repository's deliberately invalid zero-digest placeholder convention; the private operator environment supplies a verified nonzero immutable digest. Extend topology verification to reject missing, tag-only, or zero digests for a release. Preserve `deploy/reverse-proxy.conf` in the N-1 release. Run `envoy --mode validate -c rendered.yaml` with the operator-selected digest, then `bash scripts/verify/topology_test.sh`.
- [ ] **Step 4 — running parity:** In `envoy_gateway_test.sh`, use `curl --resolve "$HOST:8443:127.0.0.1"` against local TLS and assert 308/404/rewrite/static/security-header/body-limit/hostname cases; perform a WebSocket handshake and reconnect with the existing frontend E2E fixture. Run the script against an isolated Compose project; confirm every case passes.
- [ ] **Step 5 — commit:** Commit only the files for this task after `git diff --check` and the rendered topology and gateway tests pass.

### Task 2: Public identity ownership and validation

**Files:** Modify `deploy/envoy/envoy.yaml.tmpl`, `api/internal/infrastructure/router.go`, `api/pkg/settings/config.go`, `api/internal/infrastructure/configuration/config.go`, `deploy/config.production.example.yaml`; create `api/internal/infrastructure/gateway_identity.go`, `api/internal/infrastructure/gateway_identity_test.go`; modify `scripts/verify/envoy_gateway_test.sh`.

**Interfaces:** `GatewayIdentity(mode GatewayMode) gin.HandlerFunc` validates the Envoy-issued pair when `mode=envoy`; `mode=legacy` is limited to local/dev transition tests. Production rollback activates the whole immutable N-1 release, which already includes Nginx and its previous API identity behavior; it does not hot-switch the new release to legacy mode. Public requests require canonical UUIDs and equal IDs. A separately authenticated internal gateway path may preserve a bounded legacy correlation ID with a fresh canonical request ID; this internal privilege must be enforced by the Check service in the authorization plan before the internal path is enabled.

- [ ] **Step 1 — red test:** In `gateway_identity_test.go`, exercise valid public pair, missing header, malformed UUID, unequal public pair, duplicate header, direct API request missing gateway headers, and legacy-mode Nginx fallback. Signed gateway-admission proof is added in the authorization plan. Example assertion:

  ```go
  require.Equal(t, http.StatusBadRequest, serveWithGatewayIdentity("bad-id", "bad-id").Code)
  require.Equal(t, http.StatusOK, serveWithGatewayIdentity(validUUID, validUUID).Code)
  ```

  Run `cd api && go test ./internal/infrastructure -run TestGatewayIdentity -count=1`; confirm the new cases fail.
- [ ] **Step 2 — Envoy ownership:** Configure the Envoy request-ID extension with `pack_trace_reason: false`, overwrite incoming `X-Request-ID`/`X-Correlation-ID` on the public listener, copy the request ID into correlation, and put both headers on upstream and local replies. Reject public `traceparent`, `tracestate`, client IP/forwarding, and internal-admission assertion spoofing before upstream routing. Validate the exact configuration with the pinned Envoy binary.
- [ ] **Step 3 — API admission:** Install `GatewayIdentity(mode)` before `RequestLogger` in `router.go`; select strict `envoy` mode for the new ingress and `legacy` only for local/dev transition fixtures. Production N-1 rollback uses its own older API binary and config. Do not call `correlation.New()` on accepted Envoy HTTP traffic. Keep the existing Gin permission middleware during this wave. Test that the request context and response headers contain the Envoy pair, including application errors.
- [ ] **Step 4 — wire proof:** Extend `envoy_gateway_test.sh` to send spoofed request/correlation/trace/forwarding headers and assert replaced IDs in both upstream response and gateway-local 404/413/503 responses. Check that the API is not published and a direct request missing gateway headers is denied; forged-header direct-access proof is completed with signed admission in the authorization plan.
- [ ] **Step 5 — commit:** Commit only gateway identity and tests after focused Go tests, Envoy validation, and gateway contract tests pass.

### Task 3: Browser client identity and durable replay

**Files:** Modify `website/lib/api/client.ts`, `website/lib/api/error.ts`, `website/lib/api/client.test.ts`, `api/internal/submission/adapters/driver/http/intake_test.go`, `api/internal/submission/adapters/driver/http/submission.go`; create `website/lib/api/error.test.ts`; add an identity E2E case to `scripts/verify/frontend_e2e.sh`.

**Interfaces:** Browser requests send neither `X-Request-ID` nor `X-Correlation-ID`. An Envoy response supplies both; a legacy Nginx rollback response may lack `X-Request-ID` and remains usable; a transport error without a response supplies neither. Idempotency keys remain client-generated and distinct from request/correlation IDs. Submission receipt `data.cid` remains the original durable submission CID.

- [ ] **Step 1 — red client tests:** Assert outbound `fetch` headers omit both IDs, a response exposes validated server IDs, a failed `fetch` with no response has no invented ID, and refresh retry receives its own new gateway request pair while keeping the same idempotency key. Run `cd website && npm test -- lib/api/client.test.ts`; confirm failures.
- [ ] **Step 2 — client change:** Remove per-request `randomUUID()` used only for correlation and remove the outbound correlation header. Make `ApiErrorOptions.cid` and `ApiError.cid` optional for timeout/network failures with no response; keep the CID required when parsing an API response envelope. Keep UUID generation for idempotency and WebSocket protocol request IDs; those are different contracts. Read and validate `X-Request-ID` and `X-Correlation-ID` from each actual response.
- [ ] **Step 3 — replay test:** Add an intake test asserting replay request pair `R2/R2` while returned receipt `data.cid` equals original `R1`, no second outbox item is produced; the observability plan adds the separate `origin_correlation_id=R1` replay log assertion. Run focused Go tests and then the website client tests.
- [ ] **Step 4 — E2E:** `test/e2e/stack.sh` currently writes and launches a Node reverse proxy, so add an explicit `JUDGIFY_E2E_INGRESS=envoy` branch there: render a local TLS Envoy config, launch it instead of `proxy.js`, point `JUDGIFY_E2E_BASE_URL` and allowed browser origin at Envoy, configure Playwright to trust only the disposable fixture certificate, and clean up the Envoy PID in `down`. Keep the existing mode available as a baseline. Exercise browser submission and replay with the same idempotency key; assert distinct HTTP request/correlation pairs and a stable receipt CID. Run `JUDGIFY_E2E_INGRESS=envoy bash scripts/verify/frontend_e2e.sh`; a passing old proxy run alone is insufficient.
- [ ] **Step 5 — commit:** Commit only client, API, and test changes after both focused suites pass.

### Task 4: Shared correlation dependency and gateway release gate

**Files:** Modify `../go-common/pkg/common/http/middlewares/correlation.go` and its tests, `api/go.mod`, `api/go.sum`, `api/internal/infrastructure/gateway_identity.go`, `scripts/verify/envoy_gateway_test.sh`, `test/e2e/stack.sh`, `.github/workflows/ci.yml`, `scripts/verify/supply_chain.sh`, `deploy/README.md`, `ops/runbooks/deploy.md`, and `ops/runbooks/rollback.md`.

**Interfaces:** A gateway-admitted public HTTP request never receives a third generated correlation ID from `go-common`. Autonomous jobs retain `correlation.New()`. Authenticated internal propagation is implemented and tested in the authorization plan, because no current website server-side protected API call site exists and the service-credential Check gate is not present yet. Commit `go-common` separately; use `GO_COMMON_REF` only until `api/go.mod` moves to its immutable module version.

- [ ] **Step 1 — red tests:** Add `go-common` tests that HTTP middleware never silently mints a third correlation ID for a gateway-admitted request. Add Envoy tests proving untrusted public and unauthenticated internal correlation values cannot be preserved.
- [ ] **Step 2 — implementation:** Keep the existing `go-common` minting behavior available only in explicit legacy rollback/dev mode; strict Envoy mode must validate gateway headers first and never mint another ID. Autonomous jobs continue using `correlation.New()`. Keep internal preservation disabled until the authorization plan installs the private Check service credential and admission assertion.
- [ ] **Step 3 — CI/release reproducibility:** Commit `go-common` first and temporarily set `GO_COMMON_REF` to its full 40-character commit for CI. Publish an immutable `go-common` module version from that commit; change `api/go.mod` to require that version, remove its `../../go-common` replacement, update `api/go.sum`, and remove the now-redundant `go-common` checkout and `GO_COMMON_REF` requirement from all CI jobs. Run `bash scripts/verify/go.sh` on a clean checkout with no sibling repo, then `make verify-release` with the required evidence environment and approved image inventory. `make verify-release` itself runs supply-chain `--check` and `--evidence`; do not invoke `--evidence` a second time against its now-nonempty directory. Never accept `go-common v0.0.0` or an uncommitted sibling checkout as release evidence.
- [ ] **Step 4 — CI and release/rollback:** Extend existing `cross-stack-contracts` or `cross-stack-packaging` with Envoy static validation, digest and topology checks; add the Envoy image to SBOM/CVE inventory in `scripts/verify/supply_chain.sh` and keep `final-gate` dependent on all API, website, and cross-stack jobs. Extend `cross-stack-e2e` to run the Envoy-enabled fixture. Execute `scripts/verify/topology.sh`, Envoy validation, route/identity E2E, and `make verify-release` before staging. The operator's trusted `JUDGIFY_DEPLOY_DRIVER` must implement the existing `scripts/deploy/release.sh` step contract; `switch-ingress-atomically` changes traffic only after candidate readiness, and `scripts/deploy/smoke.sh` proves the product path. Rehearse `scripts/deploy/rollback.sh /absolute/path/to/immediately-previous-release` with the immutable N-1 Nginx release and its old API identity mode; verify current schema compatibility and product smoke. No database rollback is attempted. Record the commands and evidence in the runbooks. Public Envoy cutover can occur with Gin authorization authoritative; internal correlation preservation waits for the Check credential gate.
- [ ] **Step 5 — commit:** Commit Judgify-side files separately from the `go-common` commit; record the `go-common` tag/version and source commit, `api/go.mod`/`api/go.sum` diff, and exact CI/release validation outputs in the PR. Retire the obsolete `GO_COMMON_REF` repository variable after all CI jobs stop reading it.

## Wave acceptance

Envoy is the only public ingress and preserves all previously tested HTTP/WebSocket behavior. Every public attempt has a new canonical `X-Request-ID` and equal `X-Correlation-ID`, including gateway-local failures; client-supplied IDs cannot win. The website does not manufacture either ID. Gin authorization still protects routes. The internal preservation path remains disabled until Check authenticates it; rollback to Nginx has been exercised. Signed direct-API admission and authenticated internal correlation are explicit gates in the authorization plan.
