package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ContestStanding holds the schema definition for the ContestStanding entity.
type ContestStanding struct {
	ent.Schema
}

func (ContestStanding) Fields() []ent.Field {
	return []ent.Field{
		field.Int("contest_id"),
		field.Int("user_id"),
		field.Int("solved_count").Default(0),
		field.Int("penalty").Default(0).Comment("Total penalty in seconds"),
		field.JSON("problem_results", map[string]any{}).Optional().Comment("Per-problem results"),
	}
}

func (ContestStanding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "user_id").Unique(),
	}
}

func (ContestStanding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).Ref("standings").Field("contest_id").Unique().Required(),
		edge.From("user", User.Type).Ref("contest_standings").Field("user_id").Unique().Required(),
	}
}
