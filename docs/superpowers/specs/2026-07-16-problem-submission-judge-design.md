# Judgify Problem, Submission, and Judge Design

**Date:** 2026-07-16

**Status:** Draft for written review

**Deployment model:** Single host, separate API and judge-worker processes

**Primary stack:** Go, Gin, Ent, PostgreSQL, Docker sandbox, Next.js

## 1. Purpose

This document defines the production problem-authoring, submission-intake, and judging architecture for Judgify. It replaces mutable test data, an in-memory-authoritative queue, non-reproducible judging, reusable root containers, and multi-step verdict side effects with immutable revisions, PostgreSQL durable jobs, leased workers, hardened execution, and transactional finalization.

The design is governed by these invariants:

1. A submission acknowledged by the API always has durable work in PostgreSQL.
2. A submission is judged against the exact problem, testset, checker, runtime, and limits selected when its judge generation was created.
3. Contestant output, process behavior, archive metadata, or filesystem artifacts can never forge supervisor control state or escape its testcase boundary.
4. A worker cannot finalize a job after losing its lease.
5. One judge generation has at most one canonical terminal judgement, while retries and rejudges retain their history.
6. Contest, cultivation, statistics, notifications, and WebSocket delivery consume committed events; the judge worker does not update those domains directly.

## 2. Resolved decisions

- `Problem` owns stable identity and exact revision/render-source/artifact publication pointers; immutable authored revisions own source, constraints, reward facts, checker settings, and testset references, while the shared platform/authored-content modules own immutable render sources and sanitized artifacts.
- Test assets and immutable testset revisions are stored durably in PostgreSQL for the single-host release and are included in database backup/restore.
- Submission is the bounded context that owns submissions, judge specifications, durable jobs, attempts, per-test results, judgement history, and terminal verdict state.
- `internal/judge` is an execution adapter: claimant, fair scheduler, trusted host supervisor/checker, and Docker sandbox. It does not own canonical submission state.
- API and judge worker are separate process roles built from the same Judgify codebase. Only the judge-worker process can access Docker.
- PostgreSQL `judge_jobs` is the only authoritative queue. Forge is not used to acknowledge or persist judge work.
- Claims use `FOR UPDATE SKIP LOCKED`, a unique lease token, expiry, and heartbeat.
- Submission creation is idempotent and writes submission, immutable judge specification, job, and outbox in one Ent transaction.
- Judge results are finalized in one transaction guarded by lease token, job state, generation, and optimistic submission version.
- Infrastructure failures retry; contestant verdicts do not.
- Compilation and every testcase run use separate disposable, non-root containers. A trusted host supervisor and host checker obtain control state from Docker/cgroup APIs rather than contestant-controlled output.
- Scheduling is bounded, weighted by lane, aged to prevent starvation, and limited per user.
- Source, hidden tests, compile logs, and per-test data use explicit object-level authorization and redaction.
- PostgreSQL is durable truth. In-process channels, local caches, and prepared sandbox capacity are accelerators only.

## 3. Current problems being corrected

The existing implementation has production-critical failure modes:

- The API creates a submission and then publishes to Forge. A crash between those operations leaves an acknowledged submission pending forever.
- The consumer commits a Forge batch after handing jobs to an asynchronous pool, before judging completes. Worker death loses those jobs.
- Forge has no lease/heartbeat or per-record terminal acknowledgement suitable for judge work.
- The worker loads the current mutable problem and testcases, so an old submission is silently reinterpreted after admin edits.
- Database update errors during state changes and finalization are ignored.
- Contest, cultivation, problem statistics, and submission state are updated through separate calls with no idempotency boundary.
- A shell wrapper can return success even when contestant code exited non-zero, producing false Accepted results.
- Empty testsets can become Accepted.
- Standard output/error buffers are unbounded, and the metadata marker can collide with contestant output.
- Containers are reused, run as root by default, and cleanup does not reliably remove dotfiles, child processes, or all cross-submission state.
- Runtime/compiler packages and sandbox images are not pinned by digest, so results are not reproducible.
- Hidden tests and submission source lack complete ownership/redaction guarantees.

## 4. Context and process boundaries

### 4.1 Problem bounded context

Problem owns:

- stable problem identity, slug, authorship, and lifecycle;
- immutable problem revisions;
- immutable testset revisions, test groups, and test assets;
- checker and runtime constraints;
- publish validation and publication pointers;
- admin revision history and audit events.

### 4.2 Submission bounded context

Submission owns:

- intake validation and idempotency;
- source ownership and visibility;
- immutable judge specification per generation;
- submission state/version;
- `judge_jobs`, leases, retry/dead state;
- judge attempts and per-test results;
- immutable terminal judgement history;
- finalization and `submission.judged.v1` outbox event.

This ownership permits submission, job, results, judgement, and outbox to commit through one Ent/PostgreSQL transaction.

### 4.3 Judge execution adapter

`internal/judge` owns only execution concerns:

- claim eligible durable jobs;
- maintain lease heartbeat;
- schedule within bounded local capacity;
- materialize an immutable `JudgePlan` through a narrow reader;
- create and destroy sandbox containers;
- compile and execute source;
- run output checkers;
- classify contestant versus infrastructure outcomes;
- submit a structured finalization command.

It does not import contest or cultivation repositories and does not directly mutate problem counters, rankings, rewards, notifications, or standings.

Cross-context behavior uses narrow transaction-bound application contracts, not foreign repositories or Ent entities. `api/internal/application/workflow/submissionintake` owns the caller transaction. Identity contributes the account-eligibility participant `AssertSubmissionEligible`; Problem resolves/validates an immutable published judge snapshot; Contest contributes `ReserveContestSubmission`, which locks and validates contest state/registration/problem membership and atomically reserves ordering metadata; and the judge Runtime Registry contributes `AssertCapabilityActivated`. Every participant receives the same opaque transaction handle, may access only its own tables, and returns value objects; no contract exposes an Ent client, repository, or entity.

For a non-contest submission, Submission selects the published problem revision inside the caller transaction. For a contest submission, the Contest reservation returns the already snapshotted `contest_problem_id`, problem/testset revision IDs, lane, database `submitted_at`, elapsed milliseconds, and contest submission sequence. Submission cannot replace those values with current Problem state or client input.

### 4.4 Process roles

```text
judgify api
  HTTP + WebSocket + application commands/queries
  submission transaction + outbox dispatcher
  no Docker socket and no judge claims

judgify judge-worker
  PostgreSQL claim/lease/heartbeat
  fair scheduler + trusted supervisor/checker + Docker adapter
  finalization transaction + outbox
  dedicated OS identity with sandbox privileges

judgify migrate
  schema migration and checksum verification
  no API or judging workload
```

The first deployment runs API and worker on one machine as separate supervised processes. This provides privilege and failure isolation without introducing microservices, Redis, or an external broker.

## 5. Problem and testset revision model

### 5.1 Stable problem aggregate

`problems` contains stable identity and lifecycle fields:

| Column | Meaning |
|---|---|
| `id` | Stable public problem UUID |
| `slug` | Unique public slug |
| `author_id` | Owning author |
| `lifecycle` | `draft`, `published`, or `archived` |
| `draft_revision_id` | Latest saved draft revision |
| `draft_render_source_id` | Shared render source bound to the draft revision/artifact |
| `draft_render_source_kind` / `draft_render_source_checksum` | Exact source discriminator/checksum paired with the draft |
| `draft_render_artifact_id` | Sanitized artifact paired with the draft revision |
| `published_revision_id` | Raw authored revision used for ordinary submissions |
| `published_render_source_id` | Shared render source bound to the published revision/artifact |
| `published_render_source_kind` / `published_render_source_checksum` | Exact source discriminator/checksum paired with publication |
| `published_render_artifact_id` | Exact approved sanitized artifact served with the published revision |
| `submission_count` / `accepted_count` | Rebuildable projections, not judge transaction authority |
| timestamps | Aggregate lifecycle timestamps |

Pointer integrity is database-enforced through the shared render-source registry, never an application-polymorphic revision FK. `problem_revisions` exposes unique `(id, render_source_id, render_source_kind, render_source_checksum)` and `content_render_artifacts` exposes unique `(id, render_source_id, render_source_kind, source_checksum)`. `problems` therefore composite-references the raw revision with `(revision_id, render_source_id, render_source_kind, render_source_checksum)` and the artifact with `(artifact_id, render_source_id, render_source_kind, render_source_checksum)` for both draft and published selections. Each selection's five pointer fields are all null or all non-null, and its kind is checked as `problem_statement`. The two foreign keys force every revision/source/artifact/checksum selection to share the exact registered source. Draft and published selections may differ. Archiving blocks new ordinary submissions but retains every authored revision, render source, render artifact, and old submission history.

### 5.2 Immutable problem revisions

`problem_revisions` contains:

| Column | Meaning |
|---|---|
| `id` | Revision UUID |
| `problem_id` | Stable problem aggregate |
| `revision_number` | Monotonic per-problem integer |
| `title` / `statement_markdown` | Immutable authored source |
| `render_source_id` / `render_source_kind` / `render_source_checksum` | Shared render source; kind is `problem_statement`, checksum is exact normalized Markdown |
| `difficulty_id` / `difficulty_revision` | Difficulty classification snapshot at this revision |
| `base_exp` | Immutable non-negative base reward input for this revision |
| `tag_mapping_id` / `tag_mapping_revision` / `tag_snapshot` | Immutable ordered tag IDs/revisions used by projections |
| `element_mapping_id` / `element_mapping_revision` / `element_snapshot` | Optional immutable element IDs/revisions/weights |
| `reward_snapshot_checksum` | Canonical checksum over immutable reward facts and mappings |
| `cpu_time_limit_ms` | Positive CPU-time limit per test |
| `wall_time_limit_ms` | Positive monotonic wall-time limit per test, always at least the CPU limit |
| `memory_limit_kb` | Positive memory limit |
| `output_limit_bytes` | Maximum stdout/stderr captured per test |
| `process_limit` | Maximum process count |
| `checker_key` / `checker_version` | Versioned checker contract |
| `checker_config` | Validated bounded JSON for that checker |
| `testset_revision_id` | Immutable testset used by this problem revision |
| `allowed_runtime_keys` | Validated set of supported runtime profiles |
| `created_by` / `created_at` | Revision provenance |
| `content_checksum` | SHA-256 over normalized revision content |

The reward snapshot is bounded canonical JSON whose semantic form is `{difficulty:{id,revision}, base_exp, tags:{mapping_id,revision,items:[{tag_id,tag_revision}]}, elements:{mapping_id,revision,items:[{element_id,element_revision,weight_bps}]}|null, checksum}`. Items use deterministic ID order, no duplicates, and bounded integer weights. Rows are insert-only after creation. `content_checksum` covers normalized authored source, limits, the full immutable reward snapshot, checker/runtime configuration, and testset revision. Saving an authored edit creates a new raw problem revision; rendering never mutates it and a renderer upgrade never fabricates a new authored revision.

### 5.3 Immutable authored-content render artifacts

The shared platform contract first registers immutable raw render sources in `platform.content_render_sources`:

| Column | Meaning |
|---|---|
| `id` | Render-source UUID |
| `owner_revision_id` | Exact immutable raw revision that owns this source |
| `source_kind` | Checked discriminator; `problem_statement` for a problem statement |
| `source_checksum` | SHA-256 over the exact normalized authored bytes |
| `created_by` / `created_at` | Nullable system/operator actor and registration time |

The registry has primary key `id`, unique `(source_kind, owner_revision_id)`, unique `(id, source_kind, source_checksum)`, and unique `(id, owner_revision_id, source_kind, source_checksum)`. A raw problem revision stores `render_source_id`, `render_source_kind CHECK = 'problem_statement'`, and `render_source_checksum` computed from its exact normalized Markdown; composite FK `(render_source_id, id, render_source_kind, render_source_checksum)` targets the registry's `(id, owner_revision_id, source_kind, source_checksum)` tuple. `content_checksum` remains the different checksum over the whole problem revision. This is the same parent-registry contract used by every authored-content module: the database binds a typed source to its exact owner revision without a fake foreign key from the shared table to one polymorphic child table.

The shared authored-content module then owns `content_render_artifacts`:

| Column | Meaning |
|---|---|
| `id` | Artifact UUID |
| `render_source_id` | FK to the registered immutable raw source |
| `render_source_kind` / `source_checksum` | Exact typed source identity copied from the registry |
| `renderer_version` / `sanitizer_policy_version` | Exact Markdown/math/sanitizer pipeline |
| `sanitized_html` | Immutable safe artifact; nullable only for a rejected render |
| `artifact_checksum` | SHA-256 over render-source ID/checksum, pipeline versions, and sanitized HTML |
| `status` / `diagnostics` | `ready` or `rejected` plus bounded safe diagnostics |
| `created_by` / `created_at` | Render provenance |

Artifacts are insert-only, composite-reference `(render_source_id, render_source_kind, source_checksum)` to the registry, expose unique `(id, render_source_id, render_source_kind, source_checksum)`, and are unique by source plus pipeline versions. A `ready` row has non-null sanitized HTML/checksum; a `rejected` row has no servable HTML and bounded diagnostics. The server-owned renderer parses Markdown/math, then a reviewed allowlist sanitizer removes or rejects raw HTML, scripts, event attributes, unsafe URLs/embeds, dangerous CSS, and ambiguous encodings. Raw Markdown is returned only through authorized authoring endpoints; public readers receive `sanitized_html` from the exact ready artifact pointer and never render authored source in the browser.

Saving an edit registers the normalized source, creates the raw revision bound to that source and its draft artifact, then atomically switches the draft revision/source/kind/checksum/artifact selection only after all checksums and composite foreign keys validate. Publishing atomically switches the corresponding five published pointer fields as one aggregate change. A renderer security upgrade renders the same registered source into a new immutable artifact and performs an audited atomic artifact-pointer switch while retaining the revision/source/kind/checksum selection; it neither mutates the old artifact nor creates a fake authored revision. Judge specifications continue to pin the raw problem revision, so a presentation-security rerender cannot silently alter constraints, tests, checker, or reward facts.

### 5.4 Test assets

`test_assets` stores input and expected-output bodies:

| Column | Meaning |
|---|---|
| `id` | Asset UUID |
| `sha256` | Unique checksum of uncompressed bytes |
| `encoding` | `identity` or `zstd` |
| `content` | Bounded byte content |
| `uncompressed_size` | Validation and allocation guard |
| `created_at` | Provenance timestamp |

PostgreSQL is selected instead of local filesystem paths so a database backup is a complete durable backup for judge inputs. Upload and decompression limits prevent oversized allocation. Assets are deduplicated by checksum and remain immutable.

### 5.5 Testset revisions and tests

`testset_revisions` contains revision identity, problem ownership, monotonic revision number, checksum, test count, total uncompressed bytes, creator, and timestamp.

`test_cases` becomes revision-owned and immutable:

| Column | Meaning |
|---|---|
| `id` | Stable test UUID within revision history |
| `testset_revision_id` | Parent immutable testset |
| `kind` | `sample` or `hidden` |
| `group_index` / `order_index` | Deterministic execution order |
| `input_asset_id` / `expected_asset_id` | Immutable assets |
| `weight` | Score weight, `1` for all-or-nothing launch behavior |
| `stop_on_failure` | Group execution behavior |

Sample tests are exposed through a dedicated public mapper. Hidden test input, expected output, asset identifiers, and checksums never appear in public or ordinary submission APIs.

### 5.6 Publish validation

A problem revision can be published only when:

- title, statement, slug, difficulty, and limits pass bounded validation;
- at least one hidden test exists;
- test/group ordering is unique and contiguous after normalization;
- every referenced asset exists, checksum-validates, and satisfies size limits;
- every allowed runtime profile exists and is enabled;
- checker key/version and config validate;
- base EXP, difficulty, ordered tag mapping, optional element weights, immutable revision references, and reward checksum validate within bounded policy;
- time, memory, output, process, test count, and aggregate asset limits are within production policy;
- all samples are renderable and all hidden tests remain non-public;
- the candidate raw revision and selected artifact bind the same registered render source, whose source checksum matches normalized Markdown; the approved renderer/sanitizer and artifact checksum verify;
- the candidate checksum can be reproduced from canonical normalized data.

An invalid or empty testset is a publication error. The worker also treats an empty resolved hidden testset as `system_error`; it can never produce Accepted.

## 6. Runtime and checker registry

Runtime profiles are code/config-owned, versioned entries exposed read-only to authoring:

```text
runtime_key
language
display_name
compiler_version
runtime_bootstrap_version
sandbox_image_digest
compile_command_id
run_command_id
default compile timeout
compiled-artifact manifest
enabled flag
```

Compile and run commands are predefined argument-vector identifiers, not arbitrary shell strings stored by admins. Each profile declares the exact relative regular-file artifact manifest and executable allowlist that a successful compile may export. Every judge specification snapshots the selected runtime key, versions, image digest, commands, artifact manifest, and relevant limits.

Launch enables exactly the four languages already exposed by the product—C++17, Python 3, Java 17, and Go—only after each concrete compiler/runtime version and image digest passes the full golden/sandbox corpus. They use separate immutable runtime-profile keys even if release engineering temporarily builds them into one pinned image. C++/Java/Go compilation and every run are direct trusted argv profiles. Go uses a reviewed image-owned compile entrypoint or direct `go build` argv with a supervisor-supplied offline environment; it never executes the current request-interpolated `sh -c`, creates a module dynamically, or enables dependency download. Python runs in an isolated deterministic profile. A display language is not authority: submission intake accepts only an enabled runtime key whose exact profile becomes part of the activated capability tuple.

The initial checker registry supports:

- exact normalized output;
- token comparison;
- floating-point comparison with validated absolute/relative tolerance.

Checker v1 semantics are fixed and golden-tested:

- `exact.v1` converts CRLF pairs to LF, removes only terminal LF bytes, appends exactly one terminal LF, and then compares bytes. It does not trim spaces/tabs, normalize bare CR, change Unicode, or collapse internal blank lines.
- `token.v1` splits on one or more ASCII whitespace bytes (`SP`, `TAB`, `LF`, `CR`, `FF`, `VT`), ignores leading/trailing delimiters, requires equal token count, and compares every token byte-for-byte. Unicode whitespace is ordinary token data.
- `float.v1` uses the same ASCII tokenization and requires equal token count. With `digits` meaning one or more ASCII digits, the finite decimal grammar is `[+-]?(digits(.digits?)?|.digits)([eE][+-]?digits)?`. When the expected token matches it, the actual token must also parse to a finite value and passes when `abs(actual-expected) <= abs_tol` **or** `abs(actual-expected) <= rel_tol * max(abs(expected), abs(actual))`. `abs_tol` and `rel_tol` are finite configuration values in the closed interval `[0,1]`. Case-insensitive `[+-]?(nan|inf|infinity)`, grammar-matching overflow, and any parsed non-finite value fail expected-output publication and fail contestant comparison. Every expected token that does not match the decimal grammar or a reserved non-finite spelling is explicitly nonnumeric and requires a byte-for-byte actual token match.

Checkers are trusted, versioned implementations. A change to normalization, grammar, tolerance formula/bounds, tokenization, or nonnumeric policy requires a new checker version; historical specifications retain v1. User-provided checker code and interactive judging are excluded from this release boundary.

## 7. Submission and judgement data model

### 7.1 Submissions

`submissions` contains:

| Column | Meaning |
|---|---|
| `id` | Submission UUID |
| `user_id` | Owner |
| `problem_id` | Stable problem |
| `contest_id` | Optional contest context |
| `contest_problem_id` | Snapshotted contest problem membership, nullable outside contests |
| `contest_scoring_policy_version` | Snapshotted Contest policy version, nullable outside contests |
| `submitted_at` | PostgreSQL timestamp captured by the intake transaction; never client-supplied |
| `contest_elapsed_ms` | Server-derived elapsed time from contest start |
| `contest_submission_sequence` | Monotonic per-contest intake sequence, nullable outside contests |
| `source_code` | Contestant source, access-controlled |
| `source_sha256` | Source integrity and idempotency input |
| `language` / `runtime_key` | Selected runtime |
| `version` | Optimistic state-transition version |
| `latest_generation` | Newest processing/rejudge generation |
| `latest_generation_phase` | `queued`, `claimed`, `compiling`, `running`, `finalizing`, `terminal`, or `cancelled` |
| `latest_generation_outcome` | Terminal contestant verdict, `system_error`, or `cancelled`; null before terminal/cancelled |
| `active_judgement_id` | Applied contestant judgement; remains on the prior generation during an unsuccessful rejudge |
| `active_judgement_generation` | Generation of the applied contestant judgement, nullable with the pointer |
| `active_verdict` | Denormalized verdict of the applied contestant judgement, nullable before the first successful contestant evaluation |
| `idempotency_key` / `request_hash` | Intake replay protection |
| `origin_cid` | Correlation ID of the accepted command |
| latest queued/started/completed timestamps | Newest generation lifecycle |

Unique `(user_id, idempotency_key)` prevents duplicate submission creation. Keys are 1–128 printable characters and retained for the lifetime of the submission. Reusing a key with a different request hash returns `409 idempotency_conflict`; reusing it with the same hash returns the canonical existing submission.

### 7.2 Immutable judge specifications

`judge_specs` has one immutable row per `(submission_id, generation)`:

| Column | Meaning |
|---|---|
| `id` | Judge specification UUID |
| `submission_id` / `generation` | Owner and rejudge generation |
| `problem_revision_id` | Exact statement/constraint revision |
| `testset_revision_id` | Exact immutable tests |
| `runtime_key` / `runtime_profile_version` | Selected immutable runtime profile |
| `sandbox_image_digest` | Exact sandbox image |
| `compile_command_id` / `run_command_id` | Exact trusted argv profiles |
| `artifact_manifest_version` / `artifact_manifest_checksum` | Exact allowed compile outputs/executable paths |
| `supervisor_protocol_version` | Required host-supervisor control contract |
| `checker_key` / `checker_version` / `checker_config` | Exact comparison behavior |
| `capability_profile_hash` | Exact release-owned sandbox/runtime/checker capability profile required for activation and claim |
| `reward_snapshot` / `reward_snapshot_checksum` | Immutable Problem-owned reward facts carried to terminal transitions |
| limits | Per-test CPU and wall time, memory, output, process, compile, and total-attempt wall limits |
| `source_sha256` | Source integrity reference |
| `judge_spec_protocol` / `spec_checksum` | Serialization protocol and canonical specification checksum |

Large test contents are not copied into job JSON; the immutable specification references immutable test assets.

The worker obtains a transport-neutral plan:

```go
type JudgePlan struct {
    JobID             uuid.UUID
    SubmissionID      uuid.UUID
    Generation        int
    UserID            uuid.UUID
    ProblemRevisionID uuid.UUID
    TestsetRevisionID uuid.UUID
    Language           string
    Runtime            RuntimeSnapshot
    Checker            CheckerSnapshot
    RewardFacts        RewardFactsSnapshot
    Limits             Limits
    SourceCode         string
    SourceSHA256       string
    Tests              []TestSpec
    OriginCID          string
}

type JudgeSpecReader interface {
    ReadJudgePlan(ctx context.Context, jobID uuid.UUID, leaseToken string) (JudgePlan, error)
}
```

The reader verifies the active lease and checks all referenced checksums. It does not expose a mutable Problem repository to the worker.

### 7.3 Judgements, attempts, and test results

`submission_judgements` is immutable terminal history for each generation:

- submission and generation;
- terminal verdict;
- passed/total counts;
- maximum time and memory;
- compile diagnostic summary;
- judge-spec checksum;
- immutable reward snapshot and checksum copied from the judge specification;
- runtime/compiler/bootstrap/supervisor/image provenance;
- winning attempt ID, nullable only for terminal `system_error` generations with no successful execution attempt;
- completed timestamp and CID.

Unique `(submission_id, generation)` permits only one canonical judgement per generation.

`judge_attempts` records each infrastructure attempt:

- job, submission, generation, attempt number;
- lease owner/token hash, worker ID, origin CID;
- state and infrastructure failure classification;
- started/heartbeat/finished timestamps;
- runtime and sandbox provenance;
- bounded diagnostic metadata.

`judge_test_results` stores ordered result metadata for the terminal attempt:

- attempt and test-case ID/order/group;
- verdict;
- time, memory, exit code, terminating signal;
- bounded stdout/stderr byte counts and truncated safe diagnostic for samples;
- checker result metadata;
- execution start/end times.

Hidden input, hidden expected output, and full hidden stdout are never persisted in response-oriented result rows. Administrative diagnostics expose bounded infrastructure metadata, not hidden expected values.

### 7.4 Canonical solved-state projection and transitions

Submission owns `user_problem_solves`, keyed uniquely by `(user_id, problem_id)`. The row has a stable ID, `status` (`solved` or `unsolved`), the earliest currently active accepted submission/judgement/generation when solved, its immutable problem/reward snapshot, `solved_at`, a monotonic `version`, and—while solved—the `active_reward_transition_id` plus `reward_provenance` (`live` or `legacy_reconstructed`). It is the canonical current solved-state projection; immutable reward history remains in Cultivation's ledger.

The first successful contestant evaluation for a user/problem creates, if absent, a stable unsolved solve row at version `0`; a non-AC result produces no transition. Finalization acquires a transaction-scoped lock keyed by user/problem and recomputes the projection from every submission's `active_judgement_id`, ordered by database `submitted_at` and submission UUID. If any projection field changes, the same transaction updates the projection and inserts exactly one immutable `user_problem_solve_transitions` row:

```text
id UUIDv7 primary key
solve_id UUID
user_id UUID
problem_id UUID
from_version bigint
to_version bigint
from_status unsolved|solved
to_status unsolved|solved
from_submission_id UUID nullable
to_submission_id UUID nullable
from_judgement_id UUID nullable
to_judgement_id UUID nullable
from_generation bigint nullable
to_generation bigint nullable
from_problem_revision_id UUID nullable
to_problem_revision_id UUID nullable
from_reward_facts JSONB nullable
to_reward_facts JSONB nullable
from_reward_grant_source_id UUID nullable
to_reward_grant_source_id UUID nullable
from_reward_provenance live|legacy_reconstructed nullable
to_reward_provenance live|legacy_reconstructed nullable
from_solved_at timestamptz nullable
to_solved_at timestamptz nullable
provenance live|legacy_baseline
occurred_at timestamptz
correlation_id string
unique (solve_id, to_version)
check (to_version = from_version + 1)
```

Foreign keys bind all transition pointers to their immutable owners. Check constraints require all `to_*` solved pointers/timestamp/reward facts/reward-grant fields to be non-null exactly when `to_status = solved`, null when unsolved, and apply the same rule to `from_*`; the version-0 unsolved origin is represented by null pointers. Reward JSON must match the canonical bounded schema and checksum stored on the referenced judgement/problem revision. For `unsolved → solved`, `to_reward_grant_source_id` equals this transition ID; for `solved → solved`, the prior grant source/provenance is copied; for `solved → unsolved`, only the `from` side carries it.

To linearize event time against Cultivation's effective-dated trait/formula/profile revisions, every live solve-transition transaction acquires the shared transaction advisory lock `pg_advisory_xact_lock_shared(REWARD_TIMELINE_LOCK_KEY)` before time capture, then obtains one PostgreSQL `clock_timestamp()` for `occurred_at`; it never accepts worker/client time. A Cultivation revision publisher acquires the exclusive transaction form of the same key before assigning `effective_at`. The global order is Submission job → submission → user/problem solve lock → shared reward-timeline lock → transition/outbox; a Cultivation publisher uses exclusive reward-timeline lock → Cultivation rows. No transaction may lock a Cultivation effective-dated row and then request the timeline lock, and Submission never mutates those rows. Thus concurrent finalizations remain mutually shared while publication creates a precise before/after boundary without deadlock.

The transition row and its `problem.solve_status_changed.v1` outbox row commit with the submission judgement. Pointer or immutable reward-fact movement from one accepted judgement to another is a transition even when status remains `solved`; it advances the version but does not create a second first-solve reward.

The event's exact domain payload is:

```json
{
  "transition_id": "uuid",
  "solve_id": "uuid",
  "user_id": "uuid",
  "problem_id": "uuid",
  "from": {
    "version": 3,
    "status": "solved",
    "submission_id": "uuid",
    "judgement_id": "uuid",
    "generation": 1,
    "problem_revision_id": "uuid",
    "reward_grant_source_id": "uuid",
    "reward_provenance": "live",
    "reward_facts": {
      "difficulty": {"id": "uuid", "revision": 2},
      "base_exp": 100,
      "tag_mapping": {
        "id": "uuid",
        "revision": 5,
        "items": [{"tag_id": "uuid", "tag_revision": 3}]
      },
      "element_mapping": {
        "id": "uuid",
        "revision": 4,
        "items": [{"element_id": "uuid", "element_revision": 2, "weight_bps": 10000}]
      },
      "checksum": "sha256-hex"
    },
    "solved_at": "RFC3339"
  },
  "to": {
    "version": 4,
    "status": "unsolved",
    "submission_id": null,
    "judgement_id": null,
    "generation": null,
    "problem_revision_id": null,
    "reward_grant_source_id": null,
    "reward_provenance": null,
    "reward_facts": null,
    "solved_at": null
  },
  "provenance": "live",
  "occurred_at": "RFC3339",
  "correlation_id": "string"
}
```

`reward_facts`, `reward_grant_source_id`, and `reward_provenance` are present together only for a solved side. `element_mapping` is `null` when the authored revision has none; tag items may be empty but the mapping identity/revision and checksum remain explicit. Event serialization uses canonical key/value types, bounded arrays, lowercase hexadecimal SHA-256, and RFC3339 UTC timestamps; any schema change requires a new event version.

The outbox aggregate is `user_problem_solve`, aggregate ID is `solve_id`, and aggregate version is `to.version`. Consumers do not assume dispatcher ordering. Each side-effecting consumer owns a durable cursor keyed by `(consumer_name, solve_id)` and applies only the next contiguous version while holding that cursor row lock. Duplicate/older versions are no-ops. A gap invokes Submission's bounded `SolveTransitionReader` port to read immutable transitions after the cursor and apply them in order; no consumer imports Submission repositories or Ent entities. If retained transitions and the canonical projection cannot reconcile, the consumer records a visible poisoned event and stops rather than guessing.

Cultivation consumes only the event's immutable Problem-owned reward facts; it never refetches current difficulty, EXP, tags, or element mappings. `unsolved → solved` creates a grant whose ledger `source_id` is `to.reward_grant_source_id` (the transition ID). `solved → unsolved` creates one compensation whose own source is the reversal transition ID and whose `reverses_source_id` is `from.reward_grant_source_id`, whether the grant provenance is `live` or `legacy_reconstructed`. `solved → solved` updates immutable taxonomy/profile projections but carries the existing grant source and creates no second grant. A later re-solve creates a new transition/grant. Cultivation selects its own trait/formula inputs only from explicitly effective-dated immutable revisions at `event.occurred_at`, so later formula/profile edits cannot reinterpret the event.

## 8. State machines

### 8.1 Latest-generation phase and active judgement

```text
queued → claimed → compiling
compiling → running → finalizing → terminal
compiling → finalizing  (compile error skips running)

queued | claimed | compiling | running | finalizing → cancelled
claimed | compiling | running | finalizing → queued  (infrastructure retry)
```

`latest_generation_phase` describes work only. On `terminal`, `latest_generation_outcome` is exactly one of `accepted`, `wrong_answer`, `compile_error`, `runtime_error`, `time_limit_exceeded`, `memory_limit_exceeded`, `process_limit_exceeded`, `output_limit_exceeded`, or `system_error`. On `cancelled`, the outcome is `cancelled`. An infrastructure retry moves the phase back to `queued`, clears its outcome, and preserves failed attempts.

Starting a rejudge increments `latest_generation`, resets its phase/outcome, and does not change the active judgement fields. A generation replaces `active_judgement_id`, `active_judgement_generation`, and `active_verdict` only after a successful contestant evaluation, including a contestant-caused non-AC verdict. A cancelled or terminal infrastructure `system_error` generation never becomes active and never changes solved or contest projections. For an initial system error, active judgement/verdict remain null while the latest outcome visibly reports `system_error`.

API snapshots return latest generation/phase/outcome and active judgement/generation/verdict as separate fields. A compatibility `status` may be derived at the response edge, but no domain consumer, query, event, or database invariant uses that ambiguous field. Terminal transitions require optimistic aggregate version, latest generation, and active job lease. A stale worker cannot overwrite a newer rejudge or cancellation.

### 8.2 Job state

```text
ready → leased → done
leased → retry_wait → ready
ready | leased | retry_wait → cancelled
leased | retry_wait → dead
```

`judge_jobs` contains:

| Column | Meaning |
|---|---|
| `id` | Job UUID |
| `submission_id` / `generation` / `judge_spec_id` | Canonical work identity |
| `lane` | `contest`, `regular`, or `maintenance` |
| `base_priority` | Bounded priority within lane |
| `state` | Durable job state |
| `available_at` | Retry/scheduling eligibility |
| `attempts` / `max_attempts` | Infrastructure retry budget; default maximum 5 |
| `lease_owner` / `lease_token_hash` / `lease_until` | Exclusive claim |
| `heartbeat_at` | Worker liveness |
| `last_error_code` / `last_error_summary` | Bounded infrastructure diagnosis |
| timestamps | Created, started, completed, dead, cancelled |
| `origin_cid` | Correlation continuity |

Unique `(submission_id, generation)` prevents duplicate jobs for a generation.

### 8.3 Attempt state

```text
created → preparing → compiling
compiling → running → finalizing → succeeded
compiling → finalizing → succeeded  (compile error)
created/preparing/compiling/running/finalizing
  → infrastructure_failed | lease_lost | cancelled
```

Contestant outcomes are data inside a successful execution attempt and become a terminal judgement. `infrastructure_failed` is never mapped to Wrong Answer or Runtime Error.

## 9. Submission intake transaction

`POST /submissions` requires an authenticated active principal and an `Idempotency-Key`.

Source shape, language syntax selection, object authorization, and request-hash construction may be checked before opening a transaction. The canonical request hash covers user, problem, contest/contest-problem, runtime/language, normalized source hash, and command version. Every mutable fact—idempotency, quota, Identity eligibility, publication pointer, contest lifecycle, registration, contest problem membership, timestamp, and ordering—is rechecked or reserved inside one caller-owned transaction.

The transaction uses the global lock order command key → Identity user → Submission admission → Contest (when present) → Problem → Runtime Registry activation → Submission aggregate/job/outbox. Every workflow that touches more than one of these rows uses the same order; it never calls back into an earlier participant while holding a later lock.

1. Acquire a transaction-scoped PostgreSQL advisory lock derived from `(user_id, idempotency_key)`, then find the unique submission command. A hash collision only serializes unrelated commands. Same request hash returns the existing submission without allocating quota or contest order; a different hash returns `409 idempotency_conflict`.
2. Call Identity's `AssertSubmissionEligible` transaction participant, lock the authoritative user row, and revalidate active/not-banned status.
3. Lock Submission's per-user admission counter and enforce pending/running quota.
4. For an ordinary submission, read/lock the current published, non-archived Problem revision through Problem's transaction participant, validate the requested runtime, and only then capture PostgreSQL `clock_timestamp()` as `submitted_at`.
5. For a contest submission, call Contest's transaction-bound `ReserveContestSubmission` participant. It locks the contest lifecycle/version, registration/ban state, selected `contest_problems` row, and contest counter in that order; only after those potentially blocking locks does it capture PostgreSQL `clock_timestamp()`, verify the official window against that value, and increment the transactional counter with `UPDATE ... RETURNING`.
6. Contest returns an immutable reservation containing contest/problem IDs, exact problem/testset revision IDs, scoring-policy version, lane, database `submitted_at`, elapsed milliseconds, and sequence. Problem's participant verifies the returned immutable revision/testset/runtime tuple inside the same transaction; Submission cannot substitute a current pointer or client revision.
7. Call `AssertCapabilityActivated`, take a key-share lock on the exact protocol/runtime-profile/image/artifact-manifest/supervisor/checker/capability-profile tuple including every version and checksum, and enforce the bounded queue-age/depth/disk admission snapshot. Retirement takes a conflicting lock and rechecks referenced jobs, so it cannot race an acknowledged unsupported spec into existence.
8. Create `submissions` with latest generation `1`, phase `queued`, null latest outcome, and the reservation metadata.
9. Create the immutable `judge_specs`, `judge_jobs` row in `ready`, and `submission.queued.v1` outbox event.
10. Commit, then return `202 Accepted`.

`contest_submission_counters` is keyed by contest ID and updated transactionally rather than through a non-transactional PostgreSQL sequence, so rollback consumes no order. `submissions` has a partial unique constraint on `(contest_id, contest_submission_sequence)` when contest ID is non-null. A sequence attached to a committed submission is immutable and never reused, including after cancellation; retained history may therefore contain intentional gaps but no ambiguous order. Non-contest submissions still use database `submitted_at` but have null contest sequence/elapsed fields.

The API response contains submission ID, aggregate version, latest generation/phase/outcome, active judgement/generation/verdict fields, database `submitted_at`, contest ordering metadata when present, and CID. It does not claim a worker has started. Client timestamps, elapsed values, aliases, problem revisions, and sequence values are ignored.

No Forge publish participates in correctness. An optional local wake-up signal may reduce claim latency after commit, but losing the signal changes only latency because the worker polls PostgreSQL.

## 10. Claim, lease, and heartbeat

Each worker has an instance ID and a new cryptographically random boot ID on every process start. `judge_worker_instances` contains only process/boot identity, build and lifecycle state, `claims_enabled`, free/total capacity, probe summary, start time, and the fresh `last_heartbeat_at`. Heartbeats use a short TTL. Draining, a failed probe, or repeated control-plane failure closes the worker-local claim gate first and then records `claims_enabled = false`.

The worker advertises normalized `judge_worker_capabilities` rows linked to its exact `(instance_id, boot_id)`. One row represents one complete claimable tuple:

```text
(judge_spec_protocol,
 runtime_key,
 runtime_profile_version,
 sandbox_image_digest,
 artifact_manifest_version,
 artifact_manifest_checksum,
 supervisor_protocol_version,
 checker_key,
 checker_version,
 capability_profile_hash)
```

Each row also carries bounded probe/smoke status and evidence checksum. It is usable only when its owning instance heartbeat is fresh, that exact boot is current, claims are enabled, and the row's probe/smoke state is passing. Arrays, independent min/max ranges, and unrelated lists are not compatibility evidence because their Cartesian product could advertise a tuple the worker never tested.

A worker claims only when that liveness/capacity gate passes and a normalized capability row exactly equals every corresponding immutable judge-spec field; it does not lease a batch that will wait behind occupied sandboxes. The claim query joins jobs, specifications, the current boot row, and the exact capability row in SQL. An unsupported job remains `ready`, increments a bounded compatibility metric/alert, and is never claimed, downgraded, or converted to a contestant/system verdict by that worker.

The runtime registry persists `activated_judge_capabilities` for the same complete tuple plus release ID, activation status/time, actor/reason, and smoke-evidence checksum. Activating a tuple is an explicit audited deployment command that requires at least one fresh claims-enabled worker advertising that exact passing row and a successful golden smoke judgement created from the exact tuple; only then may the API emit it. Retirement is explicit, takes the conflicting activation lock, and is forbidden while a non-terminal job references the tuple. A rolling release makes the new worker advertise independently passing rows for both the new and immediately previous complete tuples, activates the new tuple, switches API emission, drains old jobs, and only then retires the previous tuple.

Submission intake checks the durable activation record, not a live heartbeat on every request. A transient worker restart/outage marks judging degraded and alerts, but the API continues acknowledging activated tuples into the durable PostgreSQL queue until bounded queue-depth, oldest-age, or host-disk admission thresholds are crossed. Intake rejects before creating work only when the target tuple was never activated, was retired, or a backlog safety threshold is exceeded. This preserves queue availability without ever emitting an unsupported version.

Within a short PostgreSQL transaction, the claimant:

1. verifies its current boot heartbeat and exact normalized capability row, then selects only exactly compatible eligible jobs using lane fairness and `FOR UPDATE SKIP LOCKED`;
2. verifies per-user and global running limits;
3. changes selected jobs to `leased`;
4. increments attempts;
5. sets a cryptographically random lease token hash, owner, heartbeat, and lease expiry;
6. inserts the attempt row;
7. commits and returns the raw lease token only to the in-process execution task.

Default lease duration is 30 seconds and job heartbeat interval is 5 seconds, both validated configuration values. Worker-capability heartbeat TTL is at most three heartbeat intervals. Job heartbeat updates require job ID, lease owner, token hash, state `leased`, and an unexpired/latest generation. Three consecutive job heartbeat failures cancel local execution; finalization is forbidden unless the lease is still valid.

A reaper moves expired leased jobs to `retry_wait` or `dead` according to the retry budget. Recovery is safe after worker/process/host restart because leases are database state.

## 11. Fair scheduling and bounded capacity

The scheduler uses three configurable weighted lanes with production defaults:

- contest: 60%;
- regular: 35%;
- maintenance/rejudge: 5%.

Weighted deficit round-robin prevents one active lane from consuming every slot. Unused lane capacity is borrowed by other lanes. An age boost increases effective priority so old jobs cannot starve. Within a lane, order is deterministic by effective priority, creation time, then job UUID.

Per-user running concurrency defaults to one ordinary job. Contest policy may allow a separately configured limit, but a single user cannot occupy all workers. Administrative bulk rejudge stays in the maintenance lane and is globally rate-limited.

The local handoff queue is bounded to at most the number of free sandboxes plus one small dispatch buffer. Database queue depth is the intended burst buffer. At the target burst of 50 submissions per second, the API persists work quickly while queue-depth and oldest-job-age telemetry expose capacity pressure.

## 12. Retry and dead-job policy

Retryable infrastructure failures include:

- temporary Docker daemon or image availability failure;
- compile or testcase-run container creation/removal failure;
- worker process death or lease expiry;
- transient PostgreSQL/finalization conflict;
- Docker control-channel, archive transport/staging I/O, cgroup-accounting, or host-supervisor failure;
- host resource pressure explicitly classified as transient.

Non-retryable contestant outcomes include:

- compile error;
- wrong answer;
- runtime error or signal caused by contestant code;
- time, memory, output, or process limit exceeded.

Invalid problem/testset/runtime references or a reproducible hostile/manifest-invalid compile artifact are deterministic system errors. They move the job to `dead`, create a canonical `system_error` judgement with a nullable winning-attempt ID, transition the submission, and create audit/outbox events rather than consuming all retry attempts. Transient archive I/O or host-capacity failure remains retryable.

Retry backoff uses bounded exponential delay with full jitter. After the default five infrastructure attempts, the job becomes `dead`, the latest generation receives a canonical `system_error` judgement, and an administrator can inspect and explicitly rejudge it through an idempotent command with a reason. Recovery creates a new generation and never mutates or reopens the terminal system-error generation.

## 13. Trusted host supervisor and sandbox

### 13.1 Trust boundary and production capability probe

`internal/judge/sandbox` contains a trusted Go `SandboxSupervisor` that runs in the dedicated judge-worker host process. It uses the Docker Engine API with typed requests and predefined argument vectors; it never invokes a shell, interpolates contestant data into commands, or treats a container file/stdout marker as control state. Exit state comes from Docker `Wait`/`Inspect`; CPU, memory, OOM, and PID state comes from the exact container's cgroup v2 files; wall deadlines use the host monotonic clock.

The production worker requires Linux, unified cgroup v2 with the `cpu`, `memory`, and `pids` controllers, and a rootless Docker daemon owned by the restricted worker identity with cgroup delegation. It has access only to that rootless daemon's local control socket; a rootful host Docker socket is not accepted by the readiness probe. At startup the worker creates a disposable probe container and verifies image-digest resolution, network disablement, non-root identity, read-only rootfs, seccomp/no-new-privileges, memory/OOM reporting, PID enforcement, CPU accounting, kill/wait, archive copy, and removal. If any required capability or accounting source is missing or inconsistent, the worker is unready and claims no jobs. Docker Desktop may support development tests but cannot satisfy the production readiness claim.

The supervisor owns lifecycle cancellation and cleanup. Every container is labeled with non-secret job, attempt, worker-boot, and boundary identifiers. A startup janitor removes only Judgify-labeled containers whose recorded worker boot is no longer live; it never performs broad name/prefix deletion. Control-channel, capability, cleanup, or accounting uncertainty is an infrastructure failure, never a contestant verdict.

### 13.2 Mandatory container profile

Compile, testcase-run, and future custom-checker containers are distinct disposable boundaries created from images pinned by digest. Every boundary receives its own cgroup and fresh namespaces and is removed before the next boundary of that kind. All use:

- a dedicated non-root UID/GID with no setuid/setgid binaries required by the runtime;
- no network, host networking, DNS dependency, or exposed ports;
- a read-only root filesystem plus only explicitly named, size-bounded tmpfs workspaces;
- all Linux capabilities dropped and `no-new-privileges` enabled;
- a reviewed seccomp profile and the host's mandatory-access-control profile where supported;
- bounded memory with swap disabled, CPU quota/accounting, PID count, file size, open files, and tmpfs bytes/inodes;
- no bind mounts, volumes, Docker socket, devices, privileged mode, or host PID/IPC/user/UTS namespaces;
- a minimal deterministic environment allowlist, fixed locale/timezone, explicit working directory, and predefined argv command;
- bounded attach streams and immutable non-secret lifecycle labels.

The Docker control socket is available only to the trusted host supervisor and is never mounted or proxied into a container. Source, test input, compiled artifacts, and checker data cross boundaries only through bounded supervisor-owned streams or Docker archive APIs. Hidden expected output is never copied into a contestant compile or testcase-run container.

### 13.3 Disposable compilation and hostile artifact validation

Each attempt creates one compile container with compile-specific CPU, wall, memory, PID, output, file, and tmpfs limits. The supervisor creates a trusted archive containing the source at fixed runtime-profile paths and copies it into tmpfs through the Docker archive API; no host directory is mounted. The container executes the snapshotted compile argv directly. The supervisor streams bounded compiler stdout/stderr, applies a host monotonic deadline, reads Docker/cgroup termination state, and classifies a trustworthy non-zero compiler exit as `compile_error`; daemon, accounting, image, or control failures remain infrastructure errors.

On success, the supervisor retrieves only the runtime profile's declared artifact root through the Docker archive API. Compiler output is hostile. A streaming archive validator spools file bodies into worker-owned content-addressed blobs rather than extracting archive paths and rejects the entire attempt on any of the following:

- an absolute, empty, non-canonical, NUL-containing, or `..` traversal path;
- duplicate/case-colliding paths or a path outside the exact runtime artifact manifest;
- symlink, hardlink, device, socket, FIFO, sparse/special file, or any non-regular entry;
- excess entry count, header/name size, per-file size, or total expanded bytes, plus unsupported PAX/GNU extension metadata;
- an undeclared executable bit, executable outside the allowlist, missing required executable, extra file, or manifest type/mode mismatch.

The content-addressed staging store lives under a dedicated worker-owned `0700` root on a `noexec,nodev,nosuid` filesystem; blob files are `0600` and are never executed on the host. Before reading an archive, the store atomically reserves the attempt's declared maximum bytes/inodes against both per-attempt quotas and global byte/inode/disk-watermark limits. It spools each regular file to a random `O_CREAT|O_EXCL|O_NOFOLLOW` temporary name while hashing and counting, fsyncs validated content, and atomically renames temp to the hash path only after the whole manifest succeeds. When a hash path already exists, byte length and contents are compared with the candidate before reuse; a mismatch is a fail-closed collision/corruption incident, not deduplication.

Each published blob reference is recorded in an atomic attempt manifest with job/attempt ID, worker boot ID, lease owner, and lease-token hash—never the raw token. Reference counts are released only after every dependent container is removed and the attempt is terminal or its lease is proven invalid. Startup recovery first reconciles PostgreSQL attempt/lease state; it removes incomplete temp files and boot/attempt orphans only after proving no live attempt can reference them, then rebuilds refcounts before accepting work. Disk-full/inode exhaustion rolls back reservations and becomes bounded infrastructure pressure; high/critical watermarks disable claims/intake through the existing admission signals. No janitor guesses liveness from file age alone.

UID/GID, timestamps, extended attributes, ownership, permissions beyond the allowlist, and every compiler-produced archive header are discarded. The supervisor constructs a new canonical manifest from trusted profile paths, byte lengths, SHA-256 hashes, and approved executable flags, then builds a fresh inbound archive itself for testcase containers. It never executes an archive-selected path, trusts a contestant-produced manifest/control JSON, or extracts compiler output with a general-purpose tar command. The compile container is force-removed before any testcase starts; its writable state and cgroup are never reused.

### 13.4 One fresh container per testcase

Every testcase gets a new run container and cgroup. The supervisor copies the already validated canonical artifact archive to fixed paths and supplies only that testcase's bounded input through attached stdin or a trusted fixed-path archive. It does not copy expected output, another testcase, compiler workspace, prior stdout, or prior writable state. The snapshotted run argv names only an executable/path declared by the trusted runtime profile.

The supervisor concurrently drains stdout and stderr through independently bounded readers. Crossing the configured aggregate/output-stream bound immediately kills the container and records `output_limit_exceeded`; slow or non-reading clients cannot create worker backpressure because these streams never pass through the API process. A host monotonic timer enforces wall time.

Immediately after start, the supervisor records the testcase cgroup's `cpu.stat usage_usec` baseline and runs an active CPU watchdog until Docker wait completes. The watchdog polls at a configured bounded interval substantially below the minimum supported CPU limit, subtracts the baseline, and kills the container as soon as total consumed CPU reaches the specification limit. Docker CPU quota limits instantaneous core share only; it is never treated as a total-CPU timer. Final `cpu.stat`, `memory.peak`, `memory.events`, and PID events are read before removal and combined with Docker `Wait`/`Inspect` exit/signal/OOM state. CPU-limit crossing, wall timeout, OOM, PID-limit events, and output kill therefore come from trusted host observations, never contestant output. Watchdog/read failure is infrastructure failure, and measured CPU at or above the limit wins over a racing zero exit.

Unreadable, missing, reused, or contradictory Docker/cgroup data is an infrastructure failure. After output capture and accounting, the supervisor kills any remaining container processes, waits for termination, records bounded results, and removes the container before checking or starting another testcase. This fresh cgroup/tmpfs/process namespace per test is the isolation mechanism; process groups or directory cleanup inside a reused attempt container are not accepted substitutes.

### 13.5 Checker trust and data isolation

The built-in exact, token, and floating-point checkers are small, versioned, fuzz-tested trusted functions in the host worker. They receive bounded captured output and bounded expected bytes only after the contestant run container has been destroyed. They return typed results without logging input, expected output, or contestant output.

User-authored checkers are outside this release. If an administrator-authored or other custom checker is enabled later, it must never execute in the host worker or a contestant container. Each invocation uses a separate disposable checker container/namespace under the mandatory profile and checker-specific limits. The supervisor copies only the bounded testcase input, hidden expected output, and bounded captured contestant output at fixed paths; it never copies contestant source, executable, compiled artifact, process state, or another testcase. The checker has no network or host bind and cannot communicate with a contestant namespace.

A custom checker returns only a strictly bounded, versioned `{verdict, score}` result through the trusted Docker attach/control path; free-form checker diagnostics are neither authoritative nor persisted. Timeout, crash, non-zero exit, malformed/oversized result, cleanup failure, or missing accounting is an infrastructure error and never Wrong Answer or Accepted. Hidden expected bytes may exist in this isolated checker namespace for the invocation, but never enter the contestant compile/run namespace or logs.

### 13.6 Resource accounting and verdict precedence

Wall and CPU limits are separate fields in problem revisions, judge specifications, supervisor commands, and results. Compile limits, per-test limits, and total-attempt wall limits are independent; compile boundaries use the same baseline-delta CPU watchdog when a total CPU limit applies. The worker never buffers unbounded source, input, output, stderr, archives, diagnostics, or result sets; every boundary is checked before allocation and while streaming.

For one testcase, classification uses only trusted host observations in this order:

1. Docker/cgroup/control/checker infrastructure failure;
2. memory, process, or output limit;
3. CPU or wall time limit;
4. non-zero exit or terminating signal as runtime error;
5. trusted built-in checker result as accepted or wrong answer.

A zero exit code alone is never sufficient for acceptance. A generation is Accepted only when compilation completed under trustworthy accounting and every required hidden testcase ran in its own boundary and passed its checker. Any ambiguity fails closed as infrastructure error.

## 14. Execution and finalization flow

1. Claim the durable job and create an attempt.
2. Restore `origin_cid` into context; enrich logs with job, submission, generation, and attempt IDs.
3. Read and checksum-verify the immutable judge plan under the lease.
4. Guardedly transition `latest_generation_phase` through `claimed` and `compiling`; phase changes never modify the active judgement fields.
5. Compile in a disposable compile container, validate/canonicalize its artifact manifest, and remove the compile container. A trustworthy compiler rejection produces `compile_error`; control ambiguity produces an infrastructure failure.
6. When compilation succeeds, transition to `running`, execute tests in deterministic group/order using one fresh run container per testcase, remove each container, then run the selected trusted built-in checker.
7. Transition to `finalizing` and classify the attempt as a successful contestant evaluation—including a contestant-caused non-AC outcome—or an infrastructure failure.
8. For a successful evaluation, call the Submission finalizer with the lease token, expected submission version/latest generation, result summary, supervisor/artifact/image provenance, and bounded per-test results.
9. On every return path, remove the current container, atomically release the attempt's staging-manifest references so only zero-reference lease-safe blobs are reclaimed, and stop work immediately if cancellation or lease loss invalidates the generation.

Each coarse progress transaction verifies job, lease, latest generation, and optimistic submission version; updates only the latest phase/timestamps/version; and commits a realtime-safe `submission.progressed.v1` outbox event with that same aggregate version. It never changes `latest_generation_outcome` before terminal state or the active judgement fields. Progress is emitted only for phase changes, not heartbeats or testcases, so reconnecting clients recover from the REST snapshot without an unbounded event stream.

The successful finalization transaction:

1. locks and verifies the job is `leased`, token/owner match, lease is current, and its generation equals the submission's latest generation;
2. verifies the submission's expected version, `latest_generation_phase = finalizing`, and null latest outcome;
3. bulk-inserts the bounded per-test results;
4. marks the attempt `succeeded`;
5. inserts the immutable `submission_judgements` row including the specification's verified reward snapshot/checksum;
6. sets latest phase `terminal` and latest outcome to the contestant verdict, replaces the active judgement/generation/verdict with this judgement, and updates summary/version/completion time;
7. locks and recomputes the canonical `user_problem_solves` projection when the active judgement changes;
8. marks the job `done` and clears its lease secret;
9. appends `submission.judged.v1` and, when the solve projection changed, the immutable solve transition plus `problem.solve_status_changed.v1` to the outbox;
10. commits.

The unique judgement key and guarded state/version make duplicate finalization an idempotent read of the canonical result or a typed stale-lease conflict; it cannot duplicate a terminal side effect.

Infrastructure-failure handling is also transactional. A retryable failure verifies the active lease, marks the attempt `infrastructure_failed`, moves the job to `retry_wait`, returns the latest phase to `queued`, clears latest outcome, records a bounded error code, and leaves every active judgement field unchanged. The reaper later moves the job to `ready` when its backoff expires.

A deterministic fatal error or exhausted retry budget marks the attempt failed when present, inserts the generation's canonical `system_error` judgement, sets latest phase/outcome to `terminal`/`system_error`, moves the job to `dead`, and appends `submission.judged.v1` with `became_active = false`. It never replaces the active judgement, recomputes solved state, changes contest/statistics contributions, or emits a solve transition. Thus an unsuccessful rejudge remains visibly failed as the latest generation while the last successful contestant evaluation remains active.

## 15. Downstream side effects

`submission.judged.v1` contains only committed, consumer-safe data:

- event ID, event version, and submission aggregate ID/version;
- submission, user, problem, generation, judgement, problem-revision, and testset-revision IDs;
- latest generation phase/outcome and terminal score summary;
- `became_active`, previous active judgement/generation/verdict, and resulting active judgement/generation/verdict;
- contest ID, contest-problem ID, scoring-policy version, database `submitted_at`, contest elapsed milliseconds, and contest submission sequence when applicable;
- occurred time and causation/correlation metadata.

It does not contain source code, test assets, hidden output, or raw compiler diagnostics.

Idempotent inbox consumers perform:

- contest standing/penalty update;
- cultivation reward/compensation ledger update from `problem.solve_status_changed.v1`;
- problem/user statistics and solved-state read projections;
- persisted notification projection;
- realtime-safe WebSocket delivery.

Each consumer commits its side effect and inbox marker in one transaction. Duplicate and out-of-order delivery is expected and tested.

Contest and statistics consumers maintain one versioned active-judgement contribution per submission. `became_active = false` events, including `system_error`, update operational history only and never change a standing, solved count, or verdict contribution. A successful rejudge atomically replaces the prior contribution; stale aggregate versions are ignored. Counters are rebuildable from canonical active judgements and are never increment-only authority.

Contest ordering never depends on outbox delivery, judgement completion, worker speed, or client time. For one contestant/problem, contributions are recomputed in ascending `(contest_submission_sequence, submission_id)` order; official penalty time uses the snapshotted `contest_elapsed_ms`, and all ranking/tie behavior uses `contest_scoring_policy_version`. The sequence is the contest-wide intake total order, while the UUID is only a defensive deterministic tie-breaker. Rejudge can change a contribution but cannot change `submitted_at`, elapsed time, or sequence. A consumer that observes a version gap reloads the affected contestant/problem contributions from the canonical query instead of applying an increment against uncertain history.

## 16. Rejudge, cancellation, and administration

### 16.1 Rejudge

An administrator selects an explicit problem revision, testset revision, runtime profile, and scope. The server returns a dry-run count and impact summary before execution.

Executing rejudge:

- requires permission and a reason;
- creates a new immutable judge specification and increments generation;
- creates a maintenance-lane job;
- sets latest phase/outcome to `queued`/null while retaining the active judgement/generation/verdict;
- retains every previous judgement and attempt;
- preserves the prior active judgement until the new generation completes a successful contestant evaluation;
- emits audit/outbox records;
- is idempotent by command key;
- never edits a historical judgement in place.

Bulk rejudge creates bounded batches and lets normal maintenance-lane capacity control throughput.

### 16.2 Cancellation

Users may cancel only their own queued ordinary submission when product rules allow. Administrators may cancel queued or leased work with a reason.

Cancellation updates job/submission state transactionally, sets latest phase/outcome to `cancelled`/`cancelled`, preserves the prior active judgement fields, and invalidates the lease/generation. A running worker receives cancellation through heartbeat failure/state check and terminates the current boundary. A stale worker cannot finalize afterward.

Cancellation is a command terminal state, not a judge verdict. It emits a cancellation event and has no `submission_judgements` row for that cancelled generation.

### 16.3 Operations console

Admin operations expose:

- queue depth and oldest age by lane;
- leased, retrying, dead, and cancelled jobs;
- worker/heartbeat/sandbox capacity;
- attempts and bounded infrastructure diagnostics;
- explicit retry, requeue, cancel, and rejudge commands;
- audit trail and CID links.

No admin endpoint returns hidden inputs/expected outputs or unrestricted source by default. Elevated source/test access is separately authorized and audited.

## 17. API contracts at design level

### Public and authenticated problem APIs

- problem list/search/filter/sort returns only published, non-archived revisions and bounded summaries;
- problem detail returns render-artifact ID, immutable `sanitized_html`, render checksum/pipeline version, snapshotted difficulty/tags, limits, supported runtimes, and sample tests; public mappers never render Markdown on demand or return authored raw HTML;
- author/admin revision APIs create immutable revisions, clone an existing revision, accept `statement_markdown`, render/sanitize a preview with the pinned pipeline, upload test assets, validate, publish, archive, and inspect audit history;
- hidden test APIs require explicit elevated authorization and never share public response mappers.

### Submission APIs

- create submission with `Idempotency-Key` returns `202` with latest generation/phase/outcome, active judgement/generation/verdict, database timestamp, and immutable contest order when applicable;
- get submission returns those latest-work and active-result fields separately in a projection filtered by owner/admin capability; a UI compatibility label is derived only in the mapper;
- list my submissions and problem submissions use keyset pagination and allowlisted filters/sorts;
- result detail exposes safe per-test status/timing metadata, samples where allowed, and bounded compile/runtime diagnostics;
- source is returned only to owner or `submission.read_any` authority;
- cancel is a guarded idempotent command;
- WebSocket events provide low-latency progress, while REST remains the canonical snapshot.

### Judge/admin APIs

- list/inspect jobs and attempts;
- retry/requeue/cancel with expected state, idempotency key, and reason;
- preview/execute rejudge;
- queue, throughput, dead-job, sandbox, and worker health projections.

All API entity identifiers are UUID strings. Error responses use stable codes and CID; they never expose stack traces, raw Docker errors, hidden data, or host paths.

## 18. Authorization and data exposure

Required application-level checks include:

- submit: active user, published revision, runtime eligibility, quota, and contest eligibility;
- source read: owner or `submission.read_any`;
- ordinary submission result: owner, contest visibility rules, or administrative authority;
- hidden tests/assets: protected authoring authority only;
- problem draft/revision: author/editor or administrator;
- job/attempt operations: judge administration capability;
- rejudge/cancel: object scope plus explicit command capability.

List queries encode ownership and publication constraints in the database query. Sensitive fields have separate response mappers so an accidental broad entity serialization cannot expose them.

## 19. Logging, correlation, and metrics

The originating HTTP CID is persisted in submission, job, attempt, and outbox metadata. The worker restores it into context. go-common supplies correlation propagation and logging; Judgify adds domain fields:

```text
submission_id, job_id, generation, attempt_id,
problem_revision_id, testset_revision_id, worker_id, event_id
```

`INFO` logs durable state transitions and execution summaries. `WARN` logs retries, lease expiry, worker incompatibility, capacity pressure, and client-caused rate limits. `ERROR` logs infrastructure/finalization failure. `DEBUG` records bounded scheduler, supervisor, container lifecycle, and checker decisions and is disabled by the production default level.

No log level includes source code, test input/expected output, raw contestant output, lease tokens, Docker credentials/socket details, or unbounded compiler diagnostics.

Required metrics include:

- accepted submission rate and idempotency replays;
- queue depth/oldest age by lane;
- claim and lease expiry rate;
- judge throughput and duration by language/verdict;
- retry/dead rate by infrastructure error code;
- worker capability-heartbeat freshness, claims-enabled capacity, activation/advertisement drift, and ready-job compatibility misses;
- compile/run container create/remove failures and capacity;
- cgroup-accounting/control-channel inconsistencies;
- artifact archive/manifest rejection by bounded reason code;
- staging reserved/used bytes/inodes, refcount/orphan recovery, hash-collision alarms, and disk watermarks;
- built-in-checker duration and internal failure count;
- finalization conflicts/latency;
- output/time/memory limit distribution;
- outbox lag after judgement.

Metrics labels are bounded enums; UUIDs and CID are never metric labels.

## 20. Failure and crash semantics

| Failure point | Required outcome |
|---|---|
| Before submission transaction commit | No success response; retry with same idempotency key is safe |
| After commit, before API response | Durable submission/job exists; retry returns the same submission |
| Contest reservation conflicts or rolls back | No submission/order is committed; lock retry preserves a single immutable contest sequence |
| Local wake-up signal lost | Worker polling still claims the PostgreSQL job |
| Target capability was never activated or was retired | API rejects before commit and never emits the unsupported tuple |
| Activated workers are transiently offline | API queues durably in degraded mode until bounded backlog/age/disk admission thresholds; jobs remain ready |
| Worker dies before claim commit | Job remains ready |
| Worker dies after claim | Lease expires; job retries with old attempt retained |
| Docker/supervisor/cgroup control fails | Infrastructure retry/dead policy; never contestant verdict |
| Compile archive is hostile or violates manifest | Attempt fails closed as infrastructure/security error; no bytes/path are executed |
| Staging hash collision/corruption is detected | Worker fails closed/unready, preserves bounded forensic metadata, and never reuses or executes the blob |
| Testcase container fails or accounting is ambiguous | Attempt retries as infrastructure failure; no guessed testcase verdict |
| Heartbeat loses lease | Local execution is cancelled; finalization is rejected |
| Worker dies after execution, before finalize | Lease recovery reruns safely; no terminal event exists |
| Finalize transaction rolls back | No judgement, terminal state, done job, or outbox is partially visible |
| Worker dies after finalize commit | Terminal state and outbox are durable; duplicate finalize is harmless |
| Generation exhausts infrastructure retries | Latest outcome is `system_error`; prior active judgement and downstream contributions remain unchanged |
| Duplicate/out-of-order event | Inbox/version checks make downstream effects idempotent |
| Solve consumer receives a version gap | It replays immutable solve transitions contiguously or poisons visibly; it never guesses/rewards twice |
| Rejudge races old attempt | Generation/version/lease guard rejects the old attempt |
| API or worker receives SIGTERM | Stop intake/claims, drain bounded work, then cancel/release through lease semantics |
| PostgreSQL restarts | API does not acknowledge writes; workers stop claims/finalize and resume from durable jobs |
| Docker restarts | Active attempts retry as infrastructure failures after lease recovery |
| Host/staging disk or inode pressure | Reserved quotas roll back safely; readiness/admission/alerts block new work before corruption; output/tmpfs remain bounded |

## 21. Migration and rollout

The migration follows expand, backfill, switch, verify, and later contract removal.

### 21.1 Expand

Add:

- raw problem/testset revision and test-asset tables, `platform.content_render_sources`, shared `content_render_artifacts`, revision/source/artifact pointers, renderer/checksum provenance, and immutable difficulty/tag/element/reward snapshots;
- problem lifecycle and atomic draft/published revision-source-artifact pointer triples;
- contest submission counters plus submission contest-problem, scoring-policy, database timestamp, elapsed, and ordering fields with the partial unique constraint;
- submission optimistic version/idempotency/correlation fields, `latest_generation`, latest phase/outcome, and separate active judgement/generation/verdict fields;
- judge specs, durable capability activations, worker capability-heartbeats, jobs, attempts, results, judgement history, and compatibility/lease indexes;
- canonical solve projection and immutable solve-transition tables with contiguous-version constraints;
- outbox, consumer inbox/cursor, poison-event, and required query/uniqueness indexes.

New columns are nullable or safely defaulted during backfill. No legacy column is dropped in the expand release.

### 21.2 Backfill

- Pin one reviewed migration renderer/sanitizer version. For every existing problem, normalize its Markdown, register a `platform.content_render_sources` row/checksum, snapshot current difficulty/tags/element/reward mapping, create one immutable testset revision from deterministically ordered tests, create one raw immutable problem revision bound to that render source, then create a separate immutable `content_render_artifacts` row referencing the same source.
- A problem is republished only when the exact revision/source/artifact triple, non-empty hidden testset, assets, checker, runtime, limits, reward snapshot, and checksums pass the same publish validator as a new triple. Unsafe/unrenderable or otherwise invalid legacy content remains an audited draft/quarantined migration exception with no published triple; migration never blesses raw legacy HTML or guesses a checksum.
- Set every historical submission's `submitted_at` to its immutable database `created_at`. For each contest, assign `contest_submission_sequence = row_number() over (partition by contest_id order by created_at, id)`, snapshot the reconstructable contest-problem/scoring-policy identity, derive elapsed time from server contest start, and set its counter to the maximum assigned sequence. Ambiguous contest membership/policy rows are explicit migration exceptions resolved before standings switch, never inferred during event consumption.
- Apply a reviewed, exhaustive legacy-status mapping. A historical contestant verdict receives generation `1`, latest phase/outcome `terminal`/that verdict, an immutable judgement/spec marked `legacy_unverifiable`, and matching active judgement/generation/verdict. Historical `system_error` receives terminal/system-error with null active fields unless an earlier successful contestant judgement can be proven. Historical cancellation receives cancelled/cancelled and no judgement. Unknown states block cutover.
- Valid pending/judging rows become generation `1`, latest phase/outcome `queued`/null with an immutable current spec and one ready PostgreSQL job; the old consumer is already stopped. Invalid problem/testset/runtime references become terminal `system_error` plus canonical judgement and audit record instead of disappearing. Active fields remain null.
- Recompute `user_problem_solves` only from active Accepted judgements, choosing the earliest by `(submitted_at, submission_id)`. Each solved pair starts at version `1` and receives one immutable `unsolved@0 → solved@1` transition with `provenance = legacy_baseline`, `reward_provenance = legacy_reconstructed`, the active revision's reward facts, and grant source equal to that transition ID.
- Before live solve-event dispatch, import a checksummed Cultivation `legacy_reward_allocation_manifest`. For each solved user/problem it maps the baseline transition ID to an exact reconstructed grant vector plus provenance evidence or explicit operator approval; migration inserts that grant and seeds Cultivation's active-ledger pointer to it. The sum of allocated solve grants plus a separately classified non-solve residual opening balance must equal the pre-cutover ledger/balance exactly. The residual is never attached to or reversed by a solve, and migration never guesses a historical grant with the current formula. Missing allocation, approval, checksum, active pointer, or sum reconciliation blocks cutover. Dependent projections are backfilled and solve consumer cursors are then seeded through baseline version `1`; later unsolve reverses the exact allocated grant and later re-solve creates a new live grant.
- Compute and verify source, render, content, testset, judge-spec, and historical provenance checksums. Rebuild contest/statistics counters from active judgement contributions rather than legacy increment-only totals.

Backfill is restartable and idempotent using stable migration keys, unique constraints, per-batch progress, and a migration manifest containing renderer/runtime/status-mapping versions. Rerunning it produces byte-identical checksums, sequences, transitions, grants, and baseline cursors.

### 21.3 Switch

- Deploy `judgify api`, `judgify judge-worker`, and `judgify migrate` process roles.
- Stop old Forge judge consumers before enabling PostgreSQL claims.
- Start capability-probed workers with separate normalized rows for the complete current and previous tuples, run a tuple-specific smoke judgement for each, and persist the audited exact current-tuple activation before switching API emission.
- Switch ordinary and contest submission creation to the caller-owned atomic PostgreSQL transaction and transaction-bound Problem/Contest participants.
- Enable worker claim/lease/finalization and new read projections.
- Enable contest/statistics consumers using active-judgement contribution replacement and Cultivation consumers using solve-transition cursors.
- Deploy frontend UUID contracts, sanitized rendered statements, separated latest phase/outcome and active judgement/verdict fields, immutable contest ordering, and WebSocket-driven progress.
- Verify no production path writes judge state through Forge or the old direct repository sequence.

There is never a period where Forge and PostgreSQL are competing sources of truth.

### 21.4 Verify and contract later

Verify every revision/source/artifact pointer triple plus renderer/content/testset/spec/reward checksums, contest sequence uniqueness/order/counter maxima, exhaustive legacy-status mapping, active-pointer/verdict consistency, pending-job coverage, terminal judgement parity, contiguous solve transitions/cursors, exact Cultivation reconstructed-grant reconciliation, worker compatibility, queue depth, and outbox consumption. Compare rebuilt contest/statistics projections with reviewed legacy reports and resolve every migration exception before switching reads.

Old mutable testcase, raw statement, ambiguous status, and legacy summary columns remain read-only during the rollback window. Their removal occurs only in a later release after production verification, a successful restore rehearsal, no remaining `N-1` jobs, and proof that no application/event consumer reads them.

## 22. Test strategy

### 22.1 Domain and state-machine tests

- all valid/invalid submission, job, attempt, and generation transitions;
- latest phase/outcome and active judgement/verdict invariants across first judge, rejudge, retry, cancellation, and terminal `system_error`;
- idempotency same-key/same-body and same-key/different-body behavior;
- terminal uniqueness and optimistic version conflicts;
- retry classification and bounded backoff;
- rejudge/cancel races;
- verdict precedence and empty-testset rejection;
- problem/testset revision immutability, renderer golden artifacts, sanitizer/XSS fuzz cases, publish validation, and render/content checksum determinism;
- database rejection of render-source kind/checksum, normalized-Markdown/source, revision/source, or artifact/source composite-FK mismatches and audited same-revision/source security artifact switches;
- checker-v1 golden matrices for CRLF/terminal-newline/trailing-space exact behavior, ASCII-versus-Unicode token boundaries and token counts, float tolerance OR-boundaries/zero/large magnitudes, numeric-versus-literal tokens, and NaN/Inf/overflow rejection, plus fuzz inputs.

### 22.2 PostgreSQL integration and crash tests

- submission/spec/job/outbox atomic creation;
- concurrent idempotent submissions;
- contest intake races proving lifecycle/registration/problem snapshot and counter reservation commit atomically, same-key replay allocates no order, rollback consumes no order, and `(sequence, UUID)` replay order is deterministic;
- concurrent claims with no duplicate active lease;
- fresh/stale current-boot heartbeat, `claims_enabled`, separate exact `N`/`N-1` tuple rollout, unsupported runtime/image/manifest/supervisor component, prevention of Cartesian-product advertisement, and exact claim-SQL compatibility filtering;
- `SKIP LOCKED`, per-user limits, lane borrowing, and aging;
- lease heartbeat, expiry, reclaim, and stale finalization rejection;
- finalize all-or-nothing transaction and duplicate finalize;
- rejudge generation wins over old workers;
- successful rejudge replaces active contribution while cancelled/system-error rejudge preserves it;
- exact solve-transition/reward-facts contract, unique contiguous versions, duplicate/old delivery, gap recovery, poison behavior, live and manifest-allocated legacy-reconstructed grant/reversal/re-solve ledger IDs, active-ledger pointers, and mutable-current-metadata independence;
- concurrent solve finalization versus Cultivation trait/formula/profile publication proving shared/exclusive reward-timeline lock order and exact `occurred_at`/`effective_at` before-or-after selection without deadlock;
- outbox/inbox idempotency and contest contribution rebuild under reverse event order;
- migration from an empty database and representative legacy snapshots covering proven/operator-approved allocation manifests, checksum failure, non-solve residual opening balance, and unresolved-allocation cutover blocking;
- worker crash at every staging reserve/spool/fsync/atomic-publish/ref-release boundary followed by lease-aware orphan recovery;
- process kill injected before/after every transaction boundary listed in the failure table.

### 22.3 Supervisor and runtime golden corpus

Each supported runtime passes deterministic fixtures for:

- Accepted;
- Wrong Answer;
- Compile Error;
- non-zero exit Runtime Error;
- signal/crash Runtime Error;
- CPU and wall Time Limit Exceeded;
- Memory Limit Exceeded;
- Process Limit Exceeded;
- Output Limit Exceeded;
- empty output and whitespace/token/float checker edges;
- maximum source/input/output bounds;
- Unicode and binary-like output handling;
- compile container removal before run and a distinct container/cgroup ID for every testcase;
- active `cpu.stat` delta watchdog kills single- and multi-process CPU consumers at total CPU limit independently of Docker rate quota/wall time;
- Docker/cgroup accounting disagreement, unavailable controller, and capability-probe failure as infrastructure errors;
- built-in checker execution only after contestant-container removal.

### 22.4 Malicious sandbox corpus

The sandbox is tested against:

- fork/process bomb;
- daemon/background process escape;
- network access;
- host filesystem and device access;
- Docker socket/namespace probing;
- absolute/traversal/NUL/duplicate/case-collision artifact paths;
- symlink, hardlink, device, socket, FIFO, sparse/special archive entries;
- archive bombs, excessive file counts/expanded bytes, ownership/xattr tricks, executable allowlist violations, missing/extra manifest files, and contestant control JSON;
- forced hash-path collision/content mismatch, disk-full/inode-full mid-spool, quota reservation races, stale boot manifests, and temp-file crash recovery;
- oversized stdout/stderr/files;
- memory allocation and shared-memory abuse;
- CPU spin, sleeping, and signal manipulation;
- stale process/file state between tests and attempts, including a first testcase that attempts to plant state consumed by the second;
- control-protocol/metadata forgery through stdout/stderr.

Every fixture verifies cleanup: no residual container, process, cgroup, mount, content-addressed blob, or unbounded host resource remains.

### 22.5 Load, race, and soak tests

- accept all 500 valid, in-quota submissions in a 50-per-second, 10-second reference burst without losing a job;
- queue those submissions with bounded API memory and acknowledgement latency while reporting judge completion latency separately;
- measure container-per-test create/copy/start/remove overhead by runtime and test count and enforce the operations capacity model rather than hiding it in attempt duration;
- validate fair scheduling under contest, regular, rejudge, and single-user floods;
- run a four-hour soak with stable goroutine, file descriptor, container, disk, and RSS counts;
- keep staging reserved/used bytes, inode count, refcounts, temp/orphan count, and disk-watermark admission stable across the soak and restarts;
- restart API, worker, PostgreSQL, and Docker at controlled phases;
- run Go race tests for claimant, heartbeat, scheduler, host-supervisor orchestration, transition replay, and finalizer.

## 23. Release gates

The slice cannot be declared production-complete until:

- judge, sandbox-supervisor/checker, scheduler, state-machine, and finalizer packages have at least 90% statement coverage;
- all changed core packages have at least 80% statement coverage;
- unit, integration, race, lint, vet, vulnerability, migration, and generated-schema-drift checks pass;
- golden and malicious sandbox corpora pass for every enabled runtime image digest;
- the production Linux capability probe passes with rootless Docker/cgroup v2, and every API-emitted complete judge capability tuple has an audited activation created from a fresh claims-enabled exact normalized row plus tuple-specific smoke judgement;
- crash injection proves zero loss of acknowledged submissions and zero duplicate terminal side effects;
- every non-cancelled terminal judge generation has exactly one canonical judgement; a cancelled generation has none by design;
- `system_error`/cancelled generations never become active, while every non-null active pointer matches its active generation/verdict and canonical contestant judgement;
- every ready/leased/retrying submission has a corresponding durable job, and every done job has a canonical judgement;
- concurrent contest intake proves immutable unique sequence order and standings replay is independent of judgement/event arrival order;
- every solve aggregate has contiguous immutable transitions/cursors, gap recovery passes, and the checksummed legacy allocation manifest's reconstructed grants plus isolated non-solve residual reconcile exactly to the pre-cutover Cultivation EXP balance;
- every published triple binds one raw revision and checksum-valid immutable sanitized artifact to the same registered source from an approved renderer version; raw authoring source never crosses a public mapper;
- no public/user API exposes another user's source or hidden test material;
- queue depth, oldest age, dead/unsupported jobs, worker capability freshness, lease expiry, per-boundary sandbox capacity, archive rejection, and outbox/solve-consumer lag are observable and alerted;
- backup and restore preserve raw revisions, render sources/artifacts/pointer triples, assets, test inputs, contestant sources, contest order, jobs, attempts, judgements, solve transitions/cursors, and outbox state;
- graceful worker shutdown stops new claims and resolves or safely expires every active lease;
- staging is host-noexec/quota-bounded, crash recovery proves reference-safe cleanup, and no blob is executable or removed while a live attempt references it;
- the API process operates without Docker access and the worker uses only its dedicated rootless Docker daemon under a restricted OS identity;
- no contestant compile/run namespace receives hidden expected output, a host bind, Docker control access, or state from another testcase.

## 24. Explicit exclusions and extension points

Interactive judging, custom/user-authored checkers, plagiarism detection, distributed multi-host scheduling, and external object storage are not part of this production slice. The launch registry rejects every checker key except versioned built-in exact/token/float implementations; no launch job creates a checker container. The immutable judge-spec version, future isolated custom-checker boundary, worker capability protocol, lease-owner model, and test-asset abstraction provide deliberate extension points.

A future custom-checker release requires a new activated specification/capability and its own release gate. Its attack corpus must prove the checker sees only bounded input/expected/contestant output, cannot access contestant artifacts/processes or host/network, and maps crash/timeout/malformed result to infrastructure failure; capacity, cleanup, and metrics must also pass. No future feature may weaken testcase-per-container isolation, run custom code in the host worker, or expose expected output to contestant namespaces.
