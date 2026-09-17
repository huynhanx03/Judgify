package schema

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	"github.com/huynhanx03/judgify/pkg/judging"
)

const (
	maximumJudgeSpecJSONBytes        = 64 << 10
	maximumCompileTimeMilliseconds   = 10 * 60 * 1_000
	maximumArtifactManifestJSONBytes = 32 << 10
)

// JudgeSpec is the immutable execution and reward contract for one submission
// generation. Workers never reconstruct this snapshot from mutable Problems.
type JudgeSpec struct{ ent.Schema }

func (JudgeSpec) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (JudgeSpec) Fields() []ent.Field {
	return append(submissionImmutableRecordFields(),
		field.UUID("submission_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.Int64("generation").Min(1).Immutable(),
		field.UUID("problem_revision_id", uuid.UUID{}).Immutable(),
		field.UUID("testset_revision_id", uuid.UUID{}).Immutable(),
		field.Enum("language").Values(judging.LanguageValues()...).Immutable(),
		field.String("runtime_key").NotEmpty().MaxLen(64).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).Immutable(),
		field.String("runtime_profile_version").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("compiler_version").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("runtime_bootstrap_version").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("sandbox_image_digest").NotEmpty().MaxLen(maximumJudgeDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("compile_command_id").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("run_command_id").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("artifact_manifest_version").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("artifact_manifest_checksum").NotEmpty().MaxLen(maximumJudgeDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.JSON("artifact_manifest", json.RawMessage{}).
			SchemaType(submissionJSONSchemaType()).Immutable(),
		field.String("supervisor_protocol_version").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("checker_key").NotEmpty().MaxLen(problemcontract.MaximumCheckerKeyBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).Immutable(),
		field.String("checker_version").NotEmpty().MaxLen(problemcontract.MaximumCheckerVersionBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).Immutable(),
		field.JSON("checker_config", json.RawMessage{}).
			SchemaType(submissionJSONSchemaType()).Immutable(),
		field.String("release_id").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("approved_release_manifest_checksum").NotEmpty().MaxLen(maximumJudgeDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("checker_implementation_checksum").NotEmpty().MaxLen(maximumJudgeDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("capability_profile_hash").NotEmpty().MaxLen(maximumJudgeDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.JSON("reward_snapshot", json.RawMessage{}).
			SchemaType(submissionJSONSchemaType()).Immutable(),
		field.Bytes("reward_snapshot_checksum").NotEmpty().Immutable(),
		field.Int("cpu_time_limit_ms").
			Min(problemcontract.MinimumCPUTimeLimitMilliseconds).
			Max(problemcontract.MaximumCPUTimeLimitMilliseconds).Immutable(),
		field.Int("wall_time_limit_ms").
			Min(problemcontract.MinimumCPUTimeLimitMilliseconds).
			Max(problemcontract.MaximumWallTimeLimitMilliseconds).Immutable(),
		field.Int("memory_limit_kb").
			Min(problemcontract.MinimumMemoryLimitKilobytes).
			Max(problemcontract.MaximumMemoryLimitKilobytes).Immutable(),
		field.Int("output_limit_bytes").
			Min(problemcontract.MinimumOutputLimitBytes).
			Max(problemcontract.MaximumOutputLimitBytes).Immutable(),
		field.Int("process_limit").
			Min(problemcontract.MinimumProcessLimit).
			Max(problemcontract.MaximumProcessLimit).Immutable(),
		field.Int("compile_cpu_time_limit_ms").Min(1).Max(maximumCompileTimeMilliseconds).Immutable(),
		field.Int("compile_wall_time_limit_ms").Min(1).Max(maximumCompileTimeMilliseconds).Immutable(),
		field.Int("total_attempt_wall_time_limit_ms").Min(1).Max(maximumAttemptWallTimeMilliseconds).Immutable(),
		field.Bytes("source_sha256").NotEmpty().Immutable(),
		field.String("judge_spec_protocol").NotEmpty().MaxLen(maximumJudgeProtocolBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.Bytes("spec_checksum").NotEmpty().Immutable(),
	)
}

func (JudgeSpec) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("submission_id", "generation").Unique(),
		index.Fields("id", "submission_id", "generation").Unique(),
		index.Fields("id", "submission_id", "generation", "testset_revision_id").Unique(),
		index.Fields(
			"id", "submission_id", "generation", "testset_revision_id", "problem_revision_id",
			"spec_checksum", "reward_snapshot_checksum", "runtime_profile_version", "compiler_version",
			"runtime_bootstrap_version", "supervisor_protocol_version",
			"sandbox_image_digest", "artifact_manifest_checksum",
			"release_id", "approved_release_manifest_checksum", "checker_implementation_checksum",
		).Unique().StorageKey("judge_specs_judgement_identity"),
		index.Fields(
			"id", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version",
			"supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum",
		).Unique().StorageKey("judge_specs_attempt_provenance_identity"),
		index.Fields("id", "testset_revision_id").Unique(),
		index.Fields("submission_id", "user_id", "problem_id", "generation"),
		index.Fields("problem_revision_id", "testset_revision_id"),
		index.Fields("capability_profile_hash", "runtime_key"),
		index.Fields("spec_checksum"),
	}
}

func (JudgeSpec) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"judge_spec_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
				AND submission_id <> ` + problemNilUUIDSQL + `
				AND user_id <> ` + problemNilUUIDSQL + `
				AND problem_id <> ` + problemNilUUIDSQL + `
				AND problem_revision_id <> ` + problemNilUUIDSQL + `
				AND testset_revision_id <> ` + problemNilUUIDSQL,
			"judge_spec_generation_bound": "generation >= 1",
			"judge_spec_checksum_shapes": `octet_length(source_sha256) = 32
				AND source_sha256 <> ` + problemZeroDigestSQL + `
				AND octet_length(reward_snapshot_checksum) = 32
				AND reward_snapshot_checksum <> ` + problemZeroDigestSQL + `
				AND octet_length(spec_checksum) = 32
				AND spec_checksum <> ` + problemZeroDigestSQL,
			"judge_spec_pinned_digests": `sandbox_image_digest ~ '^sha256:[0-9a-f]{64}$'
				AND artifact_manifest_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND approved_release_manifest_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND checker_implementation_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND capability_profile_hash ~ '^sha256:[0-9a-f]{64}$'`,
			"judge_spec_identifiers": `runtime_key ~ '^[a-z0-9][a-z0-9._-]{0,63}$'
				AND runtime_profile_version ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND compile_command_id ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND run_command_id ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND artifact_manifest_version ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND supervisor_protocol_version ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND checker_key ~ '^[a-z0-9][a-z0-9._-]{0,63}$'
				AND checker_version ~ '^[a-z0-9][a-z0-9._-]{0,63}$'
				AND release_id ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND judge_spec_protocol ~ '^[a-z0-9][a-z0-9._-]{0,127}$'`,
			"judge_spec_json_bounds": fmt.Sprintf(
				`jsonb_typeof(checker_config) = 'object'
				AND octet_length(checker_config::text) <= %d
				AND jsonb_typeof(reward_snapshot) = 'object'
				AND octet_length(reward_snapshot::text) <= %d
				AND jsonb_typeof(artifact_manifest) = 'array'
				AND jsonb_array_length(artifact_manifest) > 0
				AND octet_length(artifact_manifest::text) <= %d`,
				problemcontract.MaximumCheckerConfigBytes,
				maximumJudgeSpecJSONBytes,
				maximumArtifactManifestJSONBytes,
			),
			"judge_spec_limit_bounds": fmt.Sprintf(
				`cpu_time_limit_ms BETWEEN %d AND %d
				AND wall_time_limit_ms BETWEEN %d AND %d
				AND wall_time_limit_ms >= cpu_time_limit_ms
				AND memory_limit_kb BETWEEN %d AND %d
				AND output_limit_bytes BETWEEN %d AND %d
				AND process_limit BETWEEN %d AND %d
				AND compile_cpu_time_limit_ms BETWEEN 1 AND %d
				AND compile_wall_time_limit_ms BETWEEN compile_cpu_time_limit_ms AND %d
				AND total_attempt_wall_time_limit_ms BETWEEN compile_wall_time_limit_ms AND %d`,
				problemcontract.MinimumCPUTimeLimitMilliseconds,
				problemcontract.MaximumCPUTimeLimitMilliseconds,
				problemcontract.MinimumCPUTimeLimitMilliseconds,
				problemcontract.MaximumWallTimeLimitMilliseconds,
				problemcontract.MinimumMemoryLimitKilobytes,
				problemcontract.MaximumMemoryLimitKilobytes,
				problemcontract.MinimumOutputLimitBytes,
				problemcontract.MaximumOutputLimitBytes,
				problemcontract.MinimumProcessLimit,
				problemcontract.MaximumProcessLimit,
				maximumCompileTimeMilliseconds,
				maximumCompileTimeMilliseconds,
				maximumAttemptWallTimeMilliseconds,
			),
		}),
	}
}

func submissionImmutableRecordFields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func submissionJSONSchemaType() map[string]string {
	return map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}
}
