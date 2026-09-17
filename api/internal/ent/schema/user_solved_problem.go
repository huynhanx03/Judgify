package schema

import (
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	e "github.com/huynhanx03/go-common/pkg/database/ent"
)

// UserSolvedProblem tracks which user solved which problem (denormalized for fast lookup).
type UserSolvedProblem struct {
	ent.Schema
}

// Fields of the UserSolvedProblem.
// Mixin of the UserSolvedProblem.
func (UserSolvedProblem) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
	}
}

func (UserSolvedProblem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("problem_id", uuid.UUID{}),
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
