package schema

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

var rewardProvenances = []string{"live"}

// UserProblemSolve is the canonical current solved-state projection for one
// immutable user/problem ownership pair.
type UserProblemSolve struct{ ent.Schema }

func (UserProblemSolve) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (UserProblemSolve) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.Enum("status").Values("solved", "unsolved").Default("unsolved"),
		field.UUID("active_submission_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("active_judgement_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("active_generation").Min(1).Optional().Nillable(),
		field.UUID("active_problem_revision_id", uuid.UUID{}).Optional().Nillable(),
		field.JSON("reward_facts", json.RawMessage{}).
			SchemaType(submissionJSONSchemaType()).Optional(),
		field.Bytes("reward_facts_checksum").Optional().Nillable(),
		field.UUID("active_reward_transition_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("reward_provenance").Values(rewardProvenances...).Optional().Nillable(),
		field.Time("solved_at").Optional().Nillable(),
		field.Int64("version").Min(0).Default(0),
	}
}

func (UserProblemSolve) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "problem_id").Unique(),
		index.Fields("id", "user_id", "problem_id").Unique(),
		index.Fields("status", "problem_id", "solved_at", "id"),
		index.Fields("active_submission_id", "active_judgement_id"),
		index.Fields("active_reward_transition_id").Unique(),
	}
}

func (UserProblemSolve) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"user_problem_solve_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
				AND user_id <> ` + problemNilUUIDSQL + `
				AND problem_id <> ` + problemNilUUIDSQL,
			"user_problem_solve_version_bound": "version >= 0 AND (status = 'unsolved' OR version >= 1)",
			"user_problem_solve_pointer_invariant": `(
				status = 'unsolved'
				AND active_submission_id IS NULL
				AND active_judgement_id IS NULL
				AND active_generation IS NULL
				AND active_problem_revision_id IS NULL
				AND reward_facts IS NULL
				AND reward_facts_checksum IS NULL
				AND active_reward_transition_id IS NULL
				AND reward_provenance IS NULL
				AND solved_at IS NULL
			) OR (
				status = 'solved'
				AND active_submission_id IS NOT NULL
				AND active_judgement_id IS NOT NULL
				AND active_generation >= 1
				AND active_problem_revision_id IS NOT NULL
				AND reward_facts IS NOT NULL
				AND reward_facts_checksum IS NOT NULL
				AND active_reward_transition_id IS NOT NULL
				AND reward_provenance IS NOT NULL
				AND solved_at IS NOT NULL
			)`,
			"user_problem_solve_reward_facts_bound": fmt.Sprintf(
				`reward_facts IS NULL OR (
					jsonb_typeof(reward_facts) = 'object'
					AND octet_length(reward_facts::text) <= %d
				)`,
				maximumJudgeSpecJSONBytes,
			),
			"user_problem_solve_reward_checksum": `reward_facts_checksum IS NULL OR (
				octet_length(reward_facts_checksum) = 32
				AND reward_facts_checksum <> ` + problemZeroDigestSQL + `
			)`,
		}),
	}
}
