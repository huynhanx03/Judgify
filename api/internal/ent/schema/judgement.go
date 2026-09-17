package schema

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

// Judgement is immutable terminal history for one generation. A system-error
// judgement is operational history and is never eligible to become active.
type Judgement struct{ ent.Schema }

func (Judgement) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (Judgement) Fields() []ent.Field {
	return append(submissionImmutableRecordFields(),
		field.UUID("submission_id", uuid.UUID{}).Immutable(),
		field.Int64("generation").Min(1).Immutable(),
		field.UUID("judge_spec_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_revision_id", uuid.UUID{}).Immutable(),
		field.UUID("testset_revision_id", uuid.UUID{}).Immutable(),
		field.Enum("verdict").Values(judgementVerdicts...).Immutable(),
		field.Int("passed_count").Min(0).Max(problemcontract.MaximumTestCases).Immutable(),
		field.Int("total_count").Min(0).Max(problemcontract.MaximumTestCases).Immutable(),
		field.Int("maximum_time_ms").Min(0).Optional().Nillable().Immutable(),
		field.Int("maximum_memory_kb").Min(0).Optional().Nillable().Immutable(),
		field.String("compile_diagnostic_summary").MaxLen(maximumJudgeDiagnosticBytes).
			Optional().Nillable().Sensitive().Immutable(),
		field.Bytes("judge_spec_checksum").NotEmpty().Immutable(),
		field.JSON("reward_snapshot", json.RawMessage{}).
			SchemaType(submissionJSONSchemaType()).Immutable(),
		field.Bytes("reward_snapshot_checksum").NotEmpty().Immutable(),
		field.String("runtime_profile_version").NotEmpty().MaxLen(maximumJudgeProtocolBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("compiler_version").NotEmpty().MaxLen(maximumJudgeProtocolBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("runtime_bootstrap_version").NotEmpty().MaxLen(maximumJudgeProtocolBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("supervisor_protocol_version").NotEmpty().MaxLen(maximumJudgeProtocolBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("sandbox_image_digest").NotEmpty().MaxLen(maximumJudgeDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("artifact_manifest_checksum").NotEmpty().MaxLen(maximumJudgeDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("release_id").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("approved_release_manifest_checksum").NotEmpty().MaxLen(maximumJudgeDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("checker_implementation_checksum").NotEmpty().MaxLen(maximumJudgeDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.UUID("winning_attempt_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("completed_at").Immutable(),
		field.String("origin_cid").NotEmpty().MaxLen(maximumSubmissionCorrelationIDBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
	)
}

func (Judgement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("submission_id", "generation").Unique(),
		index.Fields("id", "submission_id", "generation", "verdict").Unique(),
		index.Fields("id", "submission_id", "generation", "problem_revision_id").Unique(),
		index.Fields(
			"id", "submission_id", "generation", "problem_revision_id", "reward_snapshot_checksum",
		).Unique().StorageKey("submission_judgements_reward_identity"),
		index.Fields("judge_spec_id").Unique(),
		index.Fields("winning_attempt_id").Unique(),
		index.Fields("submission_id", "completed_at", "id"),
		index.Fields("verdict", "completed_at", "id"),
	}
}

func (Judgement) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		&entsql.Annotation{Table: "submission_judgements", Checks: map[string]string{
			"submission_judgement_verdict_closed": `verdict IN ('accepted', 'wrong_answer', 'compile_error', 'runtime_error', 'time_limit_exceeded', 'memory_limit_exceeded', 'process_limit_exceeded', 'output_limit_exceeded', 'system_error')`,
			"submission_judgement_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
				AND submission_id <> ` + problemNilUUIDSQL + `
				AND judge_spec_id <> ` + problemNilUUIDSQL + `
				AND problem_revision_id <> ` + problemNilUUIDSQL + `
				AND testset_revision_id <> ` + problemNilUUIDSQL + `
				AND (winning_attempt_id IS NULL OR winning_attempt_id <> ` + problemNilUUIDSQL + `)`,
			"submission_judgement_generation_bound": "generation >= 1",
			"submission_judgement_count_bounds": fmt.Sprintf(
				"passed_count BETWEEN 0 AND total_count AND total_count BETWEEN 0 AND %d",
				problemcontract.MaximumTestCases,
			),
			"submission_judgement_accepted_counts":  "verdict <> 'accepted' OR (total_count > 0 AND passed_count = total_count)",
			"submission_judgement_attempt_complete": "verdict = 'system_error' OR winning_attempt_id IS NOT NULL",
			"submission_judgement_checksum_shapes": `octet_length(judge_spec_checksum) = 32
				AND judge_spec_checksum <> ` + problemZeroDigestSQL + `
				AND octet_length(reward_snapshot_checksum) = 32
				AND reward_snapshot_checksum <> ` + problemZeroDigestSQL,
			"submission_judgement_reward_bound": fmt.Sprintf(
				"jsonb_typeof(reward_snapshot) = 'object' AND octet_length(reward_snapshot::text) <= %d",
				maximumJudgeSpecJSONBytes,
			),
			"submission_judgement_diagnostic_bound": fmt.Sprintf(
				"compile_diagnostic_summary IS NULL OR octet_length(compile_diagnostic_summary) BETWEEN 1 AND %d",
				maximumJudgeDiagnosticBytes,
			),
			"submission_judgement_resource_bounds": fmt.Sprintf(
				`(maximum_time_ms IS NULL OR maximum_time_ms BETWEEN 0 AND %d)
				AND (maximum_memory_kb IS NULL OR maximum_memory_kb BETWEEN 0 AND %d)`,
				maximumAttemptWallTimeMilliseconds,
				problemcontract.MaximumMemoryLimitKilobytes,
			),
			"submission_judgement_pinned_digests": `sandbox_image_digest ~ '^sha256:[0-9a-f]{64}$'
				AND artifact_manifest_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND approved_release_manifest_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND checker_implementation_checksum ~ '^sha256:[0-9a-f]{64}$'`,
		}},
	}
}
