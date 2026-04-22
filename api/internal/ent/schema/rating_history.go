package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RatingHistory holds the schema definition for the RatingHistory entity.
type RatingHistory struct {
	ent.Schema
}

func (RatingHistory) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Int("contest_id"),
		field.Int("old_rating").Default(0),
		field.Int("new_rating").Default(0),
		field.Int("rank_position").Default(0),
	}
}

func (RatingHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("contest_id"),
		index.Fields("user_id", "contest_id").Unique(),
	}
}

func (RatingHistory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("rating_histories").Field("user_id").Unique().Required(),
		edge.From("contest", Contest.Type).Ref("rating_histories").Field("contest_id").Unique().Required(),
	}
}
