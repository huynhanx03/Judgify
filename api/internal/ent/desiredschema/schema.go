// Package desiredschema builds the complete Atlas desired state for Judgify.
package desiredschema

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	entdialect "entgo.io/ent/dialect"
	entschema "entgo.io/ent/dialect/sql/schema"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"

	"github.com/huynhanx03/judgify/internal/ent/schemaextras"
)

const (
	entSchemaPath = "./internal/ent/schema"

	contentRenderSourcesTable           = "content_render_sources"
	contentRenderArtifactsTable         = "content_render_artifacts"
	materialsTable                      = "materials"
	materialRevisionsTable              = "material_revisions"
	materialSlugsTable                  = "material_slugs"
	difficultyRevisionsTable            = "difficulty_revisions"
	elementRevisionsTable               = "element_revisions"
	problemsTable                       = "problems"
	usersTable                          = "users"
	attributeDefinitionRevisionsTable   = "attribute_definition_revisions"
	userAttributeValuesTable            = "user_attribute_values"
	contestProblemsTable                = "contest_problems"
	contestAnnouncementsTable           = "contest_announcements"
	contestClarificationsTable          = "contest_clarifications"
	contestContentRevisionsTable        = "contest_content_revisions"
	contestRegistrationsTable           = "contest_registrations"
	contestRegistrationTransitionsTable = "contest_registration_transitions"
	problemRevisionsTable               = "problem_revisions"
	tagRevisionElementsTable            = "tag_revision_elements"
	testCasesTable                      = "test_cases"
	testsetRevisionsTable               = "testset_revisions"
	testcaseGroupsTable                 = "testcase_groups"
	testcaseRevisionsTable              = "testcase_revisions"
	judgeWorkerInstancesTable           = "judge_worker_instances"
	judgeWorkerEvidenceTable            = "judge_worker_capabilities"
	activatedCapabilityTable            = "activated_judge_capabilities"
	submissionsTable                    = "submissions"
	judgeSpecsTable                     = "judge_specs"
	judgeJobsTable                      = "judge_jobs"
	judgeAttemptsTable                  = "judge_attempts"
	submissionJudgementsTable           = "submission_judgements"
	judgeTestResultsTable               = "judge_test_results"
	userProblemSolvesTable              = "user_problem_solves"
	solveTransitionsTable               = "user_problem_solve_transitions"

	renderSourceIdentityForeignKey  = "content_render_artifacts_render_source_identity_fkey"
	workerBootIdentityForeignKey    = "judge_worker_capabilities_worker_boot_fkey"
	entWorkerEvidenceForeignKey     = "judge_worker_capabilities_judge_worker_instances_capabilities"
	activationProofForeignKey       = "activated_judge_capabilities_worker_evidence_fkey"
	entActivationEvidenceForeignKey = "activated_judge_capabilities_judge_worker_capabilities_activations"
	postgresVersion                 = "16"
)

var (
	artifactIdentityColumns = []string{"render_source_id", "render_source_kind", "source_checksum"}
	sourceIdentityColumns   = []string{"id", "source_kind", "source_checksum"}
)

// State is the validated, complete desired schema consumed by Atlas.
type State struct {
	tables []*entschema.Table
}

// entc.LoadGraph writes into a hardcoded ".entc" directory next to the working
// directory and removes that whole directory on the way out, with a temp file
// name that only has one-second resolution. Concurrent loads therefore delete
// each other's sources. The Ent schema cannot change inside a running process,
// so serialise the load and reuse the graph. Callers still get freshly built
// tables from graph.Tables(), so no state is shared between them.
var (
	entGraphMutex sync.Mutex
	entGraphValue *gen.Graph
	entGraphErr   error
	entGraphDone  bool
)

func loadEntGraph() (*gen.Graph, error) {
	entGraphMutex.Lock()
	defer entGraphMutex.Unlock()
	if !entGraphDone {
		entGraphValue, entGraphErr = entc.LoadGraph(entSchemaPath, &gen.Config{})
		entGraphDone = true
	}
	return entGraphValue, entGraphErr
}

// Load reads the pinned Ent schema graph and adds constraints Ent cannot express.
// It must be called from the API module root so entSchemaPath resolves to the
// same package used by the normal Ent and Atlas authoring workflows.
func Load(ctx context.Context) (*State, error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	graph, err := loadEntGraph()
	if err != nil {
		return nil, fmt.Errorf("desired schema: load Ent graph %q: %w", entSchemaPath, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tables, err := graph.Tables()
	if err != nil {
		return nil, fmt.Errorf("desired schema: build Ent tables: %w", err)
	}
	if err := extendContentRenderIdentity(tables); err != nil {
		return nil, fmt.Errorf("desired schema: %w", err)
	}
	if err := extendMaterialIdentity(tables); err != nil {
		return nil, fmt.Errorf("desired schema: %w", err)
	}
	if err := extendProfileAttributeValueProvenance(tables); err != nil {
		return nil, fmt.Errorf("desired schema: %w", err)
	}
	if err := extendContestContentIdentity(tables); err != nil {
		return nil, fmt.Errorf("desired schema: %w", err)
	}
	if err := extendProblemIdentity(tables); err != nil {
		return nil, fmt.Errorf("desired schema: %w", err)
	}
	if err := extendSubmissionJudgeIdentity(tables); err != nil {
		return nil, fmt.Errorf("desired schema: %w", err)
	}
	if err := extendContestProblemIdentity(tables); err != nil {
		return nil, fmt.Errorf("desired schema: %w", err)
	}
	if err := extendContestRegistrationTransitionIdentity(tables); err != nil {
		return nil, fmt.Errorf("desired schema: %w", err)
	}
	if err := extendJudgeWorkerCapabilityIdentity(tables); err != nil {
		return nil, fmt.Errorf("desired schema: %w", err)
	}
	if err := extendActivationEvidenceIdentity(tables); err != nil {
		return nil, fmt.Errorf("desired schema: %w", err)
	}
	return &State{tables: tables}, nil
}

func extendActivationEvidenceIdentity(tables []*entschema.Table) error {
	evidence, err := exactTable(tables, judgeWorkerEvidenceTable)
	if err != nil {
		return err
	}
	activation, err := exactTable(tables, activatedCapabilityTable)
	if err != nil {
		return err
	}
	localNames := []string{
		"proof_evidence_id", "proof_worker_instance_id", "proof_boot_id",
		"judge_spec_protocol", "runtime_key", "runtime_profile_version",
		"compiler_version", "runtime_bootstrap_version", "compile_command_id", "run_command_id",
		"image_digest", "manifest_version", "manifest_checksum", "supervisor_protocol",
		"checker_key", "checker_version", "capability_profile_hash",
		"checker_implementation_checksum", "smoke_checksum", "golden_checksum",
	}
	targetNames := []string{
		"id", "worker_instance_id", "boot_id",
		"judge_spec_protocol", "runtime_key", "runtime_profile_version",
		"compiler_version", "runtime_bootstrap_version", "compile_command_id", "run_command_id",
		"image_digest", "manifest_version", "manifest_checksum", "supervisor_protocol",
		"checker_key", "checker_version", "capability_profile_hash",
		"checker_implementation_checksum", "smoke_checksum", "golden_checksum",
	}
	localColumns, err := exactColumns(activation, localNames)
	if err != nil {
		return err
	}
	targetColumns, err := exactColumns(evidence, targetNames)
	if err != nil {
		return err
	}
	if err := validateExactUniqueIndex(evidence, targetNames); err != nil {
		return err
	}

	var entSingle, exactComposite *entschema.ForeignKey
	for _, foreignKey := range activation.ForeignKeys {
		if foreignKey == nil {
			return fmt.Errorf("table %q contains a nil foreign key", activation.Name)
		}
		if foreignKey.RefTable != evidence {
			continue
		}
		switch foreignKey.Symbol {
		case entActivationEvidenceForeignKey:
			if entSingle != nil || !equalColumnNames(foreignKey.Columns, []string{"proof_evidence_id"}) ||
				!equalColumnNames(foreignKey.RefColumns, []string{"id"}) {
				return fmt.Errorf("unexpected Ent activation evidence foreign key shape")
			}
			entSingle = foreignKey
		case activationProofForeignKey:
			if exactComposite != nil || !equalColumnNames(foreignKey.Columns, localNames) ||
				!equalColumnNames(foreignKey.RefColumns, targetNames) {
				return fmt.Errorf("unexpected activation proof foreign key shape")
			}
			exactComposite = foreignKey
		default:
			return fmt.Errorf("unexpected activation evidence foreign key %q", foreignKey.Symbol)
		}
	}
	if exactComposite != nil {
		if entSingle != nil {
			return fmt.Errorf("exact activation proof foreign key coexists with weaker Ent foreign key")
		}
		return nil
	}
	if entSingle == nil {
		return fmt.Errorf("expected exactly one Ent activation evidence foreign key, found 0")
	}
	entSingle.Symbol = activationProofForeignKey
	entSingle.Columns = localColumns
	entSingle.RefColumns = targetColumns
	entSingle.OnUpdate = entschema.NoAction
	entSingle.OnDelete = entschema.NoAction
	return nil
}

// extendJudgeWorkerCapabilityIdentity replaces Ent's edge-derived
// single-column reference with the boot-bound identity. Keeping both would
// leave a weaker relational contract available to schema consumers.
func extendJudgeWorkerCapabilityIdentity(tables []*entschema.Table) error {
	worker, err := exactTable(tables, judgeWorkerInstancesTable)
	if err != nil {
		return err
	}
	evidence, err := exactTable(tables, judgeWorkerEvidenceTable)
	if err != nil {
		return err
	}
	localNames := []string{"worker_instance_id", "boot_id"}
	targetNames := []string{"id", "boot_id"}
	localColumns, err := exactColumns(evidence, localNames)
	if err != nil {
		return err
	}
	targetColumns, err := exactColumns(worker, targetNames)
	if err != nil {
		return err
	}
	if err := validateExactUniqueIndex(worker, targetNames); err != nil {
		return err
	}

	var entSingle, exactComposite *entschema.ForeignKey
	for _, foreignKey := range evidence.ForeignKeys {
		if foreignKey == nil {
			return fmt.Errorf("table %q contains a nil foreign key", evidence.Name)
		}
		if foreignKey.RefTable != worker {
			continue
		}
		switch foreignKey.Symbol {
		case entWorkerEvidenceForeignKey:
			if entSingle != nil || !equalColumnNames(foreignKey.Columns, []string{"worker_instance_id"}) ||
				!equalColumnNames(foreignKey.RefColumns, []string{"id"}) {
				return fmt.Errorf("unexpected Ent worker evidence foreign key shape")
			}
			entSingle = foreignKey
		case workerBootIdentityForeignKey:
			if exactComposite != nil || !equalColumnNames(foreignKey.Columns, localNames) ||
				!equalColumnNames(foreignKey.RefColumns, targetNames) {
				return fmt.Errorf("unexpected exact worker boot foreign key shape")
			}
			exactComposite = foreignKey
		default:
			return fmt.Errorf("unexpected worker evidence foreign key %q", foreignKey.Symbol)
		}
	}
	if exactComposite != nil {
		if entSingle != nil {
			return fmt.Errorf("exact worker boot foreign key coexists with weaker Ent foreign key")
		}
		return nil
	}
	if entSingle == nil {
		return fmt.Errorf("expected exactly one Ent worker evidence foreign key, found 0")
	}
	// Mutating the edge-derived object preserves Ent's graph ownership while
	// making the emitted relational contract exact. Reapplying is idempotent.
	entSingle.Symbol = workerBootIdentityForeignKey
	entSingle.Columns = localColumns
	entSingle.RefColumns = targetColumns
	entSingle.OnUpdate = entschema.NoAction
	entSingle.OnDelete = entschema.NoAction
	return nil
}

// RelationalDDL emits the PostgreSQL 16 tables, indexes, checks, and foreign
// keys that Atlas can round-trip through its dev database. PostgreSQL
// functions and triggers intentionally stay out of this representation:
// Atlas OSS does not model them, and applying them while Atlas constructs both
// the current and desired states would create the same function twice.
func (state *State) RelationalDDL(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("nil context")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if state == nil || state.tables == nil {
		return "", errors.New("desired schema: uninitialized state")
	}
	ddl, err := entschema.DDL(ctx, entschema.DDLArgs{
		Dialect: entdialect.Postgres,
		Version: postgresVersion,
		Tables:  state.tables,
	})
	if err != nil {
		return "", fmt.Errorf("desired schema: emit PostgreSQL %s DDL: %w", postgresVersion, err)
	}
	deferredDDL, err := deferredCompositeForeignKeyDDL(
		state.tables,
		deferredSubmissionJudgeForeignKeys(),
	)
	if err != nil {
		return "", fmt.Errorf("desired schema: emit deferred foreign keys: %w", err)
	}
	return strings.TrimSpace(stripAtlasPositionDirectives(ddl)) + "\n\n" + deferredDDL + "\n", nil
}

// DDL emits the complete desired state, including reviewed triggers and role
// grants that Atlas OSS cannot model. Runtime migration verification and drift
// tests use this full representation; cmd/atlas-schema uses RelationalDDL for
// diffing.
func (state *State) DDL(ctx context.Context) (string, error) {
	ddl, err := state.RelationalDDL(ctx)
	if err != nil {
		return "", err
	}
	footerDDL, err := schemaextras.JudgifyMigrationFooterDDL("")
	if err != nil {
		return "", fmt.Errorf("desired schema: build migration footer DDL: %w", err)
	}
	return strings.TrimSpace(ddl) + "\n\n" + footerDDL + "\n", nil
}

func stripAtlasPositionDirectives(ddl string) string {
	lines := strings.Split(ddl, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if line == "-- atlas:pos" || strings.HasPrefix(line, "-- atlas:pos ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func extendContentRenderIdentity(tables []*entschema.Table) error {
	return addCompositeForeignKeys(tables, []compositeForeignKey{{
		symbol:     renderSourceIdentityForeignKey,
		table:      contentRenderArtifactsTable,
		columns:    artifactIdentityColumns,
		refTable:   contentRenderSourcesTable,
		refColumns: sourceIdentityColumns,
	}})
}

// extendProfileAttributeValueProvenance proves that both the value revision
// and its source revision belong to the same attribute definition recorded on
// the row. Ent's individual edge foreign keys cannot express this tuple.
func extendProfileAttributeValueProvenance(tables []*entschema.Table) error {
	return addCompositeForeignKeys(tables, []compositeForeignKey{
		{
			symbol:     "user_attribute_values_definition_revision_owner_fkey",
			table:      userAttributeValuesTable,
			columns:    []string{"definition_revision_id", "attribute_id"},
			refTable:   attributeDefinitionRevisionsTable,
			refColumns: []string{"id", "definition_id"},
		},
		{
			symbol:     "user_attribute_values_source_revision_owner_fkey",
			table:      userAttributeValuesTable,
			columns:    []string{"source_revision_id", "attribute_id"},
			refTable:   attributeDefinitionRevisionsTable,
			refColumns: []string{"id", "definition_id"},
		},
	})
}

// extendMaterialIdentity prevents a stable material pointer or immutable
// revision from mixing a slug, owner revision, source checksum, or artifact
// that belongs to another aggregate. Ent cannot express these exact tuples.
func extendMaterialIdentity(tables []*entschema.Table) error {
	return addCompositeForeignKeys(tables, materialIdentityForeignKeys())
}

func materialIdentityForeignKeys() []compositeForeignKey {
	sourceIdentity := []string{"id", "owner_revision_id", "source_kind", "source_checksum"}
	artifactIdentity := []string{"id", "render_source_id", "render_source_kind", "source_checksum"}
	revisionSourceIdentity := []string{
		"id", "render_source_id", "render_source_kind", "normalized_source_checksum",
	}
	return []compositeForeignKey{
		{
			symbol: "material_revisions_render_source_identity_fkey",
			table:  materialRevisionsTable,
			columns: []string{
				"render_source_id", "id", "render_source_kind", "normalized_source_checksum",
			},
			refTable: contentRenderSourcesTable, refColumns: sourceIdentity,
		},
		{
			symbol: "material_revisions_render_artifact_identity_fkey",
			table:  materialRevisionsTable,
			columns: []string{
				"render_artifact_id", "render_source_id", "render_source_kind", "normalized_source_checksum",
			},
			refTable: contentRenderArtifactsTable, refColumns: artifactIdentity,
		},
		{
			symbol:     "materials_current_slug_owner_fkey",
			table:      materialsTable,
			columns:    []string{"current_slug_id", "id", "current_slug_state"},
			refTable:   materialSlugsTable,
			refColumns: []string{"id", "material_id", "state"},
		},
		{
			symbol:     "materials_current_draft_owner_fkey",
			table:      materialsTable,
			columns:    []string{"current_draft_revision_id", "id"},
			refTable:   materialRevisionsTable,
			refColumns: []string{"id", "material_id"},
		},
		{
			symbol:     "materials_published_revision_owner_fkey",
			table:      materialsTable,
			columns:    []string{"published_revision_id", "id"},
			refTable:   materialRevisionsTable,
			refColumns: []string{"id", "material_id"},
		},
		{
			symbol: "materials_published_revision_source_fkey",
			table:  materialsTable,
			columns: []string{
				"published_revision_id", "published_render_source_id",
				"published_render_source_kind", "published_source_checksum",
			},
			refTable: materialRevisionsTable, refColumns: revisionSourceIdentity,
		},
		{
			symbol: "materials_published_artifact_source_fkey",
			table:  materialsTable,
			columns: []string{
				"published_render_artifact_id", "published_render_source_id",
				"published_render_source_kind", "published_source_checksum",
			},
			refTable: contentRenderArtifactsTable, refColumns: artifactIdentity,
		},
	}
}

// extendContestContentIdentity binds every mutable aggregate pointer to one
// exact immutable owner revision, typed source, checksum, and ready artifact.
// Ent cannot express these cross-table tuples with ordinary edges.
func extendContestContentIdentity(tables []*entschema.Table) error {
	revisionIdentity := []string{
		"id", "owner_type", "owner_id", "content_kind",
		"render_source_id", "render_source_kind", "normalized_source_checksum",
	}
	artifactIdentity := []string{"id", "render_source_id", "render_source_kind", "source_checksum"}
	foreignKeys := []compositeForeignKey{
		{
			symbol:     "contest_content_revisions_render_source_identity_fkey",
			table:      contestContentRevisionsTable,
			columns:    []string{"render_source_id", "id", "render_source_kind", "normalized_source_checksum"},
			refTable:   contentRenderSourcesTable,
			refColumns: []string{"id", "owner_revision_id", "source_kind", "source_checksum"},
		},
		{
			symbol:     "contest_content_revisions_render_artifact_identity_fkey",
			table:      contestContentRevisionsTable,
			columns:    []string{"render_artifact_id", "render_source_id", "render_source_kind", "normalized_source_checksum"},
			refTable:   contentRenderArtifactsTable,
			refColumns: artifactIdentity,
		},
		{
			symbol: "contest_announcements_draft_revision_identity_fkey",
			table:  contestAnnouncementsTable,
			columns: []string{
				"current_draft_revision_id", "current_draft_owner_type", "id", "current_draft_content_kind",
				"current_draft_render_source_id", "current_draft_render_source_kind", "current_draft_normalized_source_checksum",
			},
			refTable: contestContentRevisionsTable, refColumns: revisionIdentity,
		},
		{
			symbol:   "contest_announcements_draft_artifact_identity_fkey",
			table:    contestAnnouncementsTable,
			columns:  []string{"current_draft_render_artifact_id", "current_draft_render_source_id", "current_draft_render_source_kind", "current_draft_normalized_source_checksum"},
			refTable: contentRenderArtifactsTable, refColumns: artifactIdentity,
		},
		{
			symbol: "contest_announcements_published_revision_identity_fkey",
			table:  contestAnnouncementsTable,
			columns: []string{
				"published_revision_id", "published_owner_type", "id", "published_content_kind",
				"published_render_source_id", "published_render_source_kind", "published_normalized_source_checksum",
			},
			refTable: contestContentRevisionsTable, refColumns: revisionIdentity,
		},
		{
			symbol:   "contest_announcements_published_artifact_identity_fkey",
			table:    contestAnnouncementsTable,
			columns:  []string{"published_render_artifact_id", "published_render_source_id", "published_render_source_kind", "published_normalized_source_checksum"},
			refTable: contentRenderArtifactsTable, refColumns: artifactIdentity,
		},
		{
			symbol: "contest_clarifications_question_revision_identity_fkey",
			table:  contestClarificationsTable,
			columns: []string{
				"question_revision_id", "question_owner_type", "id", "question_content_kind",
				"question_render_source_id", "question_render_source_kind", "question_normalized_source_checksum",
			},
			refTable: contestContentRevisionsTable, refColumns: revisionIdentity,
		},
		{
			symbol:   "contest_clarifications_question_artifact_identity_fkey",
			table:    contestClarificationsTable,
			columns:  []string{"question_render_artifact_id", "question_render_source_id", "question_render_source_kind", "question_normalized_source_checksum"},
			refTable: contentRenderArtifactsTable, refColumns: artifactIdentity,
		},
		{
			symbol: "contest_clarifications_public_revision_identity_fkey",
			table:  contestClarificationsTable,
			columns: []string{
				"current_public_question_revision_id", "current_public_question_owner_type", "id", "current_public_question_content_kind",
				"current_public_question_render_source_id", "current_public_question_render_source_kind", "current_public_question_normalized_source_checksum",
			},
			refTable: contestContentRevisionsTable, refColumns: revisionIdentity,
		},
		{
			symbol:   "contest_clarifications_public_artifact_identity_fkey",
			table:    contestClarificationsTable,
			columns:  []string{"current_public_question_render_artifact_id", "current_public_question_render_source_id", "current_public_question_render_source_kind", "current_public_question_normalized_source_checksum"},
			refTable: contentRenderArtifactsTable, refColumns: artifactIdentity,
		},
		{
			symbol: "contest_clarifications_answer_revision_identity_fkey",
			table:  contestClarificationsTable,
			columns: []string{
				"current_answer_revision_id", "current_answer_owner_type", "id", "current_answer_content_kind",
				"current_answer_render_source_id", "current_answer_render_source_kind", "current_answer_normalized_source_checksum",
			},
			refTable: contestContentRevisionsTable, refColumns: revisionIdentity,
		},
		{
			symbol:   "contest_clarifications_answer_artifact_identity_fkey",
			table:    contestClarificationsTable,
			columns:  []string{"current_answer_render_artifact_id", "current_answer_render_source_id", "current_answer_render_source_kind", "current_answer_normalized_source_checksum"},
			refTable: contentRenderArtifactsTable, refColumns: artifactIdentity,
		},
		{
			symbol:     "contest_clarifications_contest_problem_identity_fkey",
			table:      contestClarificationsTable,
			columns:    []string{"contest_problem_id", "contest_id"},
			refTable:   contestProblemsTable,
			refColumns: []string{"id", "contest_id"},
		},
	}
	return addCompositeForeignKeys(tables, foreignKeys)
}

func extendProblemIdentity(tables []*entschema.Table) error {
	sourceOwnerIdentity := []string{"id", "owner_revision_id", "source_kind", "source_checksum"}
	revisionSourceIdentity := []string{"id", "render_source_id", "render_source_kind", "render_source_checksum"}
	revisionOwnerIdentity := []string{"id", "problem_id"}
	testsetOwnerIdentity := []string{"id", "problem_id"}
	groupIdentity := []string{"id", "testset_revision_id", "group_index", "stop_on_failure"}

	foreignKeys := []compositeForeignKey{
		{
			symbol: "problem_revisions_render_source_identity_fkey",
			table:  problemRevisionsTable,
			columns: []string{
				"render_source_id", "id", "render_source_kind", "render_source_checksum",
			},
			refTable: contentRenderSourcesTable, refColumns: sourceOwnerIdentity,
		},
		{
			symbol: "problem_revisions_testset_owner_fkey",
			table:  problemRevisionsTable, columns: []string{"testset_revision_id", "problem_id"},
			refTable: testsetRevisionsTable, refColumns: testsetOwnerIdentity,
		},
		{
			symbol:   "tag_revision_elements_element_revision_fkey",
			table:    tagRevisionElementsTable,
			columns:  []string{"element_id", "element_revision"},
			refTable: elementRevisionsTable, refColumns: []string{"element_id", "revision"},
		},
		{
			symbol:   "problem_revisions_difficulty_revision_fkey",
			table:    problemRevisionsTable,
			columns:  []string{"difficulty_id", "difficulty_revision"},
			refTable: difficultyRevisionsTable, refColumns: []string{"difficulty_id", "revision"},
		},
		{
			symbol:   "problem_revisions_tag_mapping_owner_fkey",
			table:    problemRevisionsTable,
			columns:  []string{"problem_id", "tag_mapping_id"},
			refTable: problemsTable, refColumns: []string{"id", "tag_mapping_id"},
		},
		{
			symbol:   "problem_revisions_element_mapping_owner_fkey",
			table:    problemRevisionsTable,
			columns:  []string{"problem_id", "element_mapping_id"},
			refTable: problemsTable, refColumns: []string{"id", "element_mapping_id"},
		},
		{
			symbol: "problems_draft_revision_source_fkey",
			table:  problemsTable,
			columns: []string{
				"draft_revision_id", "draft_render_source_id",
				"draft_render_source_kind", "draft_render_source_checksum",
			},
			refTable: problemRevisionsTable, refColumns: revisionSourceIdentity,
		},
		{
			symbol: "problems_draft_artifact_source_fkey",
			table:  problemsTable,
			columns: []string{
				"draft_render_artifact_id", "draft_render_source_id",
				"draft_render_source_kind", "draft_render_source_checksum",
			},
			refTable:   contentRenderArtifactsTable,
			refColumns: []string{"id", "render_source_id", "render_source_kind", "source_checksum"},
		},
		{
			symbol: "problems_draft_revision_owner_fkey",
			table:  problemsTable, columns: []string{"draft_revision_id", "id"},
			refTable: problemRevisionsTable, refColumns: revisionOwnerIdentity,
		},
		{
			symbol: "problems_published_revision_source_fkey",
			table:  problemsTable,
			columns: []string{
				"published_revision_id", "published_render_source_id",
				"published_render_source_kind", "published_render_source_checksum",
			},
			refTable: problemRevisionsTable, refColumns: revisionSourceIdentity,
		},
		{
			symbol: "problems_published_artifact_source_fkey",
			table:  problemsTable,
			columns: []string{
				"published_render_artifact_id", "published_render_source_id",
				"published_render_source_kind", "published_render_source_checksum",
			},
			refTable:   contentRenderArtifactsTable,
			refColumns: []string{"id", "render_source_id", "render_source_kind", "source_checksum"},
		},
		{
			symbol: "problems_published_revision_owner_fkey",
			table:  problemsTable, columns: []string{"published_revision_id", "id"},
			refTable: problemRevisionsTable, refColumns: revisionOwnerIdentity,
		},
		{
			symbol: "testcase_revisions_test_case_owner_fkey",
			table:  testcaseRevisionsTable, columns: []string{"test_case_id", "problem_id"},
			refTable: testCasesTable, refColumns: []string{"id", "problem_id"},
		},
		{
			symbol: "testcase_revisions_testset_owner_fkey",
			table:  testcaseRevisionsTable, columns: []string{"testset_revision_id", "problem_id"},
			refTable: testsetRevisionsTable, refColumns: testsetOwnerIdentity,
		},
		{
			symbol: "testcase_revisions_group_identity_fkey",
			table:  testcaseRevisionsTable,
			columns: []string{
				"group_id", "testset_revision_id", "group_index", "stop_on_failure",
			},
			refTable: testcaseGroupsTable, refColumns: groupIdentity,
		},
	}
	return addCompositeForeignKeys(tables, foreignKeys)
}

// extendContestProblemIdentity makes it impossible for a Contest snapshot to
// pair a revision or testset with a different stable Problem. Ent's individual
// UUID edges cannot express these ownership tuples.
func extendContestProblemIdentity(tables []*entschema.Table) error {
	return addCompositeForeignKeys(tables, []compositeForeignKey{
		{
			symbol:     "contest_problems_problem_revision_identity_fkey",
			table:      contestProblemsTable,
			columns:    []string{"problem_revision_id", "problem_id", "testset_revision_id"},
			refTable:   problemRevisionsTable,
			refColumns: []string{"id", "problem_id", "testset_revision_id"},
		},
		{
			symbol:     "contest_problems_testset_owner_fkey",
			table:      contestProblemsTable,
			columns:    []string{"testset_revision_id", "problem_id"},
			refTable:   testsetRevisionsTable,
			refColumns: []string{"id", "problem_id"},
		},
	})
}

func extendContestRegistrationTransitionIdentity(tables []*entschema.Table) error {
	return addCompositeForeignKeys(tables, []compositeForeignKey{{
		symbol:     "contest_registration_transitions_registration_identity_fkey",
		table:      contestRegistrationTransitionsTable,
		columns:    []string{"registration_id", "contest_id", "user_id"},
		refTable:   contestRegistrationsTable,
		refColumns: []string{"id", "contest_id", "user_id"},
	}})
}

// extendSubmissionJudgeIdentity turns every duplicated ownership fact in the
// submission execution model into a database-verified composite identity. It
// also adds two referenced identities to older immutable authoring tables;
// Ent cannot express these foreign keys without introducing weaker edges.
func extendSubmissionJudgeIdentity(tables []*entschema.Table) error {
	problemRevision, err := exactTable(tables, problemRevisionsTable)
	if err != nil {
		return err
	}
	ensureExactUniqueIndex(
		problemRevision,
		"problem_revisions_submission_spec_identity",
		[]string{"id", "problem_id", "testset_revision_id"},
	)
	testcaseRevision, err := exactTable(tables, testcaseRevisionsTable)
	if err != nil {
		return err
	}
	ensureExactUniqueIndex(
		testcaseRevision,
		"testcase_revisions_judge_result_identity",
		[]string{
			"id", "testset_revision_id", "group_id", "group_index", "order_index",
			"kind", "stop_on_failure",
		},
	)

	foreignKeys := []compositeForeignKey{
		{
			symbol:     "judge_specs_submission_owner_fkey",
			table:      judgeSpecsTable,
			columns:    []string{"submission_id", "user_id", "problem_id"},
			refTable:   submissionsTable,
			refColumns: []string{"id", "user_id", "problem_id"},
		},
		{
			symbol:     "submissions_contest_problem_snapshot_fkey",
			table:      submissionsTable,
			columns:    []string{"contest_problem_id", "contest_id", "problem_id"},
			refTable:   contestProblemsTable,
			refColumns: []string{"id", "contest_id", "problem_id"},
		},
		{
			symbol:     "judge_specs_problem_revision_identity_fkey",
			table:      judgeSpecsTable,
			columns:    []string{"problem_revision_id", "problem_id", "testset_revision_id"},
			refTable:   problemRevisionsTable,
			refColumns: []string{"id", "problem_id", "testset_revision_id"},
		},
		{
			symbol:     "judge_specs_testset_owner_fkey",
			table:      judgeSpecsTable,
			columns:    []string{"testset_revision_id", "problem_id"},
			refTable:   testsetRevisionsTable,
			refColumns: []string{"id", "problem_id"},
		},
		{
			symbol:     "judge_jobs_spec_generation_fkey",
			table:      judgeJobsTable,
			columns:    []string{"judge_spec_id", "submission_id", "generation"},
			refTable:   judgeSpecsTable,
			refColumns: []string{"id", "submission_id", "generation"},
		},
		{
			symbol:     "judge_attempts_job_generation_spec_fkey",
			table:      judgeAttemptsTable,
			columns:    []string{"job_id", "submission_id", "generation", "judge_spec_id"},
			refTable:   judgeJobsTable,
			refColumns: []string{"id", "submission_id", "generation", "judge_spec_id"},
		},
		{
			symbol: "judge_attempts_spec_provenance_fkey",
			table:  judgeAttemptsTable,
			columns: []string{
				"judge_spec_id", "runtime_profile_version", "compiler_version",
				"runtime_bootstrap_version", "supervisor_protocol_version",
				"sandbox_image_digest", "artifact_manifest_checksum",
			},
			refTable: judgeSpecsTable,
			refColumns: []string{
				"id", "runtime_profile_version", "compiler_version",
				"runtime_bootstrap_version", "supervisor_protocol_version",
				"sandbox_image_digest", "artifact_manifest_checksum",
			},
		},
		{
			symbol:     "judge_attempts_worker_boot_fkey",
			table:      judgeAttemptsTable,
			columns:    []string{"worker_id", "worker_boot_id"},
			refTable:   judgeWorkerInstancesTable,
			refColumns: []string{"id", "boot_id"},
		},
		{
			symbol: "submission_judgements_spec_generation_fkey",
			table:  submissionJudgementsTable,
			columns: []string{
				"judge_spec_id", "submission_id", "generation", "testset_revision_id",
				"problem_revision_id", "judge_spec_checksum", "reward_snapshot_checksum", "runtime_profile_version",
				"compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version",
				"sandbox_image_digest", "artifact_manifest_checksum", "release_id",
				"approved_release_manifest_checksum", "checker_implementation_checksum",
			},
			refTable: judgeSpecsTable,
			refColumns: []string{
				"id", "submission_id", "generation", "testset_revision_id",
				"problem_revision_id", "spec_checksum", "reward_snapshot_checksum", "runtime_profile_version",
				"compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version",
				"sandbox_image_digest", "artifact_manifest_checksum", "release_id",
				"approved_release_manifest_checksum", "checker_implementation_checksum",
			},
		},
		{
			symbol:     "submission_judgements_winning_attempt_fkey",
			table:      submissionJudgementsTable,
			columns:    []string{"winning_attempt_id", "submission_id", "generation", "judge_spec_id"},
			refTable:   judgeAttemptsTable,
			refColumns: []string{"id", "submission_id", "generation", "judge_spec_id"},
		},
		{
			symbol:     "submissions_active_judgement_fkey",
			table:      submissionsTable,
			columns:    []string{"active_judgement_id", "id", "active_judgement_generation", "active_verdict"},
			refTable:   submissionJudgementsTable,
			refColumns: []string{"id", "submission_id", "generation", "verdict"},
		},
		{
			symbol:     "judge_test_results_attempt_spec_fkey",
			table:      judgeTestResultsTable,
			columns:    []string{"attempt_id", "judge_spec_id"},
			refTable:   judgeAttemptsTable,
			refColumns: []string{"id", "judge_spec_id"},
		},
		{
			symbol:     "judge_test_results_spec_testset_fkey",
			table:      judgeTestResultsTable,
			columns:    []string{"judge_spec_id", "testset_revision_id"},
			refTable:   judgeSpecsTable,
			refColumns: []string{"id", "testset_revision_id"},
		},
		{
			symbol:     "judge_test_results_group_identity_fkey",
			table:      judgeTestResultsTable,
			columns:    []string{"group_id", "testset_revision_id", "group_index", "group_stop_on_failure"},
			refTable:   testcaseGroupsTable,
			refColumns: []string{"id", "testset_revision_id", "group_index", "stop_on_failure"},
		},
		{
			symbol: "judge_test_results_testcase_identity_fkey",
			table:  judgeTestResultsTable,
			columns: []string{
				"testcase_revision_id", "testset_revision_id", "group_id", "group_index",
				"case_order", "test_kind", "group_stop_on_failure",
			},
			refTable: testcaseRevisionsTable,
			refColumns: []string{
				"id", "testset_revision_id", "group_id", "group_index", "order_index",
				"kind", "stop_on_failure",
			},
		},
		{
			symbol:     "user_problem_solves_active_submission_fkey",
			table:      userProblemSolvesTable,
			columns:    []string{"active_submission_id", "user_id", "problem_id"},
			refTable:   submissionsTable,
			refColumns: []string{"id", "user_id", "problem_id"},
		},
		{
			symbol: "user_problem_solves_active_judgement_fkey",
			table:  userProblemSolvesTable,
			columns: []string{
				"active_judgement_id", "active_submission_id", "active_generation",
				"active_problem_revision_id", "reward_facts_checksum",
			},
			refTable: submissionJudgementsTable,
			refColumns: []string{
				"id", "submission_id", "generation", "problem_revision_id", "reward_snapshot_checksum",
			},
		},
		{
			symbol:     "solve_transitions_solve_owner_fkey",
			table:      solveTransitionsTable,
			columns:    []string{"solve_id", "user_id", "problem_id"},
			refTable:   userProblemSolvesTable,
			refColumns: []string{"id", "user_id", "problem_id"},
		},
		{
			symbol: "solve_transitions_from_judgement_fkey",
			table:  solveTransitionsTable,
			columns: []string{
				"from_judgement_id", "from_submission_id", "from_generation",
				"from_problem_revision_id", "from_reward_facts_checksum",
			},
			refTable: submissionJudgementsTable,
			refColumns: []string{
				"id", "submission_id", "generation", "problem_revision_id", "reward_snapshot_checksum",
			},
		},
		{
			symbol: "solve_transitions_to_judgement_fkey",
			table:  solveTransitionsTable,
			columns: []string{
				"to_judgement_id", "to_submission_id", "to_generation",
				"to_problem_revision_id", "to_reward_facts_checksum",
			},
			refTable: submissionJudgementsTable,
			refColumns: []string{
				"id", "submission_id", "generation", "problem_revision_id", "reward_snapshot_checksum",
			},
		},
		{
			symbol:     "user_problem_solves_active_transition_fkey",
			table:      userProblemSolvesTable,
			columns:    []string{"active_reward_transition_id", "id", "version"},
			refTable:   solveTransitionsTable,
			refColumns: []string{"id", "solve_id", "to_version"},
		},
	}
	if err := addCompositeForeignKeys(tables, foreignKeys); err != nil {
		return err
	}
	return addPrimaryKeyForeignKeys(tables, []compositeForeignKey{
		{
			symbol: "user_problem_solves_user_fkey",
			table:  userProblemSolvesTable, columns: []string{"user_id"},
			refTable: usersTable, refColumns: []string{"id"},
		},
		{
			symbol: "user_problem_solves_problem_fkey",
			table:  userProblemSolvesTable, columns: []string{"problem_id"},
			refTable: problemsTable, refColumns: []string{"id"},
		},
	})
}

// deferredSubmissionJudgeForeignKeys returns self-references whose target is a
// composite unique index. PostgreSQL cannot create these constraints inline
// because Ent emits the target index after CREATE TABLE. RelationalDDL appends
// them as ALTER TABLE statements after every table and index exists.
func deferredSubmissionJudgeForeignKeys() []compositeForeignKey {
	return []compositeForeignKey{
		{
			symbol:     "solve_transitions_from_reward_grant_fkey",
			table:      solveTransitionsTable,
			columns:    []string{"from_reward_grant_source_id", "solve_id"},
			refTable:   solveTransitionsTable,
			refColumns: []string{"id", "solve_id"},
		},
		{
			symbol:     "solve_transitions_to_reward_grant_fkey",
			table:      solveTransitionsTable,
			columns:    []string{"to_reward_grant_source_id", "solve_id"},
			refTable:   solveTransitionsTable,
			refColumns: []string{"id", "solve_id"},
		},
	}
}

func deferredCompositeForeignKeyDDL(tables []*entschema.Table, values []compositeForeignKey) (string, error) {
	var ddl strings.Builder
	for index, value := range values {
		if value.symbol == "" || len(value.columns) < 2 || len(value.columns) != len(value.refColumns) {
			return "", fmt.Errorf("invalid deferred composite foreign key specification %q", value.symbol)
		}
		table, err := exactTable(tables, value.table)
		if err != nil {
			return "", err
		}
		if _, err := exactColumns(table, value.columns); err != nil {
			return "", err
		}
		refTable, err := exactTable(tables, value.refTable)
		if err != nil {
			return "", err
		}
		if _, err := exactColumns(refTable, value.refColumns); err != nil {
			return "", err
		}
		if err := validateExactUniqueIndex(refTable, value.refColumns); err != nil {
			return "", err
		}
		if index > 0 {
			ddl.WriteString("\n")
		}
		fmt.Fprintf(
			&ddl,
			"ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s) ON UPDATE NO ACTION ON DELETE NO ACTION;\n",
			quotePostgreSQLIdentifier(value.table),
			quotePostgreSQLIdentifier(value.symbol),
			quotePostgreSQLIdentifiers(value.columns),
			quotePostgreSQLIdentifier(value.refTable),
			quotePostgreSQLIdentifiers(value.refColumns),
		)
	}
	return strings.TrimSpace(ddl.String()), nil
}

func quotePostgreSQLIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func quotePostgreSQLIdentifiers(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = quotePostgreSQLIdentifier(value)
	}
	return strings.Join(quoted, ", ")
}

func ensureExactUniqueIndex(table *entschema.Table, name string, columns []string) {
	for _, index := range table.Indexes {
		if index != nil && index.Unique && equalColumnNames(index.Columns, columns) {
			return
		}
	}
	table.AddIndex(name, true, columns)
}

func addPrimaryKeyForeignKeys(tables []*entschema.Table, values []compositeForeignKey) error {
	for _, value := range values {
		if value.symbol == "" || len(value.columns) != 1 || len(value.refColumns) != 1 {
			return fmt.Errorf("invalid primary-key foreign key specification %q", value.symbol)
		}
		table, err := exactTable(tables, value.table)
		if err != nil {
			return err
		}
		refTable, err := exactTable(tables, value.refTable)
		if err != nil {
			return err
		}
		columns, err := exactColumns(table, value.columns)
		if err != nil {
			return err
		}
		refColumns, err := exactColumns(refTable, value.refColumns)
		if err != nil {
			return err
		}
		if !equalColumnNames(refTable.PrimaryKey, value.refColumns) {
			return fmt.Errorf(
				"expected primary key on table %q for columns (%s)",
				refTable.Name,
				strings.Join(value.refColumns, ", "),
			)
		}
		for _, foreignKey := range table.ForeignKeys {
			if foreignKey == nil {
				return fmt.Errorf("table %q contains a nil foreign key", table.Name)
			}
			if foreignKey.Symbol == value.symbol {
				return fmt.Errorf("foreign key symbol %q already exists", value.symbol)
			}
		}
		table.AddForeignKey(&entschema.ForeignKey{
			Symbol: value.symbol, Columns: columns,
			RefTable: refTable, RefColumns: refColumns,
			OnUpdate: entschema.NoAction, OnDelete: entschema.NoAction,
		})
	}
	return nil
}

type compositeForeignKey struct {
	symbol     string
	table      string
	columns    []string
	refTable   string
	refColumns []string
}

func addCompositeForeignKeys(tables []*entschema.Table, values []compositeForeignKey) error {
	seenSymbols := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value.symbol == "" || len(value.columns) < 2 ||
			len(value.columns) != len(value.refColumns) {
			return fmt.Errorf("invalid composite foreign key specification %q", value.symbol)
		}
		if _, duplicate := seenSymbols[value.symbol]; duplicate {
			return fmt.Errorf("duplicate composite foreign key symbol %q", value.symbol)
		}
		seenSymbols[value.symbol] = struct{}{}

		refTable, err := exactTable(tables, value.refTable)
		if err != nil {
			return err
		}
		table, err := exactTable(tables, value.table)
		if err != nil {
			return err
		}
		columns, err := exactColumns(table, value.columns)
		if err != nil {
			return err
		}
		refColumns, err := exactColumns(refTable, value.refColumns)
		if err != nil {
			return err
		}
		if err := validateExactUniqueIndex(refTable, value.refColumns); err != nil {
			return err
		}
		for _, foreignKey := range table.ForeignKeys {
			if foreignKey == nil {
				return fmt.Errorf("table %q contains a nil foreign key", table.Name)
			}
			if foreignKey.Symbol == value.symbol {
				return fmt.Errorf("foreign key symbol %q already exists", value.symbol)
			}
			if foreignKey.RefTable == refTable &&
				equalColumnNames(foreignKey.Columns, value.columns) &&
				equalColumnNames(foreignKey.RefColumns, value.refColumns) {
				return fmt.Errorf("composite foreign key tuple already exists as %q", foreignKey.Symbol)
			}
		}
		table.AddForeignKey(&entschema.ForeignKey{
			Symbol:     value.symbol,
			Columns:    columns,
			RefTable:   refTable,
			RefColumns: refColumns,
			OnUpdate:   entschema.NoAction,
			OnDelete:   entschema.NoAction,
		})
	}
	return nil
}

func exactTable(tables []*entschema.Table, name string) (*entschema.Table, error) {
	matches := make([]*entschema.Table, 0, 1)
	for _, table := range tables {
		if table != nil && table.Name == name {
			matches = append(matches, table)
		}
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("expected exactly one table %q, found %d", name, len(matches))
	}
	return matches[0], nil
}

func exactColumns(table *entschema.Table, names []string) ([]*entschema.Column, error) {
	columns := make([]*entschema.Column, 0, len(names))
	for _, name := range names {
		matches := make([]*entschema.Column, 0, 1)
		for _, column := range table.Columns {
			if column != nil && column.Name == name {
				matches = append(matches, column)
			}
		}
		if len(matches) != 1 {
			return nil, fmt.Errorf(
				"expected exactly one column %q on table %q, found %d",
				name,
				table.Name,
				len(matches),
			)
		}
		columns = append(columns, matches[0])
	}
	return columns, nil
}

func validateExactUniqueIndex(table *entschema.Table, names []string) error {
	var matches int
	for _, index := range table.Indexes {
		if index != nil && index.Unique && equalColumnNames(index.Columns, names) {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf(
			"expected exactly one unique index on table %q for columns (%s), found %d",
			table.Name,
			strings.Join(names, ", "),
			matches,
		)
	}
	return nil
}

func equalColumnNames(columns []*entschema.Column, names []string) bool {
	if len(columns) != len(names) {
		return false
	}
	for index, name := range names {
		if columns[index] == nil || columns[index].Name != name {
			return false
		}
	}
	return true
}
