package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserSolvedProblem tracks which user solved which problem (denormalized for fast lookup).
type UserSolvedProblem struct {
	ent.Schema
}

// Fields of the UserSolvedProblem.
func (UserSolvedProblem) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Int("problem_id"),
		field.Time("solved_at").
			Default(time.Now),
	}
}

// Edges of the UserSolvedProblem.
func (UserSolvedProblem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("solved_problems").Field("user_id").Unique().Required(),
		edge.From("problem", Problem.Type).Ref("solvers").Field("problem_id").Unique().Required(),
	}
}

// Indexes of the UserSolvedProblem.
func (UserSolvedProblem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "problem_id").Unique(),
		index.Fields("user_id"),
	}
}
