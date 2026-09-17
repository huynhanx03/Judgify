package schema

import (
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

var judgeAttemptStates = []string{
	"created", "preparing", "compiling", "running", "finalizing", "succeeded",
	"infrastructure_failed", "lease_lost", "cancelled",
}

// JudgeAttempt records one fenced infrastructure execution. Contestant
// outcomes are result data on a succeeded attempt, never attempt failure state.
type JudgeAttempt struct{ ent.Schema }

func (JudgeAttempt) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (JudgeAttempt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("job_id", uuid.UUID{}).Immutable(),
		field.UUID("submission_id", uuid.UUID{}).Immutable(),
		field.Int64("generation").Min(1).Immutable(),
		field.UUID("judge_spec_id", uuid.UUID{}).Immutable(),
		field.Uint32("attempt_number").Min(1).Max(maximumJudgeAttempts).Immutable(),
		field.String("lease_owner").NotEmpty().MaxLen(maximumJudgeOwnerBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.Bytes("lease_token_hash").NotEmpty().Sensitive().Immutable(),
		field.UUID("worker_id", uuid.UUID{}).Immutable(),
		field.UUID("worker_boot_id", uuid.UUID{}).Immutable(),
		field.String("origin_cid").NotEmpty().MaxLen(maximumSubmissionCorrelationIDBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.Enum("state").Values(judgeAttemptStates...).Default("created"),
		field.String("failure_classification").MaxLen(maximumJudgeErrorCodeBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Optional().Nillable(),
		field.String("failure_code").MaxLen(maximumJudgeErrorCodeBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Optional().Nillable(),
		field.String("failure_summary").MaxLen(maximumJudgeDiagnosticBytes).
			Optional().Nillable().Sensitive(),
		field.Time("started_at").Immutable(),
		field.Time("heartbeat_at").Optional().Nillable(),
		field.Time("finished_at").Optional().Nillable(),
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
	}
}

func (JudgeAttempt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("job_id", "attempt_number").Unique(),
		index.Fields("id", "job_id", "submission_id", "generation", "judge_spec_id").
			Unique().StorageKey("judge_attempts_job_identity"),
		index.Fields("id", "submission_id", "generation", "judge_spec_id").
			Unique().StorageKey("judge_attempts_generation_identity"),
		index.Fields("id", "judge_spec_id").Unique(),
		index.Fields("lease_token_hash").Unique(),
		index.Fields("job_id", "state", "attempt_number"),
		index.Fields("worker_id", "state", "heartbeat_at"),
		index.Fields("worker_id", "worker_boot_id", "attempt_number"),
		index.Fields("submission_id", "generation", "attempt_number"),
	}
}

func (JudgeAttempt) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"judge_attempt_state_closed": `state IN ('created', 'preparing', 'compiling', 'running', 'finalizing', 'succeeded', 'infrastructure_failed', 'lease_lost', 'cancelled')`,
			"judge_attempt_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
				AND job_id <> ` + problemNilUUIDSQL + `
				AND submission_id <> ` + problemNilUUIDSQL + `
				AND judge_spec_id <> ` + problemNilUUIDSQL + `
				AND worker_id <> ` + problemNilUUIDSQL + `
				AND worker_boot_id <> ` + problemNilUUIDSQL,
			"judge_attempt_number_bound": fmt.Sprintf(
				"generation >= 1 AND attempt_number BETWEEN 1 AND %d",
				maximumJudgeAttempts,
			),
			"judge_attempt_lease_hash_shape": `octet_length(lease_token_hash) = 32
				AND lease_token_hash <> ` + problemZeroDigestSQL,
			"judge_attempt_failure_metadata": `(
				state IN ('created', 'preparing', 'compiling', 'running', 'finalizing', 'succeeded')
				AND failure_classification IS NULL
				AND failure_code IS NULL
				AND failure_summary IS NULL
			) OR (
				state IN ('infrastructure_failed', 'lease_lost', 'cancelled')
				AND failure_classification IS NOT NULL
				AND failure_code IS NOT NULL
				AND failure_summary IS NOT NULL
				AND octet_length(failure_classification) BETWEEN 1 AND 64
				AND octet_length(failure_code) BETWEEN 1 AND 64
				AND octet_length(failure_summary) BETWEEN 1 AND 16384
				AND failure_classification ~ '^[a-z][a-z0-9._-]{0,63}$'
				AND failure_code ~ '^[a-z][a-z0-9._-]{0,63}$'
			)`,
			"judge_attempt_terminal_timestamp": `(
				state IN ('created', 'preparing', 'compiling', 'running', 'finalizing')
				AND finished_at IS NULL
			) OR (
				state IN ('succeeded', 'infrastructure_failed', 'lease_lost', 'cancelled')
				AND finished_at IS NOT NULL
				AND finished_at >= started_at
			)`,
			"judge_attempt_heartbeat_order": "heartbeat_at IS NULL OR heartbeat_at >= started_at",
			"judge_attempt_pinned_digests": `sandbox_image_digest ~ '^sha256:[0-9a-f]{64}$'
				AND artifact_manifest_checksum ~ '^sha256:[0-9a-f]{64}$'`,
		}),
	}
}
