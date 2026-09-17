package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	e "github.com/huynhanx03/go-common/pkg/database/ent"
)

// RatingHistory holds the schema definition for the RatingHistory entity.
type RatingHistory struct {
	ent.Schema
}

// Mixin of the RatingHistory.
func (RatingHistory) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
	}
}

func (RatingHistory) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("contest_id", uuid.UUID{}),
		field.UUID("rating_run_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("rating_sequence").Min(1).Optional().Nillable(),
		field.Int64("run_version").Min(1).Optional().Nillable(),
		field.Int("old_rating").Default(1200),
		field.Int("new_rating").Default(1200),
		field.Int("rank_position").Default(0),
		field.Int("prior_rated_contest_count").NonNegative().Default(0),
		field.Float("actual_rank").Positive().Optional().Nillable(),
		field.Float("expected_rank").Positive().Optional().Nillable(),
		field.Int("delta").Default(0),
		field.String("algorithm_version").MaxLen(64).Optional().Nillable(),
	}
}

func (RatingHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("contest_id"),
		index.Fields("user_id", "contest_id"),
		index.Fields("rating_run_id", "user_id").Unique(),
		index.Fields("rating_sequence", "user_id"),
	}
}

func (RatingHistory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("rating_histories").Field("user_id").Unique().Required(),
		edge.From("contest", Contest.Type).Ref("rating_histories").Field("contest_id").Unique().Required(),
		edge.From("rating_run", ContestRatingRun.Type).
			Ref("compatibility_histories").
			Field("rating_run_id").
			Unique().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}
