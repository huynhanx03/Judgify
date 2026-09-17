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

// ContestRatingResult is one immutable participant result within a run.
type ContestRatingResult struct{ ent.Schema }

func (ContestRatingResult) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (ContestRatingResult) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("rating_run", ContestRatingRun.Type).
			Ref("results").
			Field("rating_run_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("user", User.Type).
			Ref("contest_rating_results").
			Field("user_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContestRatingResult) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("rating_run_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.Int("prior_rating").NonNegative().Immutable(),
		field.Int("prior_rated_contest_count").NonNegative().Immutable(),
		field.Int("rank_position").Positive().Immutable(),
		field.Float("actual_rank").Positive().Immutable(),
		field.Float("expected_rank").Positive().Immutable(),
		field.Int("delta").Immutable(),
		field.Int("new_rating").NonNegative().Immutable(),
	}
}

func (ContestRatingResult) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("rating_run_id", "user_id").Unique(),
		index.Fields("user_id", "created_at", "id"),
	}
}

func (ContestRatingResult) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"contest_rating_result_nonzero_ids": "rating_run_id <> '00000000-0000-0000-0000-000000000000'::uuid AND user_id <> '00000000-0000-0000-0000-000000000000'::uuid",
			"contest_rating_result_math":        "new_rating = GREATEST(0, prior_rating + delta)",
		}),
	}
}
