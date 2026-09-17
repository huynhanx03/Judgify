# Server-Owned Taxonomy Reward Snapshots Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every immutable Problem reward snapshot originate from versioned server-owned Difficulty, Tag, and Element facts so callers cannot forge revisions, base experience, mappings, weights, or checksums.

**Architecture:** Mutable taxonomy rows remain stable identities/current projections. Every create or update appends an exact immutable catalog revision in the same Ent transaction. `SaveDraft` accepts only a difficulty ID and tag IDs, resolves exact active revisions inside its transaction, then a pure domain factory derives mapping identity, deterministic element weights, and the reward checksum.

**Tech Stack:** Go, Ent, PostgreSQL 16, existing Judgify transaction unit of work, existing append-only Ent mixin/database triggers, Atlas checked-in baseline, GoMock, Testify.

## Global Constraints

- Single-tenant first; no tenant column or tenant-specific abstraction.
- Use only the current repository technology stack; add no dependency.
- Run Go commands with `GOPROXY=off`, `GOMAXPROCS=4`, and `-p=1`.
- Do not download packages or images. PostgreSQL verification must use `postgres:16` with `--pull never`.
- Do not commit. Preserve every unrelated dirty-worktree change.
- Use `apply_patch` for authored source changes; generated Ent/GoMock output may be regenerated mechanically.
- Difficulty, Tag, Element, and Problem mapping identities are Judgify concerns and must not enter `go-common`.
- Catalog revision and link tables are append-only at both Ent and PostgreSQL boundaries.
- The caller may supply only `difficulty_id` and `tag_ids`; it must never supply revision numbers, base EXP, mapping IDs, element IDs/weights, or checksums.
- Resolution and snapshot construction happen inside the same physical transaction as the immutable Problem revision.
- Historical published/draft revisions remain valid after taxonomy rename, update, or soft deletion.

---

### Task 1: Define Reward Intent, Exact Facts, and the Pure Snapshot Factory

**Files:**
- Modify: `api/internal/problem/core/entity/revision.go`
- Modify: `api/internal/problem/core/entity/publication.go`
- Create: `api/internal/problem/core/entity/reward_factory.go`
- Create: `api/internal/problem/core/entity/reward_factory_test.go`
- Modify: `api/internal/problem/ports/revision.go`
- Modify: `api/internal/problem/core/service/draft.go`
- Modify: `api/internal/problem/core/service/draft_test.go`

**Interfaces:**
- Consumes: existing `entity.RewardSnapshot`, `entity.ProblemAggregate`, and `ports.SaveDraftCommand`.
- Produces:

```go
type RewardIntent struct {
    DifficultyID uuid.UUID
    TagIDs       []uuid.UUID
}

type RewardFacts struct {
    Difficulty DifficultyRewardFact
    Tags       []TaxonomyRevisionFact
    Elements   []TaxonomyRevisionFact
}

type DifficultyRewardFact struct {
    ID             uuid.UUID
    Revision       int64
    BaseExperience int64
}

type TaxonomyRevisionFact struct {
    ID       uuid.UUID
    Revision int64
}

type RewardMappingIdentity struct {
    TagMappingID     uuid.UUID
    ElementMappingID uuid.UUID
}

func BuildRewardSnapshot(
    identity RewardMappingIdentity,
    mappingRevision int64,
    facts RewardFacts,
) (RewardSnapshot, error)
```

- `ports.SaveDraftCommand.Reward` becomes `ports.RewardIntent`.
- `ports.TaxonomyCatalog` becomes:

```go
type TaxonomyCatalog interface {
    ResolveRewardFacts(ctx context.Context, intent RewardIntent) (entity.RewardFacts, error)
    VerifyRewardSnapshot(ctx context.Context, snapshot entity.RewardSnapshot) error
}
```

- [ ] **Step 1: Write failing factory tests**

Cover:

```go
func TestBuildRewardSnapshotRejectsCallerControlledOrInvalidFacts(t *testing.T)
func TestBuildRewardSnapshotIsOrderIndependentAndDeterministic(t *testing.T)
func TestBuildRewardSnapshotDeduplicatesElementUnionAndAllocatesExactlyTenThousandBasisPoints(t *testing.T)
func TestBuildRewardSnapshotUsesServerOwnedMappingIdentityAndRevision(t *testing.T)
```

The deterministic element allocation is:

```text
base = 10000 / element_count
remainder = 10000 % element_count
sorted element indexes [0,remainder) receive base+1; all others receive base
```

Reject nil UUIDs, duplicate fact IDs, non-positive/out-of-range revisions, excessive fact counts, negative/out-of-range base EXP, nil mapping IDs, and mapping revisions outside `1..MaximumRewardMappingRevision`.

- [ ] **Step 2: Run the factory tests and verify RED**

Run:

```bash
cd api
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go test -p=1 ./internal/problem/core/entity \
  -run '^TestBuildRewardSnapshot' -count=1
```

Expected: compile failure because the fact/identity types and factory do not exist.

- [ ] **Step 3: Implement the minimal pure factory**

Canonicalize tag and element facts by UUID byte order, reject duplicates instead of silently hiding malformed catalog output, derive `TagMappingSnapshot` and optional `ElementMappingSnapshot`, call `ComputeChecksum`, and return a snapshot that passes `Validate`.

Add `TagMappingID` and `ElementMappingID` to `ProblemAggregate`; make `ProblemAggregate.Validate` require both to be non-nil stable identity fields.

- [ ] **Step 4: Refactor draft input without weakening transaction ordering**

`prepare` may normalize/dedupe/sort only the authored `RewardIntent`; it must not build a snapshot. Inside `SaveDraft`, after `LockAggregate`, authorization, and `NextRevisionNumbers`, call:

```go
facts, err := service.dependencies.Taxonomy.ResolveRewardFacts(txCtx, command.Reward)
reward, err := entity.BuildRewardSnapshot(
    entity.RewardMappingIdentity{
        TagMappingID: aggregate.TagMappingID,
        ElementMappingID: aggregate.ElementMappingID,
    },
    numbers.ProblemRevision,
    facts,
)
```

Pass `reward` to `buildDraftRevision`. Publication continues to call `VerifyRewardSnapshot` through the same catalog port.

- [ ] **Step 5: Update service fakes and prove no writes occur on taxonomy/factory failure**

Tests must assert taxonomy resolution occurs after lock/authorization/number allocation but before assets, testsets, content, revisions, audit, outbox, or aggregate CAS. Add a test whose fake returns forged duplicate facts and assert the transaction rolls back with zero staged writes.

- [ ] **Step 6: Run focused domain/service gates**

Run:

```bash
cd api
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go test -p=1 ./internal/problem/core/entity ./internal/problem/core/service \
  -run 'Test(BuildRewardSnapshot|Draft|Publication)' -count=1
```

Expected: all pass. Record evidence; do not commit.

---

### Task 2: Add Stable Catalog Versions and Append-Only Taxonomy Schemas

**Files:**
- Modify: `api/internal/ent/schema/difficulty.go`
- Modify: `api/internal/ent/schema/tag.go`
- Modify: `api/internal/ent/schema/element.go`
- Create: `api/internal/ent/schema/difficulty_revision.go`
- Create: `api/internal/ent/schema/tag_revision.go`
- Create: `api/internal/ent/schema/element_revision.go`
- Create: `api/internal/ent/schema/tag_revision_element.go`
- Modify: `api/internal/ent/schema/problem.go`
- Modify: `api/internal/ent/schema/problem_revision.go`
- Modify: `api/internal/ent/schema/problem_test.go`
- Create: `api/internal/ent/schema/taxonomy_revision_test.go`
- Modify: `api/internal/ent/schemaextras/append_only.go`
- Modify: `api/internal/ent/desiredschema/schema.go`
- Modify: `api/internal/ent/desiredschema/schema_test.go`

**Interfaces:**
- Consumes: contract maxima and existing `problemAppendOnlyRecordFields`.
- Produces stable projection fields:

```text
difficulties.version bigint not null default 1
tags.version bigint not null default 1
elements.version bigint not null default 1
problems.tag_mapping_id uuid not null immutable
problems.element_mapping_id uuid not null immutable
```

- Produces append-only tables:

```text
difficulty_revisions(id, difficulty_id, revision, name, level, exp_reward, description, checksum, created_at)
tag_revisions(id, tag_id, revision, name, checksum, created_at)
element_revisions(id, element_id, revision, name, code, description, checksum, created_at)
tag_revision_elements(id, tag_revision_id, element_id, element_revision, created_at)
```

- [ ] **Step 1: Write failing Ent-schema contract tests**

Assert:

- stable version fields are required, default to 1, and positive;
- Problem mapping IDs are required and immutable;
- every revision/link schema uses `commonent.AppendOnlyMixin`;
- every authored revision field is immutable;
- unique tuples exist for `(difficulty_id,revision)`, `(tag_id,revision)`, `(element_id,revision)`, and `(tag_revision_id,element_id)`;
- checksums are exactly 32 non-zero bytes;
- revision/link IDs and owner IDs are non-zero;
- tag revision links carry an exact element revision, not only an element identity.

- [ ] **Step 2: Run schema tests and verify RED**

Run:

```bash
cd api
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go test -p=1 ./internal/ent/schema ./internal/ent/schemaextras \
  -run 'Test(Taxonomy|Problem.*Mapping|AppendOnlyTableCatalog)' -count=1
```

Expected: missing schemas/fields and append-only catalog mismatch.

- [ ] **Step 3: Implement schemas with closed bounds**

Use `problemAppendOnlyRecordFields()` plus `commonent.AppendOnlyMixin{}`. Reuse existing name/code/description/EXP bounds exactly. Use SHA-256 checksum bytes for each revision. Do not add `updated_at`, `deleted_at`, or mutable edges to revision/link tables.

`TagRevisionElement` stores exact `(element_id,element_revision)` scalar fields. Do not use an implicit Ent M2M join because it cannot carry the exact revision tuple or the append-only hook.

- [ ] **Step 4: Extend relational identity constraints**

Add exact unique indexes needed by composite FKs and extend `desiredschema` with:

```text
tag_revision_elements(element_id, element_revision)
  -> element_revisions(element_id, revision)

problem_revisions(difficulty_id, difficulty_revision)
  -> difficulty_revisions(difficulty_id, revision)

problem_revisions(problem_id, tag_mapping_id)
  -> problems(id, tag_mapping_id)

problem_revisions(problem_id, element_mapping_id)
  -> problems(id, element_mapping_id)
```

The element mapping FK is nullable as a complete tuple; DB checks still require all-or-none element mapping fields. Problem stable indexes on `(id,tag_mapping_id)` and `(id,element_mapping_id)` are unique.

- [ ] **Step 5: Extend append-only coverage**

Add the four table names to `schemaextras.AppendOnlyTables`. The derived catalog test must resolve them automatically from Ent mixins and match exactly.

- [ ] **Step 6: Generate Ent offline and run schema gates**

Run:

```bash
cd api
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go generate ./internal/ent
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go test -p=1 ./internal/ent/schema ./internal/ent/schemaextras ./internal/ent/desiredschema -count=1
```

Expected: generated code and all schema gates pass. Record evidence; do not commit.

---

### Task 3: Make Catalog Writes Transactional and Revision-Producing

**Files:**
- Modify: `api/internal/problem/core/entity/difficulty.go`
- Modify: `api/internal/problem/core/entity/tag.go`
- Modify: `api/internal/cultivation/core/entity/element.go`
- Modify: `api/internal/problem/adapters/driven/db/mapper/difficulty.go`
- Modify: `api/internal/problem/adapters/driven/db/mapper/tag.go`
- Modify: `api/internal/cultivation/adapters/driven/db/mapper/element.go`
- Modify: `api/internal/problem/adapters/driven/db/difficulty.go`
- Modify: `api/internal/problem/adapters/driven/db/tag.go`
- Modify: `api/internal/cultivation/adapters/driven/db/element.go`
- Modify: corresponding builder files
- Create: `api/internal/problem/adapters/driven/db/taxonomy_revision_write_test.go`
- Create: `api/internal/cultivation/adapters/driven/db/element_revision_write_test.go`

**Interfaces:**
- Stable entities gain `Version int64`.
- Existing repository interfaces remain source-compatible.
- `Create` produces stable version 1 and revision 1 in one physical transaction.
- `Update` locks the stable row, requires `stored.version == entity.Version`, appends revision `version+1`, updates the current projection and current mutable tag-element projection, then advances stable version.

- [ ] **Step 1: Write transaction-probe and optimistic-conflict tests**

For Difficulty, Tag, and Element, prove:

- no root-client write occurs inside an existing transaction;
- stable create plus revision insert commit together;
- injected second-write failure rolls back the stable row;
- two updates from the same entity version produce one success and one typed conflict;
- tag membership update creates `tag_revision_elements` bound to exact current Element revisions;
- missing/deleted Element ID rejects a Tag create/update before any commit.

- [ ] **Step 2: Run focused repository tests and verify RED**

Run:

```bash
cd api
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go test -p=1 ./internal/problem/adapters/driven/db ./internal/cultivation/adapters/driven/db \
  -run 'Test(Difficulty|Tag|Element).*Revision' -count=1
```

Expected: current repositories write only mutable projections and builders still use the global client.

- [ ] **Step 3: Implement checksum helpers and transactional writers**

Use a versioned canonical length-prefixed SHA-256 preimage per catalog type:

```text
judgify.difficulty-revision.v1
judgify.tag-revision.v1
judgify.element-revision.v1
```

Tag revision checksum includes sorted `(element_id, element_revision)` tuples. Never hash locale-dependent formatting or JSON map iteration.

Use `r.client.DoInTx`; acquire `FOR UPDATE` on update; use `r.client.DB(txCtx)` for every builder/query. Remove `global.EntClient` from touched builders.

- [ ] **Step 4: Preserve current projection compatibility**

Existing reads and DTOs continue to use stable rows. Map `Version` through entities and responses so later admin UI can submit/display concurrency state. Cache invalidation stays after successful transaction completion only.

- [ ] **Step 5: Run repository/service gates**

Run:

```bash
cd api
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go test -p=1 ./internal/problem/... ./internal/cultivation/... -count=1
```

Expected: all pass. Record evidence; do not commit.

---

### Task 4: Implement the Exact Ent Taxonomy Catalog and Wire Server-Owned Draft Resolution

**Files:**
- Create: `api/internal/problem/adapters/driven/db/taxonomy_catalog.go`
- Create: `api/internal/problem/adapters/driven/db/taxonomy_catalog_test.go`
- Modify: `api/internal/problem/adapters/driven/db/revision.go`
- Modify: `api/internal/problem/core/service/draft.go`
- Modify: `api/internal/problem/core/service/publication.go`
- Modify: `api/internal/problem/core/service/draft_test.go`
- Modify: `api/internal/problem/core/service/publication_test.go`

**Interfaces:**
- Implements the Task 1 `ports.TaxonomyCatalog`.
- Resolution returns exact current revision facts only for active stable rows.
- Verification checks the exact historical Difficulty/Tag/Element revision tuples, mapping ownership, deterministic weights, and checksum; it does not require those revisions to remain current.

- [ ] **Step 1: Write failing catalog tests**

Cover:

- missing, duplicate, or soft-deleted Difficulty/Tag/Element fails closed;
- all requested tag IDs must be returned exactly once;
- tag input order cannot change facts or checksum;
- element union is exact, sorted, and deduplicated;
- current Difficulty revision supplies base EXP;
- historical `VerifyRewardSnapshot` still passes after a newer catalog revision;
- forged base EXP, mapping ID, revision, weight, element membership, or checksum fails;
- typed-nil/nil context/cancelled context fails closed;
- all reads use the transaction-scoped Ent client.

- [ ] **Step 2: Run focused catalog tests and verify RED**

Run:

```bash
cd api
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go test -p=1 ./internal/problem/adapters/driven/db \
  -run '^TestTaxonomyCatalog' -count=1
```

Expected: catalog adapter does not exist.

- [ ] **Step 3: Implement exact resolution**

Read the active stable Difficulty and Tags with their `version`. Load matching revision rows by exact `(stable_id,version)`, load `tag_revision_elements`, and load the referenced exact Element revisions. Compare requested and returned UUID sets after canonical sorting; never accept partial `IN` query results.

- [ ] **Step 4: Implement historical verification**

Rebuild expected facts from exact persisted revision tuples, call `BuildRewardSnapshot` with the snapshot's mapping identity/revision, and compare the entire canonical snapshot including checksum. Verify Problem mapping ownership separately when loading/storing the Problem revision.

- [ ] **Step 5: Prove service transaction ordering**

Update draft/publication fakes for the new port methods. Add tests that resolve/verify through the transaction-derived context and that taxonomy failures leave zero immutable/evidence writes.

- [ ] **Step 6: Run Task 2 core gates repeatedly**

Run:

```bash
cd api
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go test -p=1 ./internal/problem/core/entity ./internal/problem/core/service \
  ./internal/problem/adapters/driven/db -count=10
```

Expected: all pass. Record evidence; do not commit.

---

### Task 5: Rebuild the Baseline and Prove PostgreSQL Acceptance

**Files:**
- Modify: `api/internal/ent/migrate/migrations/20260722073038_baseline.sql`
- Modify: `api/internal/ent/migrate/migrations/atlas.sum`
- Modify: `api/internal/ent/migrate/problem_revision_test.go`
- Modify: `api/internal/ent/migrate/inventory/inventory.go` only if generated inventory code requires it
- Modify: `docs/superpowers/baseline/2026-07-18-migration-inventory.md`
- Create: `api/internal/problem/adapters/driven/db/taxonomy_postgres_test.go`
- Modify: `scripts/verify/problem_revision_postgres.sh`

**Interfaces:**
- Consumes the complete desired schema from Tasks 2–4.
- Produces one clean no-data baseline with append-only triggers, exact FKs, role grants, and an updated inventory receipt.

- [ ] **Step 1: Add failing migration/static/PostgreSQL tests**

Static tests pin:

- all four append-only tables and triggers;
- stable version/mapping columns;
- all exact unique indexes/FKs;
- API role cannot update/delete/truncate revision tables;
- worker role cannot mutate taxonomy revisions.

PostgreSQL tests prove:

- create/update writes stable projection and exact revision atomically;
- old revisions remain unchanged/readable;
- update/delete/truncate/upsert-update against revision/link tables return SQLSTATE `55000`;
- concurrent catalog update and draft resolution yields one complete before-or-after snapshot, never mixed facts;
- forged/nonexistent exact FK tuples are rejected;
- injected late failure rolls back stable version, revision rows, links, audit, and outbox;
- an old Problem reward snapshot remains valid after catalog update and soft delete.

- [ ] **Step 2: Generate a reviewed baseline offline**

Use the existing Atlas/desired-schema workflow with `GOPROXY=off` and local `postgres:16 --pull never`. Do not hand-invent generated table DDL. Hand-maintain only the already-reviewed extras/role-grant sections that Atlas OSS cannot model.

- [ ] **Step 3: Rehash and refresh migration inventory**

Run the existing offline Atlas hash command and `go run ./cmd/judgify inventory`. Update the checked receipt fingerprint, byte count, and SHA-256 exactly.

- [ ] **Step 4: Run PostgreSQL verifier**

Run:

```bash
cd /Users/jerry/nhanhmc/coding/Judgify
env GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  scripts/verify/problem_revision_postgres.sh
```

Expected: uses `postgres:16 --pull never`; all revision, slug, rollback/concurrency, and taxonomy tests pass.

- [ ] **Step 5: Run final offline gates**

Run:

```bash
cd api
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go test -p=1 ./internal/problem/... ./internal/cultivation/... ./internal/ent/... -count=1
env GOPROXY=off GOCACHE=/private/tmp/judgify-go-cache GOMAXPROCS=4 \
  go test -race -p=1 ./internal/problem/core/entity ./internal/problem/core/service -count=1
git diff --check
```

Expected: all pass, inventory is current, generated drift is empty, and no commit exists.

## Completion Boundary

This plan completes server-owned taxonomy/reward history and the Task 2 reward blocker. It deliberately does not register SaveDraft/Publish/Archive HTTP routes until Task 3 provides real runtime/checker registries. Immutable public/admin/judge projections remain Task 11, and legacy-field/backfill/DB-role cutover remains Task 12.
