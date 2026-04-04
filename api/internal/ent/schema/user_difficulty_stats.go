package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserDifficultyStats tracks how many problems a user solved per difficulty.
type UserDifficultyStats struct {
	ent.Schema
}

func (UserDifficultyStats) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Int("difficulty_id"),
		field.Int("solved_count").Default(0).NonNegative(),
	}
}

func (UserDifficultyStats) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("difficulty_stats").Field("user_id").Unique().Required(),
		edge.From("difficulty", Difficulty.Type).Ref("user_difficulty_stats").Field("difficulty_id").Unique().Required(),
	}
}

func (UserDifficultyStats) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "difficulty_id").Unique(),
		index.Fields("user_id"),
	}
}
