package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// ContestProblemResult is the normalized official contribution of one
// contestant/problem. It is always rebuilt from ContestScoringAttempt order,
// never incremented from delivery timing.
type ContestProblemResult struct{ ent.Schema }

func (ContestProblemResult) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (ContestProblemResult) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).
			Ref("problem_results").
			Field("contest_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("user", User.Type).
			Ref("contest_problem_results").
			Field("user_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("problem", Problem.Type).
			Ref("contest_problem_results").
			Field("problem_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("contest_problem", ContestProblem.Type).
			Ref("scoring_results").
			Field("contest_problem_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("last_scoring_submission", Submission.Type).
			Ref("last_contest_problem_results").
			Field("last_scoring_submission_id").
			Unique().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContestProblemResult) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.UUID("contest_problem_id", uuid.UUID{}).Immutable(),
		field.Bool("accepted").Default(false),
		field.Int64("accepted_elapsed_ms").Min(0).Optional().Nillable(),
		field.Int("wrong_attempts_before_accept").NonNegative().Default(0),
		field.Int("best_score").NonNegative().Default(0),
		field.UUID("last_scoring_submission_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("last_accepted_sequence").Min(1).Optional().Nillable(),
		field.Int64("version").Min(1).Default(1),
	}
}

func (ContestProblemResult) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "user_id", "contest_problem_id").Unique(),
		index.Fields("contest_id", "user_id", "problem_id").Unique(),
		index.Fields("last_scoring_submission_id"),
	}
}

func (ContestProblemResult) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"contest_problem_result_nonzero_ids": "contest_id <> '00000000-0000-0000-0000-000000000000'::uuid AND user_id <> '00000000-0000-0000-0000-000000000000'::uuid AND problem_id <> '00000000-0000-0000-0000-000000000000'::uuid AND contest_problem_id <> '00000000-0000-0000-0000-000000000000'::uuid",
			"contest_problem_result_acceptance":  "(accepted AND accepted_elapsed_ms IS NOT NULL AND last_scoring_submission_id IS NOT NULL AND last_accepted_sequence IS NOT NULL) OR (NOT accepted AND accepted_elapsed_ms IS NULL AND last_accepted_sequence IS NULL)",
		}),
	}
}
