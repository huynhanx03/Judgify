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

var judgeJobStates = []string{
	"ready", "leased", "retry_wait", "done", "cancelled", "dead",
}

// JudgeJob is the durable scheduling record for exactly one submission
// generation. Raw lease tokens exist only in worker memory; storage contains
// their unique SHA-256 hashes.
type JudgeJob struct{ ent.Schema }

func (JudgeJob) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (JudgeJob) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("submission_id", uuid.UUID{}).Immutable(),
		field.Int64("generation").Min(1).Immutable(),
		field.UUID("judge_spec_id", uuid.UUID{}).Immutable(),
		field.Enum("lane").Values("contest", "regular", "maintenance").Immutable(),
		field.Int("base_priority").Min(-maximumJudgePriority).Max(maximumJudgePriority).Default(0),
		field.Enum("state").Values(judgeJobStates...).Default("ready"),
		field.Time("available_at"),
		field.Uint32("attempts").Default(0),
		field.Uint32("max_attempts").Min(1).Max(maximumJudgeAttempts).Default(5).Immutable(),
		field.String("lease_owner").MaxLen(maximumJudgeOwnerBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Optional().Nillable(),
		field.Bytes("lease_token_hash").Sensitive().Optional().Nillable(),
		field.Time("lease_until").Optional().Nillable(),
		field.Time("heartbeat_at").Optional().Nillable(),
		field.String("last_error_code").MaxLen(maximumJudgeErrorCodeBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Optional().Nillable(),
		field.String("last_error_summary").MaxLen(maximumJudgeDiagnosticBytes).
			Optional().Nillable().Sensitive(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("dead_at").Optional().Nillable(),
		field.Time("cancelled_at").Optional().Nillable(),
		field.String("origin_cid").NotEmpty().MaxLen(maximumSubmissionCorrelationIDBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
	}
}

func (JudgeJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("submission_id", "generation").Unique(),
		index.Fields("id", "submission_id", "generation", "judge_spec_id").Unique(),
		index.Fields("judge_spec_id").Unique(),
		index.Fields("lease_token_hash").Unique(),
		index.Fields("state", "lane", "available_at", "base_priority", "created_at", "id"),
		index.Fields("state", "lease_until"),
		index.Fields("submission_id", "created_at", "id"),
	}
}

func (JudgeJob) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"judge_job_state_closed": "state IN ('ready', 'leased', 'retry_wait', 'done', 'cancelled', 'dead')",
			"judge_job_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
				AND submission_id <> ` + problemNilUUIDSQL + `
				AND judge_spec_id <> ` + problemNilUUIDSQL,
			"judge_job_generation_and_attempt_bounds": fmt.Sprintf(
				"generation >= 1 AND attempts BETWEEN 0 AND max_attempts AND max_attempts BETWEEN 1 AND %d",
				maximumJudgeAttempts,
			),
			"judge_job_priority_bound": fmt.Sprintf(
				"base_priority BETWEEN %d AND %d",
				-maximumJudgePriority,
				maximumJudgePriority,
			),
			"judge_job_lease_hash_shape": `lease_token_hash IS NULL OR (
				octet_length(lease_token_hash) = 32
				AND lease_token_hash <> ` + problemZeroDigestSQL + `
			)`,
			"judge_job_lease_complete": `(
				state = 'leased'
				AND lease_owner IS NOT NULL
				AND lease_token_hash IS NOT NULL
				AND lease_until IS NOT NULL
				AND heartbeat_at IS NOT NULL
			) OR (
				state <> 'leased'
				AND lease_owner IS NULL
				AND lease_token_hash IS NULL
				AND lease_until IS NULL
				AND heartbeat_at IS NULL
			)`,
			"judge_job_state_timestamps": `(
				state IN ('ready', 'leased', 'retry_wait')
				AND completed_at IS NULL AND dead_at IS NULL AND cancelled_at IS NULL
			) OR (
				state = 'done'
				AND completed_at IS NOT NULL AND dead_at IS NULL AND cancelled_at IS NULL
			) OR (
				state = 'dead'
				AND completed_at IS NULL AND dead_at IS NOT NULL AND cancelled_at IS NULL
			) OR (
				state = 'cancelled'
				AND completed_at IS NULL AND dead_at IS NULL AND cancelled_at IS NOT NULL
			)`,
			"judge_job_started_before_terminal": `(started_at IS NULL OR started_at >= created_at)
				AND (completed_at IS NULL OR (started_at IS NOT NULL AND completed_at >= started_at))
				AND (dead_at IS NULL OR dead_at >= created_at)
				AND (cancelled_at IS NULL OR cancelled_at >= created_at)`,
			"judge_job_error_metadata_complete": `(
				last_error_code IS NULL AND last_error_summary IS NULL
			) OR (
				last_error_code IS NOT NULL AND last_error_summary IS NOT NULL
				AND octet_length(last_error_code) BETWEEN 1 AND 64
				AND octet_length(last_error_summary) BETWEEN 1 AND 16384
				AND last_error_code ~ '^[a-z][a-z0-9._-]{0,63}$'
			)`,
		}),
	}
}
