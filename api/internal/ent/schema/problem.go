package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Problem holds the schema definition for the Problem entity.
type Problem struct {
	ent.Schema
}

// Mixin of the Problem.
func (Problem) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Problem.
func (Problem) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").
			NotEmpty().
			MaxLen(300),
		field.Text("description").
			NotEmpty(),
		field.Int("difficulty_id"),
		field.Int("time_limit_ms").
			Default(1000),
		field.Int("memory_limit_kb").
			Default(262144),
		field.Int("author_id"),
		field.Bool("is_published").
			Default(false),
		field.Int("submission_count").
			Default(0).
			Comment("Denormalized total submission count"),
		field.Int("accepted_count").
			Default(0).
			Comment("Denormalized accepted submission count"),
	}
}

// Edges of the Problem.
func (Problem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("author", User.Type).Ref("problems").Field("author_id").Unique().Required(),
		edge.From("difficulty", Difficulty.Type).Ref("problems").Field("difficulty_id").Unique().Required(),
		edge.To("test_cases", TestCase.Type),
		edge.To("submissions", Submission.Type),
		edge.To("tags", Tag.Type),
		edge.To("solvers", UserSolvedProblem.Type),
	}
}
