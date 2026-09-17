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
)

// UserProblemSolveTransition is one immutable contiguous projection change.
// Its to_version is also the outbox aggregate version.
type UserProblemSolveTransition struct{ ent.Schema }

func (UserProblemSolveTransition) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (UserProblemSolveTransition) Fields() []ent.Field {
	return append(submissionImmutableRecordFields(),
		field.UUID("event_id", uuid.UUID{}).Immutable(),
		field.UUID("solve_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.Int64("from_version").Min(0).Immutable(),
		field.Int64("to_version").Min(1).Immutable(),
		field.Enum("from_status").Values("solved", "unsolved").Immutable(),
		field.Enum("to_status").Values("solved", "unsolved").Immutable(),
		field.UUID("from_submission_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("to_submission_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("from_judgement_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("to_judgement_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int64("from_generation").Min(1).Optional().Nillable().Immutable(),
		field.Int64("to_generation").Min(1).Optional().Nillable().Immutable(),
		field.UUID("from_problem_revision_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("to_problem_revision_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.JSON("from_reward_facts", json.RawMessage{}).
			SchemaType(submissionJSONSchemaType()).Optional().Immutable(),
		field.JSON("to_reward_facts", json.RawMessage{}).
			SchemaType(submissionJSONSchemaType()).Optional().Immutable(),
		field.Bytes("from_reward_facts_checksum").Optional().Nillable().Immutable(),
		field.Bytes("to_reward_facts_checksum").Optional().Nillable().Immutable(),
		field.UUID("from_reward_grant_source_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("to_reward_grant_source_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Enum("from_reward_provenance").Values(rewardProvenances...).Optional().Nillable().Immutable(),
		field.Enum("to_reward_provenance").Values(rewardProvenances...).Optional().Nillable().Immutable(),
		field.Time("from_solved_at").Optional().Nillable().Immutable(),
		field.Time("to_solved_at").Optional().Nillable().Immutable(),
		field.Enum("provenance").Values("live").Immutable(),
		field.Time("occurred_at").Immutable(),
		field.String("correlation_id").NotEmpty().MaxLen(maximumSubmissionCorrelationIDBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
	)
}

func (UserProblemSolveTransition) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("event_id").Unique(),
		index.Fields("solve_id", "to_version").Unique(),
		index.Fields(
			"id", "event_id", "solve_id", "user_id", "problem_id", "to_version",
		).Unique(),
		index.Fields("id", "solve_id", "to_version").Unique(),
		index.Fields("id", "solve_id").Unique(),
		index.Fields("solve_id", "occurred_at", "id"),
		index.Fields("user_id", "problem_id", "to_version"),
		index.Fields("correlation_id", "occurred_at", "id"),
	}
}

func (UserProblemSolveTransition) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"user_problem_solve_transition_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
					AND event_id <> ` + problemNilUUIDSQL + `
					AND solve_id <> ` + problemNilUUIDSQL + `
				AND user_id <> ` + problemNilUUIDSQL + `
				AND problem_id <> ` + problemNilUUIDSQL,
			"user_problem_solve_transition_contiguous": "to_version = from_version + 1",
			"user_problem_solve_transition_changes_projection": `NOT (
				from_status = 'unsolved' AND to_status = 'unsolved'
			)`,
			"user_problem_solve_transition_from_side": solveTransitionSideCheck("from"),
			"user_problem_solve_transition_to_side":   solveTransitionSideCheck("to"),
			"user_problem_solve_transition_reward_bounds": fmt.Sprintf(
				`(from_reward_facts IS NULL OR (
					jsonb_typeof(from_reward_facts) = 'object'
					AND octet_length(from_reward_facts::text) <= %d
				)) AND (to_reward_facts IS NULL OR (
					jsonb_typeof(to_reward_facts) = 'object'
					AND octet_length(to_reward_facts::text) <= %d
				))`,
				maximumJudgeSpecJSONBytes,
				maximumJudgeSpecJSONBytes,
			),
			"user_problem_solve_transition_reward_checksums": `(
				from_reward_facts_checksum IS NULL OR (
					octet_length(from_reward_facts_checksum) = 32
					AND from_reward_facts_checksum <> ` + problemZeroDigestSQL + `
				)
			) AND (
				to_reward_facts_checksum IS NULL OR (
					octet_length(to_reward_facts_checksum) = 32
					AND to_reward_facts_checksum <> ` + problemZeroDigestSQL + `
				)
			)`,
			"user_problem_solve_transition_reward_grant": `(
				from_status = 'unsolved' AND to_status = 'solved'
				AND to_reward_grant_source_id = id
			) OR (
				from_status = 'solved' AND to_status = 'solved'
				AND to_reward_grant_source_id = from_reward_grant_source_id
				AND to_reward_provenance = from_reward_provenance
			) OR (
				from_status = 'solved' AND to_status = 'unsolved'
			)`,
			"user_problem_solve_transition_origin": `from_version <> 0 OR (
				from_status = 'unsolved'
				AND from_submission_id IS NULL
				AND from_judgement_id IS NULL
			)`,
		}),
	}
}

func solveTransitionSideCheck(side string) string {
	return fmt.Sprintf(`(
		%s_status = 'unsolved'
		AND %s_submission_id IS NULL
		AND %s_judgement_id IS NULL
		AND %s_generation IS NULL
		AND %s_problem_revision_id IS NULL
		AND %s_reward_facts IS NULL
		AND %s_reward_facts_checksum IS NULL
		AND %s_reward_grant_source_id IS NULL
		AND %s_reward_provenance IS NULL
		AND %s_solved_at IS NULL
	) OR (
		%s_status = 'solved'
		AND %s_submission_id IS NOT NULL
		AND %s_judgement_id IS NOT NULL
		AND %s_generation >= 1
		AND %s_problem_revision_id IS NOT NULL
		AND %s_reward_facts IS NOT NULL
		AND %s_reward_facts_checksum IS NOT NULL
		AND %s_reward_grant_source_id IS NOT NULL
		AND %s_reward_provenance IS NOT NULL
		AND %s_solved_at IS NOT NULL
	)`,
		side, side, side, side, side, side, side, side, side, side,
		side, side, side, side, side, side, side, side, side, side,
	)
}
