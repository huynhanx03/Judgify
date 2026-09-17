# Judgify Problem, Submission, and Judge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make problem publication immutable and safe, make submission acknowledgement durable, and produce reproducible verdicts for untrusted code through exact worker capabilities and a fresh rootless sandbox per testcase.

**Architecture:** Problem owns immutable authored/testset/runtime snapshots; Submission owns the aggregate, durable jobs, attempts/results, active judgement, solved-state transitions, and user queries. The judge is a worker/execution adapter that claims PostgreSQL jobs and finalizes through narrow Submission/Problem contracts; it never owns business state or runs inside the API process.

**Tech Stack:** Go 1.26.5, Ent/PostgreSQL, transactional UnitOfWork, PostgreSQL jobs/outbox, Docker rootless mode, cgroup v2, pinned runtime images, C++17/Python3/Java17/Go profiles, trusted host supervisor/checkers, and go-common lifecycle/correlation/logging.

## Global Constraints

- Acknowledgement occurs only after submission, immutable judge specification, durable job, and outbox commit.
- Current-generation processing phase and active contestant judgement/verdict are separate state.
- An infrastructure-failed/cancelled rejudge never erases an older valid active result.
- Workers claim only one exact activated capability tuple they have smoke-tested; no range/list Cartesian advertisement.
- API never imports Docker or accesses its socket.
- Compile occurs in one disposable non-root container; each testcase runs in a new container and cgroup.
- Network is disabled; root filesystem is read-only; capabilities are dropped; no-new-privileges and seccomp are required.
- Hidden input/expected output and trusted checker remain outside contestant namespaces.
- Contestant source/output and hidden tests are never logged.
- Initial runtimes are exactly C++17, Python3, Java17, and Go.
- Go compilation uses a trusted argv profile and never interpolates source/user data into `sh -c`.
- Durable IDs are UUIDs; ordering uses explicit database sequence/version fields, not UUID order.
- No commits; preserve the dirty worktree.

---

## Required Execution Order

The numeric task ID is authoritative: **1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12**. Task sections are grouped in this document for domain and sandbox review; an implementer must jump to and finish the next numeric ID before starting a later one.

## File Structure

```text
api/internal/
├── platform/contentrender/
├── application/workflow/submissionintake/
├── problem/
├── submission/
└── judge/
    ├── capability/
    ├── supervisor/
    ├── executor/
    └── checker/
```

### Task 1: Add the Shared Deterministic Content Renderer

**Files:**
- Create: `api/internal/platform/contentrender/source.go`
- Create: `api/internal/platform/contentrender/artifact.go`
- Create: `api/internal/platform/contentrender/renderer.go`
- Create: `api/internal/platform/contentrender/sanitizer.go`
- Create: `api/internal/platform/contentrender/repository.go`
- Create: `api/internal/platform/contentrender/renderer_test.go`
- Create: `api/internal/platform/contentrender/security_test.go`
- Create: `api/internal/ent/schema/content_render_source.go`
- Create: `api/internal/ent/schema/content_render_artifact.go`

**Interfaces:**

```go
type SourceKind string

const (
    ProblemStatement SourceKind = "problem_statement"
    ContestDescription SourceKind = "contest_description"
    ContestAnnouncement SourceKind = "contest_announcement"
    ContestClarification SourceKind = "contest_clarification"
    MaterialBody SourceKind = "material_body"
    NotificationCampaignBody SourceKind = "notification_campaign_body"
)

type RenderRequest struct {
    SourceID       uuid.UUID
    SourceKind     SourceKind
    SourceChecksum [32]byte
    NormalizedMarkdown []byte
}

type Artifact struct {
    ID                     uuid.UUID
    RendererVersion        string
    SanitizerPolicyVersion string
    SanitizedHTML          string
    ArtifactChecksum       [32]byte
    Status                 string
    Diagnostics            []Diagnostic
}
```

- [x] **Step 1: Write hostile renderer tests**

Use fixtures for raw HTML, script/event attributes, `javascript:`/`data:` URLs, hostile SVG/images, KaTeX trust escapes, malicious code highlighting, excessive nesting/input/output, malformed UTF-8, and nondeterministic rendering. Assert identical normalized bytes produce identical checksum/artifact.

- [x] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/platform/contentrender -count=1`

Expected: FAIL because renderer and schemas do not exist.

- [x] **Step 3: Implement the canonical source/artifact identity**

Schemas enforce:

```text
content_render_sources:
  unique(source_kind, owner_revision_id)
  unique(id, source_kind, source_checksum)
  unique(id, owner_revision_id, source_kind, source_checksum)

content_render_artifacts:
  FK(render_source_id, render_source_kind, source_checksum)
    -> content_render_sources(id, source_kind, source_checksum)
```

Normalize line endings/Unicode according to one versioned policy, disable raw HTML, sanitize allowlisted output, cap diagnostics, and never mutate an existing artifact.

- [x] **Step 4: Verify**

Run:

```bash
cd api
go generate ./internal/ent
go test ./internal/platform/contentrender -count=100
go test ./...
```

Expected: hostile corpus and API suite pass. Do not commit.

**Retained evidence (2026-07-26):**

- Pinned server-side KaTeX `0.16.44` runs in a bounded shared Goja pool with
  authenticated vendored assets, context cancellation, a closed authored-math
  dialect, exact MathML element/attribute resource policy, and an independent
  sanitizer allowlist.
- Renderer hostile corpus, compatibility corpus, deterministic golden artifact,
  container-aware preflight, immutable repository, Ent/schema extras, baseline
  migration, inventory receipt, and append-only contracts are covered by tests.
- `go test ./internal/platform/contentrender -count=100`, renderer/KaTeX race
  tests, `go vet`, full `go test ./...`, offline Ent generated-drift check,
  Atlas migration validation, database-baseline contract tests, and
  `git diff --check` pass.

### Task 2: Introduce Stable Problems and Immutable Revisions/Testsets

**Files:**
- Modify: `api/internal/ent/schema/problem.go`
- Modify: `api/internal/ent/schema/test_case.go`
- Create: `api/internal/ent/schema/problem_revision.go`
- Create: `api/internal/ent/schema/test_asset.go`
- Create: `api/internal/ent/schema/testset_revision.go`
- Create: `api/internal/ent/schema/testcase_group.go`
- Create: `api/internal/ent/schema/testcase_revision.go`
- Create: `api/internal/problem/core/entity/revision.go`
- Create: `api/internal/problem/core/entity/testset.go`
- Modify: `api/internal/problem/ports/problem.go`
- Create: `api/internal/problem/ports/revision.go`
- Create: `api/internal/problem/adapters/driven/db/revision.go`
- Create: `api/internal/problem/core/service/publication.go`
- Create: `api/internal/problem/core/service/publication_test.go`

**Interfaces:**

```go
type PublishCommand struct {
    ProblemID        uuid.UUID
    DraftRevisionID  uuid.UUID
    TestsetRevisionID uuid.UUID
    ExpectedVersion  int64
    Reason           string
}

type JudgeSnapshot struct {
    ProblemID          uuid.UUID
    ProblemRevisionID  uuid.UUID
    TestsetRevisionID  uuid.UUID
    RuntimeProfileID   uuid.UUID
    CheckerProfileID   uuid.UUID
    Limits             Limits
}
```

- [ ] **Step 1: Write immutable lifecycle tests**

Test draft edits create new rows, published rows cannot update/delete, source/artifact FK cannot cross revisions/kinds/checksums, hidden assets never enter public DTOs, publish validates every group/test/runtime/checker/limit, optimistic conflict, archive visibility, and published pointer switch with audit/outbox.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/problem/core/service -run TestPublication -count=1`

Expected: FAIL because revisions/publication service are absent.

- [ ] **Step 3: Implement append-only revisions and content-addressed test assets**

Store encrypted or worker-authorized asset references plus size/checksum/media metadata; do not store hidden content in public entities. Publication locks the stable problem, verifies exact render artifact and immutable testset tuple, then advances active pointers atomically.

- [ ] **Step 4: Generate schema and verify**

Run:

```bash
cd api
go generate ./internal/ent
go test ./internal/problem/... -run TestPublication -count=50
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 7: Replace the Container Pool with a Trusted Rootless Supervisor

**Files:**
- Replace: `api/internal/judge/executor/docker_pool.go`
- Modify: `api/internal/judge/executor/runner.go`
- Create: `api/internal/judge/supervisor/supervisor.go`
- Create: `api/internal/judge/supervisor/docker.go`
- Create: `api/internal/judge/supervisor/profile.go`
- Create: `api/internal/judge/supervisor/result.go`
- Create: `api/internal/judge/supervisor/probe.go`
- Create: `api/internal/judge/supervisor/supervisor_test.go`
- Create: `api/internal/judge/supervisor/docker_integration_test.go`
- Modify: `docker/sandbox/Dockerfile`

**Interfaces:**

```go
type ExecuteRequest struct {
    ImageDigest string
    Argv        []string
    Env         []string
    Stdin       io.Reader
    InputFiles  []InputFile
    Limits      ResourceLimits
    OutputLimit int64
}

type ExecuteResult struct {
    ExitCode       int
    Signal         string
    WallTime       time.Duration
    CPUTime        time.Duration
    PeakMemoryBytes int64
    Stdout         []byte
    Stderr         []byte
    StdoutTruncated bool
    StderrTruncated bool
    OOMKilled      bool
    TimedOut       bool
}

type Supervisor interface {
    Execute(ctx context.Context, request ExecuteRequest) (ExecuteResult, error)
    Probe(ctx context.Context) (capability.CapabilityEvidence, error)
    Shutdown(ctx context.Context) error
}
```

- [ ] **Step 1: Write fake-engine lifecycle tests**

Assert structured argv, no network, non-root UID/GID, read-only root, tmpfs quotas, dropped capabilities, no-new-privileges, seccomp, pids/memory/CPU/file/output limits, no host source/test bind, bounded copy/attach/wait/stats channels, kill-whole-container on timeout/cancel, and destroy in every error path.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/judge/supervisor -count=1`

Expected: FAIL because supervisor is absent.

- [ ] **Step 3: Implement trusted orchestration**

The supervisor validates worker-owned paths/argv/environment and never accepts raw Docker options from jobs. It creates one container, starts bounded readers before execution, samples/reads cgroup v2 accounting, kills on context/resource/output breach, waits, captures structured result, and removes the container/cgroup in a defer. Docker errors are infrastructure errors, not contestant verdicts.

- [ ] **Step 4: Remove reusable contestant containers**

Delete production pooling/reuse behavior. Optional warm preparation may pre-pull images or build immutable worker-owned layers only; it never retains a container that ran contestant code.

- [ ] **Step 5: Verify unit and opt-in integration suites**

Run:

```bash
cd api
go test ./internal/judge/supervisor -count=100
go test -race ./internal/judge/supervisor -count=10
go test -tags=sandbox ./internal/judge/supervisor -run TestDocker -count=1
```

Expected: unit/race pass; sandbox job passes only on a probed rootless cgroup v2 host and otherwise skips with an explicit capability reason, never a false pass. Do not commit.

### Task 8: Stage Hostile Artifacts and Run a Fresh Container per Testcase

**Files:**
- Create: `api/internal/judge/executor/staging.go`
- Create: `api/internal/judge/executor/staging_test.go`
- Create: `api/internal/judge/executor/compiler.go`
- Create: `api/internal/judge/executor/compiler_test.go`
- Create: `api/internal/judge/executor/testcase.go`
- Create: `api/internal/judge/executor/testcase_test.go`
- Modify: `api/internal/judge/executor/runner.go`

**Interfaces:**

```go
type CompiledArtifact struct {
    ManifestChecksum string
    Files            []ArtifactFile
    RuntimeProfile   RuntimeProfile
}

type CaseResult struct {
    TestcaseID uuid.UUID
    Execution  supervisor.ExecuteResult
    Output     []byte
}
```

- [ ] **Step 1: Write hostile archive/staging tests**

Reject absolute/traversal paths, symlink/hardlink/device/FIFO/socket, duplicate normalized names, excessive files/bytes/inodes/depth, sparse expansion, checksum mismatch/collision, partial writes, and filesystem watermark exhaustion. Prove `O_EXCL|O_NOFOLLOW`, 0700 root, 0600 blobs, fsync+rename, reservation release, and no host execution.

- [ ] **Step 2: Write fresh-case tests**

Use a fake supervisor and assert compilation gets one container, every testcase gets a different container/cgroup ID, state/process/files cannot cross cases, expected output is never mounted, and source is available only where the profile needs it.

- [ ] **Step 3: Run focused tests**

Run: `cd api && go test ./internal/judge/executor -run 'Test(Staging|Compile|Testcase)' -count=1`

Expected: FAIL against current runner/pool behavior.

- [ ] **Step 4: Implement staged compile and case execution**

Reserve global/per-attempt bytes/inodes before copy, stream each file to a random exclusive temp while hashing/counting, fsync validated content, then atomically rename. Compile through one disposable supervisor call. For each case, construct a new request/container with only input, compiled artifact, and trusted limits; capture bounded contestant output for host-side checking.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/judge/executor -run 'Test(Staging|Compile|Testcase)' -count=100
go test -race ./internal/judge/executor -count=10
```

Expected: all exit 0 with exact quota release. Do not commit.

### Task 9: Implement Trusted Checkers, Verdict Precedence, and Structured Execution

**Files:**
- Replace: `api/internal/judge/comparator/cf_compare.go`
- Create: `api/internal/judge/checker/checker.go`
- Create: `api/internal/judge/checker/exact.go`
- Create: `api/internal/judge/checker/token.go`
- Create: `api/internal/judge/checker/float.go`
- Create: `api/internal/judge/checker/checker_test.go`
- Create: `api/internal/judge/verdict.go`
- Create: `api/internal/judge/verdict_test.go`
- Modify: `api/internal/judge/worker.go`

**Interfaces:**

```go
type Checker interface {
    Check(ctx context.Context, expected io.Reader, actual io.Reader, policy Policy) (Result, error)
}

type Result struct {
    Accepted   bool
    Code       string
    Diagnostic string
}

func Classify(result supervisor.ExecuteResult, checker Result) Verdict
```

- [ ] **Step 1: Write golden checker and precedence tests**

Cover CRLF/trailing policy, token whitespace, numeric NaN/Inf/absolute+relative tolerance, output caps, malformed expected data as infrastructure error, contestant non-zero exit, signal, timeout, OOM, output limit, checker error, and deterministic precedence.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/judge/checker ./internal/judge -run 'Test(Checker|Verdict)' -count=1`

Expected: FAIL because current comparator/runner can misclassify non-zero exits.

- [ ] **Step 3: Implement trusted host-side checking**

Checker code runs in worker process over bounded readers. Only built-in versioned checker keys can activate. Diagnostics are bounded and safe for admin; ordinary users receive stable verdict metadata without hidden expected values.

- [ ] **Step 4: Implement phase execution**

Worker restores origin CID, enriches bounded IDs, verifies lease, compiles, runs groups/cases with cancellation, applies fail-fast/group scoring policy, and accumulates structured per-test results. Contestant verdicts are terminal; infrastructure failures enter retry policy.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/judge/checker ./internal/judge -count=100
go test -race ./internal/judge -count=10
```

Expected: all exit 0; no non-zero exit is classified Accepted. Do not commit.

### Task 10: Finalize Judgements, Solved Transitions, Rewards, and Outbox Atomically

**Files:**
- Create: `api/internal/submission/core/service/finalization.go`
- Create: `api/internal/submission/core/service/finalization_test.go`
- Create: `api/internal/submission/adapters/driven/db/finalization.go`
- Create: `api/internal/submission/adapters/driven/db/finalization_test.go`
- Create: `api/internal/submission/core/service/solved_state.go`
- Create: `api/internal/submission/core/service/solved_state_test.go`
- Modify: `api/internal/judge/exp_reward.go`
- Modify: `api/internal/judge/worker.go`

**Interfaces:**

```go
type FinalizeCommand struct {
    LeaseToken []byte
    JobID      uuid.UUID
    AttemptID  uuid.UUID
    Result     GenerationResult
    TestResults []TestResult
}

type FinalizeResult struct {
    SubmissionID       uuid.UUID
    AggregateVersion   int64
    ActiveVerdictChanged bool
    SolveTransitionID  *uuid.UUID
}
```

- [ ] **Step 1: Write lease/finalization crash tests**

Prove stale/wrong lease rejects, duplicate same finalization is idempotent, per-test/judgement/job/phase commit together, active result changes only for valid evaluation, failed rejudge preserves active result, earliest accepted solve projection is deterministic, unsolve/re-solve transitions are contiguous, outbox uses aggregate version, and crash before/after commit yields one canonical result.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/submission/core/service -run 'Test(Finalize|Solved)' -count=1`

Expected: FAIL because finalization service is missing.

- [ ] **Step 3: Implement one finalization transaction**

Lock job by ID and compare lease HMAC in constant time. Insert immutable attempt/test/judgement rows, update latest generation outcome, conditionally update active judgement, recompute affected user/problem solve from active Accepted judgements ordered by `(submitted_at, submission_id)`, append versioned transition when changed, complete/dead/retry job, and append browser-safe/domain outbox events.

- [ ] **Step 4: Move rewards behind solved transitions**

Judge no longer directly mutates cultivation EXP. It emits one immutable solve transition/event containing snapshotted difficulty/tag/reward-rule facts. Cultivation consumes idempotently in its plan.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/submission/core/service \
  ./internal/submission/adapters/driven/db -run 'Test(Finalize|Solved)' -count=50
go test -race ./internal/submission/core/service -count=10
```

Expected: all exit 0 and every successful finalization has matching outbox state. Do not commit.

### Task 11: Add Secure Query, Rejudge, Cancellation, and Judge Operations APIs

**Files:**
- Modify: `api/internal/problem/adapters/driver/http/problem.go`
- Modify: `api/internal/submission/adapters/driver/http/submission.go`
- Modify: `api/internal/submission/adapters/driver/http/handler.go`
- Create: `api/internal/submission/core/service/rejudge.go`
- Create: `api/internal/submission/core/service/cancellation.go`
- Create: `api/internal/submission/core/service/operations.go`
- Create: `api/internal/submission/core/service/rejudge_test.go`
- Create: `api/internal/submission/adapters/driver/http/admin.go`
- Create: `api/internal/submission/adapters/driver/http/http_test.go`

**Interfaces:**

```go
type RejudgeCommand struct {
    SubmissionID   uuid.UUID
    ExpectedVersion int64
    Reason         string
    IdempotencyKey uuid.UUID
}

type CancelCommand struct {
    SubmissionID   uuid.UUID
    Generation     int64
    ExpectedVersion int64
    Reason         string
}
```

- [ ] **Step 1: Write authorization/data-exposure tests**

Test public published problem mapping, draft/editor access, own/any source rules, hidden-test non-exposure, list constraints in SQL, rejudge idempotency/new generation, cancel queued/running lease signaling, protected active result, dead-job retry, bounded admin filters, optimistic conflict, audit reason, and error/CID mapping.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/problem/... ./internal/submission/... -run 'Test(Query|Rejudge|Cancel|HTTP)' -count=1
```

Expected: FAIL until secure projections and commands are added.

- [ ] **Step 3: Implement separate response projections**

Create explicit public, owner, author, and judge-admin DTO mappers. Never serialize Ent/domain entities directly. Repositories accept authorization-aware predicates for lists; service verifies object ownership/state for detail.

- [ ] **Step 4: Implement durable commands**

Rejudge creates next generation/spec/job/outbox atomically and keeps prior active result. Cancellation marks desired cancellation under lock, worker observes it via heartbeat/context, and finalization records cancelled generation without active-result loss. Every admin mutation writes audit.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/problem/... ./internal/submission/... -count=50
go test -race ./internal/submission/... -count=10
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 12: Backfill, Switch Worker Roles, and Run Crash/Sandbox/Load Gates

**Files:**
- Create: `api/internal/submission/migration/backfill.go`
- Create: `api/internal/submission/migration/backfill_test.go`
- Create: `api/internal/judge/integration/crash_test.go`
- Create: `api/internal/judge/integration/sandbox_test.go`
- Create: `api/internal/judge/integration/load_test.go`
- Create: `api/cmd/judgify/judge_worker.go`
- Modify: `api/cmd/server/main.go`
- Modify: `api/internal/infrastructure/run.go`
- Modify: `api/internal/infrastructure/router.go`
- Modify: `docs/BACKEND_GUIDE.md`

**Interfaces:**
- Produces separate API/worker roles and verified migration from legacy submissions.

- [ ] **Step 1: Write idempotent legacy backfill tests**

Backfill immutable revision-one problems/testsets/specs, existing submission active judgements, durable terminal/current generation, solved pairs ordered by `(submitted_at, submission_id)`, and one `legacy_baseline` transition per pair. Invalid/missing legacy facts go to quarantine, never guessed state.

- [ ] **Step 2: Run migration tests**

Run: `cd api && go test -tags=integration ./internal/submission/migration -count=1`

Expected: FAIL until backfill is implemented.

- [ ] **Step 3: Split executable roles**

`judgify api` builds routes/outbox/security-email but has no Docker import/access. `judgify judge-worker` initializes capability probe, starts claims disabled, persists exact evidence, enables claims after smoke, runs claim/scheduler/supervisor, drains claims before shutdown. `judgify all` exists only behind development configuration.

- [ ] **Step 4: Inject every crash boundary**

Kill/restart before and after intake commit, claim, lease heartbeat, compile, each case, finalization commit, outbox append, and response. Assert no acknowledged job is lost, no duplicate active judgement/reward, and expired leases recover.

- [ ] **Step 5: Run malicious and load suites**

Exercise fork bombs, memory bombs, infinite output, filesystem probing, network attempts, lingering children, symlink/archive attacks, compiler crashes, and hidden-data probing. Run 500 submissions in a 50/s ten-second burst and a four-hour soak with bounded RSS/goroutines/FDs/queue.

- [ ] **Step 6: Verify the complete plan**

Run:

```bash
cd api
go test ./internal/problem/... ./internal/submission/... ./internal/judge/...
go test -race ./internal/submission/... ./internal/judge/...
go test -tags=integration ./internal/submission/migration ./internal/judge/integration
go test -tags=sandbox ./internal/judge/integration -run TestSandbox
go test ./...
git diff --check
```

Expected: all applicable commands exit 0 with retained capability/crash/load evidence. Do not commit.

## Acceptance Criteria Reference

- [ ] Published problem/testset/render artifacts are immutable and database-bound to exact source identities.
- [ ] Acknowledged submission always has one durable job and immutable judge specification.
- [ ] Worker claim SQL matches one exact passing, fresh, claims-enabled capability tuple.
- [ ] API has no Docker dependency and worker is the only sandbox process.
- [ ] Every testcase receives a fresh non-root container/cgroup.
- [ ] Hostile artifacts and output are bounded before retention.
- [ ] Trusted host checker and hidden expected output never enter contestant namespace.
- [ ] Non-zero exits, signals, timeout, OOM, and output limit receive deterministic verdicts.
- [ ] Failed/cancelled rejudge never erases a prior active contestant result.
- [ ] Solve/reward side effects originate from versioned canonical transitions.
- [ ] Crash/load/malicious suites prove no lost job or unbounded resource growth.
- [ ] No commit was created.

### Task 3: Add Runtime, Checker, and Exact Worker Capability Registries

**Files:**
- Create: `api/internal/ent/schema/runtime_profile.go`
- Create: `api/internal/ent/schema/checker_profile.go`
- Create: `api/internal/ent/schema/judge_worker_instance.go`
- Create: `api/internal/ent/schema/judge_worker_capability.go`
- Create: `api/internal/problem/core/entity/runtime.go`
- Create: `api/internal/problem/core/entity/checker.go`
- Create: `api/internal/problem/core/service/runtime_registry.go`
- Create: `api/internal/problem/core/service/runtime_registry_test.go`
- Create: `api/internal/judge/capability/profile.go`
- Create: `api/internal/judge/capability/probe.go`
- Create: `api/internal/judge/capability/probe_test.go`

**Interfaces:**

```go
type CapabilityTuple struct {
    JudgeSpecProtocol   string
    RuntimeKey          string
    RuntimeProfileVersion string
    ImageDigest         string
    ManifestVersion     string
    ManifestChecksum    string
    SupervisorProtocol  string
    CheckerKey          string
    CheckerVersion      string
    CapabilityProfileHash string
}

type CapabilityEvidence struct {
    Tuple          CapabilityTuple
    BootID         uuid.UUID
    SmokeChecksum  string
    ProbedAt       time.Time
    Passing        bool
}
```

- [ ] **Step 1: Write exact-tuple and activation tests**

Reject floating image tags, shell-interpolated commands, inactive/missing profile components, stale boot/heartbeat, claims-disabled worker, failing probe, separately advertised lists/ranges, and any synthetic Cartesian combination. Prove N/N-1 are two complete independent rows.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/problem/core/service ./internal/judge/capability -run 'Test(Runtime|Capability)' -count=1
```

Expected: FAIL because registries do not exist.

- [ ] **Step 3: Implement four trusted argv profiles**

Create versioned exact compile/run argv templates for C++17, Python3, Java17, and Go. Templates interpolate only validated worker-owned paths and numeric limits through structured arguments. Go build uses a prepared trusted module workspace; it never executes user-derived `sh -c`.

- [ ] **Step 4: Implement production capability probe**

Probe rootless Docker, cgroup v2 controllers/delegation, pinned image digest, seccomp/no-new-privileges, user namespace, read-only root, disabled network, kill/accounting semantics, compiler/runtime manifest, and checker golden smoke. Persist one normalized row per exact passing tuple; worker starts claims-disabled.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go generate ./internal/ent
go test ./internal/problem/core/service ./internal/judge/capability -count=50
go test ./...
```

Expected: all unit tests pass; production activation remains false without real probe evidence. Do not commit.

### Task 4: Expand Submission, Judge Specification, Job, Attempt, and Result State

**Files:**
- Modify: `api/internal/ent/schema/submission.go`
- Create: `api/internal/ent/schema/judge_spec.go`
- Create: `api/internal/ent/schema/judge_job.go`
- Create: `api/internal/ent/schema/judge_attempt.go`
- Create: `api/internal/ent/schema/judgement.go`
- Create: `api/internal/ent/schema/judge_test_result.go`
- Create: `api/internal/ent/schema/user_problem_solve.go`
- Create: `api/internal/ent/schema/user_problem_solve_transition.go`
- Modify: `api/internal/submission/core/entity/submission.go`
- Create: `api/internal/submission/core/entity/job.go`
- Create: `api/internal/submission/core/entity/judgement.go`
- Create: `api/internal/submission/core/entity/solve.go`
- Modify: `api/internal/submission/ports/submission.go`
- Create: `api/internal/submission/ports/job.go`
- Create: `api/internal/submission/ports/finalization.go`
- Create: `api/internal/submission/adapters/driven/db/schema_test.go`
- Create: `api/internal/ent/migrate/migrations/20260718000200_problem_submission_judge_expand.sql`
- Modify: `api/internal/ent/migrate/migrations/atlas.sum`

**Interfaces:**

```go
type Phase string
type Verdict string

type SubmissionState struct {
    ID                    uuid.UUID
    LatestGeneration      int64
    LatestPhase           Phase
    LatestOutcome         string
    ActiveGeneration      *int64
    ActiveJudgementID     *uuid.UUID
    ActiveVerdict         *Verdict
    AggregateVersion      int64
}
```

- [ ] **Step 1: Write schema/state-machine tests**

Prove one judge spec/job/judgement per `(submission_id,generation)`, valid closed transitions, active judgement updates only for valid contestant evaluation, failed/cancelled rejudge preserves prior active result, per-test results bind attempt/spec/group/test, lease tokens are unique secret hashes, and solved transition versions are contiguous.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/submission/... -run 'Test(State|Schema)' -count=1`

Expected: FAIL because durable model is absent.

- [ ] **Step 3: Implement additive model and transition functions**

Pure functions return typed next state:

```go
func StartGeneration(current SubmissionState, generation int64) (SubmissionState, error)
func FinishGeneration(current SubmissionState, result GenerationResult) (SubmissionState, error)
func ApplyJudgement(current SubmissionState, judgement Judgement) (SubmissionState, error)
```

Repositories use optimistic aggregate version and database constraints; handlers never set state fields directly.

- [ ] **Step 4: Generate and verify**

Run:

```bash
cd api
go generate ./internal/ent
go test ./internal/submission/... -run 'Test(State|Schema)' -count=100
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 5: Make Submission Intake One Durable Transaction

**Files:**
- Create: `api/internal/application/workflow/submissionintake/workflow.go`
- Create: `api/internal/application/workflow/submissionintake/workflow_test.go`
- Modify: `api/internal/problem/ports/problem.go`
- Modify: `api/internal/submission/ports/submission.go`
- Modify: `api/internal/contest/ports/registration.go`
- Modify: `api/internal/identity/ports/user.go`
- Modify: `api/internal/submission/core/service/submission.go`
- Modify: `api/internal/submission/adapters/driver/http/submission.go`

**Interfaces:**

```go
type Command struct {
    UserID         uuid.UUID
    ProblemID      uuid.UUID
    ContestID      *uuid.UUID
    Language       string
    Source         []byte
    IdempotencyKey uuid.UUID
    CorrelationID  string
}

type Result struct {
    SubmissionID    uuid.UUID
    Generation      int64
    AggregateVersion int64
}
```

- [ ] **Step 1: Write transactional and race tests**

Cover exact idempotent replay, changed payload conflict, source/language/size limits, active user lock, problem published snapshot, contest registration/lifecycle/capacity/end/unregister/ban races, database contest time/sequence, exact judge tuple, job/outbox creation, and rollback of every participant.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/application/workflow/submissionintake -count=1`

Expected: FAIL because workflow does not exist.

- [ ] **Step 3: Implement declared lock order**

Inside `UnitOfWork`: Identity active-user lock → per-user Submission admission → Contest lifecycle/registration/sequence/snapshot when applicable → Problem immutable tuple verification → Submission aggregate/spec/job/outbox insert. No network, Docker, WebSocket, or unbounded work occurs in the callback.

- [ ] **Step 4: Switch HTTP handler**

Handler validates bounded DTO/idempotency UUID, calls workflow, and responds only after commit. Remove local queue publication from the acknowledgement path; local wakeup after commit is best-effort and disposable.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/application/workflow/submissionintake -count=50
go test -race ./internal/application/workflow/submissionintake -count=10
go test ./...
```

Expected: no acknowledged result without one durable job. Do not commit.

### Task 6: Implement Exact Claim, Lease, Heartbeat, and Fair Admission

**Files:**
- Create: `api/internal/submission/adapters/driven/db/job.go`
- Create: `api/internal/submission/adapters/driven/db/job_test.go`
- Create: `api/internal/submission/core/service/job.go`
- Create: `api/internal/submission/core/service/job_test.go`
- Modify: `api/internal/judge/fair_scheduler.go`
- Create: `api/internal/judge/fair_scheduler_test.go`
- Modify: `api/internal/judge/worker.go`

**Interfaces:**

```go
type ClaimRequest struct {
    WorkerID uuid.UUID
    BootID   uuid.UUID
    Capacity int
}

type Lease struct {
    JobID       uuid.UUID
    AttemptID   uuid.UUID
    Token       []byte
    ExpiresAt   time.Time
    JudgeSpec   JudgeSpec
}

func (r *JobRepository) Claim(ctx context.Context, request ClaimRequest) ([]Lease, error)
func (r *JobRepository) Heartbeat(ctx context.Context, lease Lease, extendTo time.Time) error
```

- [ ] **Step 1: Write PostgreSQL claim and scheduler tests**

Prove `FOR UPDATE SKIP LOCKED`, exact capability SQL join, fresh current boot, claims-enabled passing row, unique lease token, stale-token rejection, expiry reclaim, max attempts/backoff/dead job, bounded per-user/global queue, weighted fair scheduling, no starvation, cancellation, disk/depth/age admission thresholds, and no commit-before-work semantics.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/submission/... ./internal/judge -run 'Test(Claim|Lease|Scheduler)' -count=1
```

Expected: FAIL against current local queue/worker behavior.

- [ ] **Step 3: Implement database claim query**

Join each job’s immutable tuple to one exactly equal `judge_worker_capabilities` row and its fresh current boot. Claim updates job/attempt/lease in one transaction and returns bounded rows. Arrays/ranges/lists are never expanded into compatibility.

- [ ] **Step 4: Implement local fair accelerator**

Scheduler contains only already durably claimed leases, owns all goroutines, has bounded queues, and returns overload rather than blocking forever. Losing scheduler memory is safe because lease expiry returns unfinished work to PostgreSQL.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/submission/... ./internal/judge \
  -run 'Test(Claim|Lease|Scheduler)' -count=50
go test -race ./internal/judge -run TestScheduler -count=20
```

Expected: all exit 0. Do not commit.

## Final Acceptance Gate

After Task 12, re-evaluate every checkbox in **Acceptance Criteria Reference**. The plan exits only when each durability, sandbox, verdict, migration, crash, and load item has direct evidence.
