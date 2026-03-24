package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Submission holds the schema definition for the Submission entity.
type Submission struct {
	ent.Schema
}

// Mixin of the Submission.
func (Submission) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Submission.
func (Submission) Fields() []ent.Field {
	return []ent.Field{
		field.Int("problem_id"),
		field.Int("user_id"),
		field.Enum("language").
			Values("cpp", "python", "java", "go"),
		field.Text("source_code").
			NotEmpty(),
		field.Enum("status").
			Values(
				"pending",
				"judging",
				"accepted",
				"wrong_answer",
				"time_limit_exceeded",
				"memory_limit_exceeded",
				"runtime_error",
				"compile_error",
			).
			Default("pending"),
		field.Int("passed_count").
			Default(0),
		field.Int("total_count").
			Default(0),
		field.Int("time_ms").
			Optional().
			Nillable(),
		field.Int("memory_kb").
			Optional().
			Nillable(),
		field.Text("error_message").
			Optional().
			Nillable(),
	}
}

// Edges of the Submission.
func (Submission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("problem", Problem.Type).Ref("submissions").Field("problem_id").Unique().Required(),
		edge.From("user", User.Type).Ref("submissions").Field("user_id").Unique().Required(),
	}
}
