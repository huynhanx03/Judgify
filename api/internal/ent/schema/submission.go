package schema

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
	"github.com/huynhanx03/judgify/pkg/judging"
)

// Submission holds the schema definition for the Submission entity.
type Submission struct {
	ent.Schema
}

const (
	maximumSubmissionIdempotencyKeyBytes = 128
	maximumSubmissionCorrelationIDBytes  = 128
	maximumSubmissionRuntimeKeyBytes     = 64
	maximumJudgeDiagnosticBytes          = 16 << 10
	maximumJudgeErrorCodeBytes           = 64
	maximumJudgeOwnerBytes               = 128
	maximumJudgeWorkerIDBytes            = 128
	maximumJudgeProtocolBytes            = 128
	maximumJudgeDigestTextBytes          = 71
	maximumJudgeAttempts                 = 100
	maximumJudgePriority                 = 1_000
	maximumAttemptWallTimeMilliseconds   = 24 * 60 * 60 * 1_000
)

var (
	submissionPhases = []string{
		"queued", "claimed", "compiling", "running", "finalizing", "terminal", "cancelled",
	}
	contestantVerdicts = []string{
		"accepted", "wrong_answer", "compile_error", "runtime_error",
		"time_limit_exceeded", "memory_limit_exceeded", "process_limit_exceeded",
		"output_limit_exceeded",
	}
	judgementVerdicts = append(append([]string(nil), contestantVerdicts...), "system_error")
)

// Mixin of the Submission.
func (Submission) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Submission.
func (Submission) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("contest_problem_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int64("contest_scoring_policy_version").Min(1).Optional().Nillable().Immutable(),
		field.Time("submitted_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
		field.Int64("contest_elapsed_ms").Min(0).Optional().Nillable().Immutable(),
		field.Int64("contest_submission_sequence").Min(1).Optional().Nillable().Immutable(),
		field.Enum("language").
			Values(judging.LanguageValues()...).
			Immutable(),
		field.String("runtime_key").
			NotEmpty().
			MaxLen(maximumSubmissionRuntimeKeyBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Immutable(),
		field.Text("source_code").
			NotEmpty().
			Immutable(),
		field.Bytes("source_sha256").NotEmpty().Immutable(),
		field.Int64("version").Min(1).Default(1),
		field.Int64("latest_generation").Min(1).Default(1),
		field.Enum("latest_generation_phase").
			Values(submissionPhases...).
			Default("queued"),
		field.Enum("latest_generation_outcome").
			Values(append(append([]string(nil), judgementVerdicts...), "cancelled")...).
			Optional().
			Nillable(),
		field.UUID("active_judgement_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("active_judgement_generation").Min(1).Optional().Nillable(),
		field.Enum("active_verdict").
			Values(contestantVerdicts...).
			Optional().
			Nillable(),
		field.String("idempotency_key").
			NotEmpty().
			MaxLen(maximumSubmissionIdempotencyKeyBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.Bytes("request_hash").NotEmpty().Immutable(),
		field.String("origin_cid").
			NotEmpty().
			MaxLen(maximumSubmissionCorrelationIDBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.Time("latest_generation_queued_at").
			Default(func() time.Time { return time.Now().UTC() }),
		field.Time("latest_generation_started_at").Optional().Nillable(),
		field.Time("latest_generation_completed_at").Optional().Nillable(),
		field.Enum("status").
			Values(
				"pending",
				"judging",
				"accepted",
				"wrong_answer",
				"time_limit_exceeded",
				"memory_limit_exceeded",
				"process_limit_exceeded",
				"output_limit_exceeded",
				"runtime_error",
				"compile_error",
				"internal_error",
			).
			Default("pending"),
		field.Time("judged_at").Optional().Nillable(),
		field.Int64("verdict_version").Min(0).Default(0),
		field.Int("passed_count").
			Default(0),
		field.Int("total_count").
			Default(0),
		field.Int("time_ms").
			Optional().
			Nillable(),
		field.Int("memory_kb").
			Optional().
			Nillable(),
		field.Text("error_message").
			Optional().
			Nillable(),
		field.UUID("contest_id", uuid.UUID{}).Optional().Nillable().Immutable(),
	}
}

// Indexes of the Submission.
func (Submission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "problem_id", "created_at", "id"),
		index.Fields("user_id", "problem_id", "contest_id", "created_at", "id"),
		index.Fields("id", "user_id", "problem_id").Unique(),
		index.Fields("user_id", "idempotency_key").Unique(),
		index.Fields("user_id"),
		index.Fields("problem_id"),
		index.Fields("contest_id"),
		index.Fields("status", "created_at", "id"),
		index.Fields("user_id", "created_at", "id"),
		index.Fields("problem_id", "created_at", "id"),
		index.Fields("latest_generation_phase", "latest_generation_queued_at", "id"),
		index.Fields("active_judgement_id"),
		index.Fields("contest_id", "contest_submission_sequence").Unique(),
		index.Fields("contest_id", "user_id", "problem_id", "contest_submission_sequence"),
	}
}

// Edges of the Submission.
func (Submission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("problem", Problem.Type).Ref("submissions").Field("problem_id").Unique().Immutable().Required(),
		edge.From("user", User.Type).Ref("submissions").Field("user_id").Unique().Immutable().Required(),
		edge.From("contest", Contest.Type).Ref("submissions").Field("contest_id").Unique().Immutable(),
		edge.To("contest_scoring_attempt", ContestScoringAttempt.Type).Unique(),
		edge.To("last_contest_problem_results", ContestProblemResult.Type),
	}
}

func (Submission) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"submission_source_code_bytes": fmt.Sprintf(
				"octet_length(source_code) BETWEEN 1 AND %d",
				judging.MaximumSourceCodeBytes,
			),
			"submission_status_closed": "status IN ('pending', 'judging', 'accepted', 'wrong_answer', 'time_limit_exceeded', 'memory_limit_exceeded', 'process_limit_exceeded', 'output_limit_exceeded', 'runtime_error', 'compile_error', 'internal_error')",
			"submission_verdict_complete": `(
				(status IN ('pending', 'judging') AND judged_at IS NULL AND verdict_version = 0) OR
				(status NOT IN ('pending', 'judging') AND judged_at IS NOT NULL AND verdict_version > 0)
			)`,
			"submission_nonzero_durable_identity": `id <> ` + problemNilUUIDSQL + `
				AND user_id <> ` + problemNilUUIDSQL + `
				AND problem_id <> ` + problemNilUUIDSQL + `
				AND (active_judgement_id IS NULL OR active_judgement_id <> ` + problemNilUUIDSQL + `)`,
			"submission_source_and_request_hashes": `octet_length(source_sha256) = 32
				AND source_sha256 <> ` + problemZeroDigestSQL + `
				AND octet_length(request_hash) = 32
				AND request_hash <> ` + problemZeroDigestSQL,
			"submission_runtime_key_bound": fmt.Sprintf(
				"octet_length(runtime_key) BETWEEN 1 AND %d AND runtime_key ~ '^[a-z0-9][a-z0-9._-]*$'",
				maximumSubmissionRuntimeKeyBytes,
			),
			"submission_idempotency_key_bound": fmt.Sprintf(
				"octet_length(idempotency_key) BETWEEN 1 AND %d AND idempotency_key ~ '^[ -~]+$'",
				maximumSubmissionIdempotencyKeyBytes,
			),
			"submission_origin_cid_bound": fmt.Sprintf(
				"octet_length(origin_cid) BETWEEN 1 AND %d",
				maximumSubmissionCorrelationIDBytes,
			),
			"submission_generation_and_version_bounds": "latest_generation >= 1 AND version >= 1",
			"submission_latest_generation_state": `(
				latest_generation_phase IN ('queued', 'claimed', 'compiling', 'running', 'finalizing')
				AND latest_generation_outcome IS NULL
				AND latest_generation_completed_at IS NULL
			) OR (
				latest_generation_phase = 'terminal'
				AND latest_generation_outcome IN ('accepted', 'wrong_answer', 'compile_error', 'runtime_error', 'time_limit_exceeded', 'memory_limit_exceeded', 'process_limit_exceeded', 'output_limit_exceeded', 'system_error')
				AND latest_generation_completed_at IS NOT NULL
			) OR (
				latest_generation_phase = 'cancelled'
				AND latest_generation_outcome = 'cancelled'
				AND latest_generation_completed_at IS NOT NULL
			)`,
			"submission_active_judgement_complete": `(
				active_judgement_id IS NULL
				AND active_judgement_generation IS NULL
				AND active_verdict IS NULL
			) OR (
				active_judgement_id IS NOT NULL
				AND active_judgement_generation BETWEEN 1 AND latest_generation
				AND active_verdict IN ('accepted', 'wrong_answer', 'compile_error', 'runtime_error', 'time_limit_exceeded', 'memory_limit_exceeded', 'process_limit_exceeded', 'output_limit_exceeded')
			)`,
			"submission_contest_snapshot_complete": `(
				contest_id IS NULL AND contest_problem_id IS NULL
				AND contest_scoring_policy_version IS NULL
				AND contest_elapsed_ms IS NULL
				AND contest_submission_sequence IS NULL
			) OR (
				contest_id IS NOT NULL AND contest_problem_id IS NOT NULL
				AND contest_scoring_policy_version >= 1
				AND contest_elapsed_ms >= 0
				AND contest_submission_sequence >= 1
			)`,
			"submission_generation_timestamps_ordered": `latest_generation_queued_at >= submitted_at
				AND (latest_generation_started_at IS NULL OR latest_generation_started_at >= latest_generation_queued_at)
				AND (latest_generation_completed_at IS NULL OR latest_generation_completed_at >= latest_generation_queued_at)`,
		}),
	}
}
