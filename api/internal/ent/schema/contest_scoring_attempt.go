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

// ContestScoringAttempt is the canonical ordered scoring input for one
// official submission. Rejudge replaces only its active judgement fields;
// intake-owned identity, order, and elapsed time remain immutable.
type ContestScoringAttempt struct{ ent.Schema }

func (ContestScoringAttempt) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (ContestScoringAttempt) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).
			Ref("scoring_attempts").
			Field("contest_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("user", User.Type).
			Ref("contest_scoring_attempts").
			Field("user_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("problem", Problem.Type).
			Ref("contest_scoring_attempts").
			Field("problem_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("contest_problem", ContestProblem.Type).
			Ref("scoring_attempts").
			Field("contest_problem_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("submission", Submission.Type).
			Ref("contest_scoring_attempt").
			Field("submission_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContestScoringAttempt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.UUID("contest_problem_id", uuid.UUID{}).Immutable(),
		field.UUID("submission_id", uuid.UUID{}).Immutable(),
		field.Time("submitted_at").Immutable(),
		field.Int64("submission_elapsed_ms").Min(0).Immutable(),
		field.Int64("contest_submission_sequence").Min(1).Immutable(),
		field.Int64("active_generation").Min(1),
		field.Enum("terminal_verdict").
			Values(
				"accepted",
				"wrong_answer",
				"compile_error",
				"runtime_error",
				"time_limit_exceeded",
				"memory_limit_exceeded",
				"process_limit_exceeded",
				"output_limit_exceeded",
			),
		field.Int64("judgement_version").Min(1),
		field.UUID("source_event_id", uuid.UUID{}),
		field.Int64("version").Min(1).Default(1),
	}
}

func (ContestScoringAttempt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("submission_id").Unique(),
		index.Fields("contest_id", "contest_submission_sequence").Unique(),
		index.Fields("contest_id", "user_id", "contest_problem_id", "contest_submission_sequence"),
		index.Fields("contest_id", "user_id", "problem_id", "contest_submission_sequence"),
		index.Fields("source_event_id").Unique(),
	}
}

func (ContestScoringAttempt) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"contest_scoring_attempt_nonzero_ids": "contest_id <> '00000000-0000-0000-0000-000000000000'::uuid AND user_id <> '00000000-0000-0000-0000-000000000000'::uuid AND problem_id <> '00000000-0000-0000-0000-000000000000'::uuid AND contest_problem_id <> '00000000-0000-0000-0000-000000000000'::uuid AND submission_id <> '00000000-0000-0000-0000-000000000000'::uuid AND source_event_id <> '00000000-0000-0000-0000-000000000000'::uuid",
		}),
	}
}
