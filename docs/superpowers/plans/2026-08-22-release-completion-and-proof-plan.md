# Judgify Release Completion and Proof Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Turn the current feature-complete Judgify source tree into a release candidate whose critical paths have been executed and whose verification gates demonstrably fail closed.

**Architecture:** Do not add broad product features before this plan is closed. The source already contains the intended single-tenant product planes. Remaining work is proof: reconcile evidence, make verification self-verifying, execute the judge path on a suitable host, repair the WebSocket lifecycle defect, then collect release evidence.

**Tech Stack:** Go 1.26, Next.js, PostgreSQL 16, Ent, Casbin, Forge local durable queue, coder/websocket, Docker/Compose, rootless Docker with cgroup v2, Playwright, k6, Atlas, shell.

**Spec:** docs/superpowers/specs/2026-07-16-judgify-production-master-design.md. Implementation map: docs/PRODUCT_COMPLETION_STATUS.md. Operational checklist: REMAINING_WORK.md.

## Global Constraints

- Single tenant / one-machine target. Do not add Kafka, Redis, SSE, polling, or a new frontend framework.
- Keep Forge as local durable queue, the shared browser WebSocket, and outbox as the sole durable event-to-browser route.
- Never weaken rootless Docker, cgroup v2, sandbox, digest pinning, CSRF, origin, cookie, authorization, or least-privilege guards to pass macOS.
- Every release gate needs one intentional negative mutation proving it exits non-zero for the intended reason.
- Destructive commands run only against a uniquely named disposable environment and require their acknowledgement variables.
- Keep Vietnamese text centralized; no mock production data, direct browser fetch, any bypass, SSE, or polling.
- Do not commit, stage, reset, or delete unrelated dirty-worktree changes without explicit approval.

---

## Current Truth: Evidence Conflict Must Be Resolved First

The two existing status files conflict. Do not hand either one to another AI as the sole source of truth.

| Subject | Most recent evidence | Stale statement that must be removed or marked superseded |
|---|---|---|
| Backup / restore | REMAINING_WORK.md records test/operations/run_backup_restore_proof.sh passed after blank restore and domain-invariant proof. | PRODUCT_COMPLETION_STATUS.md still says its driver was absent or had never run. |
| Playwright | REMAINING_WORK.md says stack booted and the measured remaining defect is old/new WebSocket overlap during login. | PRODUCT_COMPLETION_STATUS.md still says 7 pass / 567 fail from an earlier cookie/session snapshot. |
| Resilience | test/chaos/driver.sh and test/operations/resilience_assertions.sh exist, but no runtime load/chaos run has happened. | Older status text says both drivers are absent. |
| Static suites | Later notes report API, go-common, generated/baseline/contracts, and frontend verify/build/bundle green. | These results apply only to their exact source snapshot and must be rerun after repair. |

Until a dated receipt is written, the safe status is:

- **Implemented but not release-proven:** all feature planes in PRODUCT_COMPLETION_STATUS sections 3–4.
- **Proven once:** PostgreSQL baseline/migrations/roles, API readiness, image builds, blank backup/restore proof, and judge-worker boot through capability probe.
- **Release blockers:** WebSocket handoff and browser E2E; actual sandbox judgement; worker restart/lease/replay; load/chaos/soak; release supply-chain evidence; reproducible sandbox inputs; docs/dirty-tree review.
- **External blocker, not a source defect:** macOS OrbStack/Docker Desktop cannot satisfy the strict Linux rootless Docker plus cgroup-v2 probe. Never workaround it in code.

## Ordered Completion Matrix

| Priority | Deliverable | Done only when | Dependency |
|---|---|---|---|
| P0 | Evidence reconciliation | One dated receipt has command, tree ID, exit status, artifact path, and no contradictory prose. | None |
| P0 | Gate mutation proof | Every release script has an isolated bad input that fails closed. | None |
| P0 | One WebSocket per tab | Session transition has no temporal overlap and reconnect/replay still works. | Local E2E stack |
| P0 | Browser release E2E | Real API/DB/browser product flows pass with traces/evidence. | Local E2E stack |
| P0 | Actual judgement | Accepted and rejected solutions execute, finalize once, notify, and render. | Linux rootless Docker host |
| P0 | Resilience | 23 chaos scenarios plus load converge without loss/duplication. | Linux rootless Docker host, k6 |
| P1 | Recovery/reproducibility | Backup/restore is repeated and sandbox inputs are immutable. | Artifact location |
| P1 | Supply chain | Released go-common, clean tree, SBOM/scans/image inventory/provenance. | Publish/tag authority |
| P1 | Final handoff | Docs match evidence; all final gates rerun; dirty paths classified. | None |

## Task 1: Create a Single Evidence Ledger and Reconcile Docs

**Files:**
- Create: docs/release/evidence/2026-08-22-release-proof.md
- Modify: REMAINING_WORK.md
- Modify: docs/PRODUCT_COMPLETION_STATUS.md
- Test: scripts/verify/workflows_test.sh

**Produces:** one append-only receipt table used by both status files.

- [ ] **Step 1: Record the exact baseline**

~~~sh
cd /Users/jerry/nhanhmc/coding/Judgify
git rev-parse HEAD
git status --short
git diff --check
~~~

Every evidence row must contain UTC time, exact command, Git HEAD, intentional dirty-tree note, exit code, artifact directory, and interpretation.

- [ ] **Step 2: Supersede instead of overwrite**

Replace stale backup/Playwright/resilience claims in PRODUCT_COMPLETION_STATUS with links to the ledger. REMAINING_WORK stays short and operational. No E2E claim is valid unless linked to Playwright report and WebSocket evidence.

- [ ] **Step 3: Add a stale-claim fixture test**

Extend scripts/verify/workflows_test.sh with fixture text that contains both PASS and CHƯA CHẠY for the same named gate. The parser must exit non-zero and name the duplicated gate. Test fixture text, not historical ledger content.

- [ ] **Step 4: Verify**

~~~sh
cd /Users/jerry/nhanhmc/coding/Judgify
sh scripts/verify/workflows_test.sh
~~~

## Task 2: Prove Every Verification Gate Fails Closed

**Files:**
- Create: scripts/verify/release_gate_mutation_test.sh
- Modify: scripts/verify/contracts.sh
- Modify: scripts/verify/*_test.sh as required
- Test: scripts/verify/release_gate_mutation_test.sh

**Produces:** a test that copies fixtures into a temporary directory, makes one controlled defect, runs the real script, and requires non-zero plus the expected diagnostic.

- [ ] **Step 1: Implement these mandatory mutations**

~~~text
generated: change generated output -> "generated check: generated files changed"
migrations: remove atlas.sum binding -> "migration verification: atlas.sum"
topology: remove image digest -> "image must be pinned"
database baseline: break append-only trigger expectation -> baseline rejects mismatch
resilience --check: remove required driver/catalog asset -> resilience error
supply chain: add external filesystem replace -> "external filesystem replace"
go release: omit coverage floor -> "requires the measured coverage floor"
~~~

- [ ] **Step 2: Make contracts invoke the mutation suite**

Add it to the explicit executable list in scripts/verify/contracts.sh. It must not use Docker, mutate databases, or require the Linux worker host.

- [ ] **Step 3: Verify positive and negative behavior**

~~~sh
cd /Users/jerry/nhanhmc/coding/Judgify
sh scripts/verify/release_gate_mutation_test.sh
sh scripts/verify/contracts.sh
~~~

The test script exits zero only after observing every expected failure. Also change resilience --check success copy to say static assets only; it is not runtime proof.

## Task 3: Repair WebSocket Session Handoff

**Files:**
- Modify: website/contexts/realtime-context.tsx
- Modify: website/lib/realtime/websocket-client.ts
- Modify: website/e2e/support/quality.ts
- Modify: website/e2e/admin.spec.ts
- Test: website/contexts/realtime-context.test.tsx (create if absent)

**Produces:** an awaitable close/handoff fence. A replacement client cannot open until the old client is terminal or a bounded timeout has fenced its generation.

- [ ] **Step 1: Write failing lifecycle test**

Simulate anonymous client A with a subscription; change session to authenticated; delay A close event; assert client B is not constructed. Release A close; assert B is constructed and subscriptions replay exactly once.

- [ ] **Step 2: Implement generation-fenced handoff**

Cancel old ticket/reconnect work, close, await terminal close with bounded timeout, then construct replacement. A stale generation cannot update context state or resubscribe. Preserve anonymous websocket behavior.

- [ ] **Step 3: Make Playwright check overlap over time**

Capture open and close timestamps. Fail if two WebSockets overlap within one tab lifecycle; do not use only a final count.

- [ ] **Step 4: Verify**

~~~sh
cd /Users/jerry/nhanhmc/coding/Judgify
bash test/e2e/stack.sh up
cd website
JUDGIFY_E2E_MANAGED_WEB_SERVER=0 npm run test:e2e:release
cd ..
bash test/e2e/stack.sh down
~~~

Save report, trace, console logs, and socket evidence to the ledger.

## Task 4: Execute Real Browser Product Flows

**Files:**
- Modify: website/e2e/public.spec.ts
- Modify: website/e2e/member.spec.ts
- Modify: website/e2e/admin.spec.ts
- Modify: website/e2e/theme-responsive.spec.ts
- Modify: website/e2e/support/{environment,quality,session}.ts

**Produces:** evidence-backed release E2E; no direct SQL/Docker fallback may seed a failed lifecycle transition.

- [ ] **Step 1: Cover these real paths**

~~~text
register / OAuth or invitation -> onboarding schema -> session
admin role, policy, user-role with recent re-auth and durable replay
profile attributes -> privacy -> anonymous public profile
problem draft -> publish -> submit -> judgement -> websocket invalidation -> REST refresh
contest register/freeze/submit/standings/rating
material review/publish/archive/rollback
notification inbox/campaign
recovery contact/password reset/revoke
admin audit/operation/observability
~~~

- [ ] **Step 2: Add negative-path checks**

For destructive/permissioned flows, assert the correct 401/403/409 UX and recovery. A lack of console errors is insufficient.

- [ ] **Step 3: Run configured light/dark/system themes and mobile/tablet/desktop widths**

Persist Playwright traces/screenshots/report. A setup endpoint failing must fail the test; fix product code instead of introducing seed fallback.

## Task 5: Provision and Validate Linux Judge Host

**Files:**
- Create: docs/operations/rootless-judge-host.md
- Modify: test/e2e/stack.sh
- Modify: test/chaos/README.md

**Produces:** disposable environment receipt and successful ProbeDockerSecurity without weakening guards.

- [ ] **Step 1: Document and run host acceptance checks**

~~~sh
docker info --format '{{json .SecurityOptions}}' | grep -F 'name=rootless'
test -f /sys/fs/cgroup/cgroup.controllers
test -x /usr/local/bin/judgify-capability-inspector
docker context show
k6 version
~~~

macOS Docker Desktop/OrbStack is explicitly unsupported for this proof.

- [ ] **Step 2: Require the isolated test contract**

~~~text
JUDGIFY_TEST_ENVIRONMENT=judgify-resilience-test
JUDGIFY_LOAD_ACK
JUDGIFY_CHAOS_ACK
JUDGIFY_RESILIENCE_PGSERVICE
JUDGIFY_RESILIENCE_EVIDENCE_DIR (empty absolute directory, mode 0700)
JUDGIFY_CHAOS_VOLUME (contains .judgify-chaos-volume)
JUDGIFY_CHAOS_SANDBOX_CONTAINER
~~~

- [ ] **Step 3: Prove the host fences bite**

Run with missing marker and with a non-rootless Docker context. Both must fail before modifying containers, volume, or DB.

- [ ] **Step 4: Run real judge pipeline**

~~~sh
cd /srv/judgify/Judgify
bash test/e2e/stack.sh up
bash test/e2e/stack.sh judge
~~~

Steps 11–13 must run. Never edit manifest/digest/probe requirements to match a weak host.

## Task 6: Prove Actual Judgement, Forge Recovery, and Resilience

**Files:**
- Modify: test/e2e/stack.sh
- Modify: test/operations/resilience_assertions.sh
- Modify: test/chaos/{driver.sh,common.sh,scenarios.tsv} only for a demonstrated harness defect
- Modify: scripts/verify/resilience.sh
- Test: test/load/{http,submissions,soak}.js

**Produces:** accepted/rejected execution evidence and a complete 23-scenario resilience receipt.

- [ ] **Step 1: Assert real accepted and rejected submissions**

Each must reach terminal state with exactly one judgement, persisted verdict/per-test result, Forge acknowledgement after durable finalization, replayable outbox event, and browser invalidation followed by canonical REST refresh.

- [ ] **Step 2: Prove worker failure recovery**

Kill/restart worker during claimed work. Require lease reaping, exactly one final judgement, no duplicate ledger mutation, drained outbox, and no dead event. Retain query snapshots, logs, and Forge metadata.

- [ ] **Step 3: Run all scenarios**

~~~sh
cd /srv/judgify/Judgify
JUDGIFY_TEST_ENVIRONMENT=judgify-resilience-test \
JUDGIFY_LOAD_ACK=I_UNDERSTAND_THIS_LOAD_TEST \
JUDGIFY_CHAOS_ACK=I_UNDERSTAND_THIS_CHAOS_TEST \
scripts/verify/resilience.sh --run
~~~

All 23 JSONL manifest records must be passed. Stop at a failure, repair and negative-test it, then rerun in a new environment.

- [ ] **Step 4: Run soak after chaos passes**

~~~sh
JUDGIFY_RUN_SOAK=1 scripts/verify/resilience.sh --run
~~~

Record duration, resources, queue depth, reconnects, and errors.

## Task 7: Finish Recovery and Sandbox Reproducibility

**Files:**
- Modify: docker/sandbox/Dockerfile
- Create or modify: docker/sandbox/README.md
- Modify: scripts/backup/postgres.sh
- Modify: scripts/restore/postgres.sh
- Modify: test/operations/{backup_restore_driver,backup_restore_test,run_backup_restore_proof}.sh

- [ ] **Step 1: Repeat recovery proof**

~~~sh
cd /Users/jerry/nhanhmc/coding/Judgify
sh test/operations/run_backup_restore_proof.sh
~~~

It must prove matched client major, mismatched-client rejection before publishing, incompatible restore rejection before fencing target, and zero target runtime grants.

- [ ] **Step 2: Pin sandbox package inputs**

Choose one: immutable Alpine snapshot URL with recorded date/digest, or content-addressed APKs installed with apk add --no-network. Build twice from empty cache and compare package list/image inputs. Pinning package versions against a moving mirror is not enough.

- [ ] **Step 3: Measure RPO/RTO**

Record backup duration, restore duration, size, and source/target invariant result. Compare measured values to RPO <= 5 minutes and RTO <= 60 minutes.

## Task 8: Close Supply Chain and Runtime SQL Permission Gaps

**Files:**
- Modify: api/go.mod and api/go.sum only after go-common release
- Modify: scripts/verify/supply_chain.sh
- Create: test/operations/runtime_sql_permission_proof.sh

- [ ] **Step 1: Publish immutable go-common**

Tag/publish go-common, replace ../../go-common and v0.0.0 in api/go.mod, then run:

~~~sh
cd /Users/jerry/nhanhmc/coding/Judgify
JUDGIFY_VERIFY_MODE=release JUDGIFY_GO_COVERAGE_FLOOR_PERCENT=37.8 \
scripts/verify/go.sh
~~~

If authored coverage moves, revise the floor only with explanation/review.

- [ ] **Step 2: Produce release evidence from clean tree**

~~~sh
JUDGIFY_RELEASE_EVIDENCE_ACK=GENERATE_UNSIGNED_RELEASE_EVIDENCE \
JUDGIFY_RELEASE_EVIDENCE_DIR=/secure/judgify-release-evidence \
JUDGIFY_RELEASE_IMAGES_FILE=/secure/judgify-images.env \
scripts/verify/supply_chain.sh --evidence
~~~

Require SBOM, vulnerability reports, secret scan, image inventory, and SHA256SUMS. Label receipt unsigned unless signing/provenance actually occurred.

- [ ] **Step 3: Execute dynamic SQL with runtime roles**

PREPARE does not check privileges. Test known dynamic SQL families under judgify_api and judgify_worker, verify allowed behavior and forbidden append-only mutation. Store statement family/count without logging solution/token/credential data.

## Task 9: Final Rehearsal and Handoff

**Files:**
- Modify: README.md, CONTRIBUTING.md, deploy and docs/operations runbooks, REMAINING_WORK.md, docs/PRODUCT_COMPLETION_STATUS.md
- Review: every path from git status --short in Judgify and go-common

- [ ] **Step 1: Reconcile docs with executed facts**

Document host requirements, rootless socket, backup-client policy, restore/unfence, sandbox input pinning, delivery provider config, release evidence, and WebSocket invariant. Remove every unproven claim.

- [ ] **Step 2: Classify every dirty path**

~~~sh
cd /Users/jerry/nhanhmc/coding/Judgify
git status --short
git diff --check
rg -n 'TODO|FIXME|XXX|HACK' api website --glob '!**/generate/**' --glob '!**/mocks/**'
~~~

Classify each as implementation, generated output, obsolete legacy, evidence, or accidental artifact. Do not bulk-delete.

- [ ] **Step 3: Run final gates**

~~~sh
cd /Users/jerry/nhanhmc/coding/Judgify
make verify
JUDGIFY_VERIFY_MODE=release JUDGIFY_GO_COVERAGE_FLOOR_PERCENT=37.8 scripts/verify/go.sh
sh scripts/verify/contracts.sh
cd website && npm run verify && npm run build && npm run check:bundle
~~~

Then run the Linux judge/resilience and release-evidence commands above.

## Completion Criterion

Judgify is release-ready only when every row in Ordered Completion Matrix is green with linked evidence, the two status documents no longer contradict each other, and the final tree has no unclassified dirty path. Implemented features alone are not enough.

## External Decisions Still Required

1. Provide or rent a Linux host with rootless dockerd and cgroup v2 delegation.
2. Authorize tagging/publishing go-common when supply-chain evidence is to be closed.
3. Choose immutable sandbox package storage: snapshot mirror or artifact store.

## Non-Goals Until This Plan Is Green

- Multi-node deployment, Redis/Kafka, SSE, another websocket library, or new end-user feature areas.
- Weakening security gates, direct DB seed fallback, or treating macOS as evidence for Linux sandbox isolation.

## Execution Handoff

Plan saved to docs/superpowers/plans/2026-08-22-release-completion-and-proof-plan.md. Execute tasks in order: Tasks 1–4 locally, Tasks 5–6 on Linux rootless Docker, Tasks 7–9 after evidence exists. Do not report production completion before Task 9.

