package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// TestCase holds the schema definition for the TestCase entity.
type TestCase struct {
	ent.Schema
}

// Mixin of the TestCase.
func (TestCase) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the TestCase.
func (TestCase) Fields() []ent.Field {
	return []ent.Field{
		field.Int("problem_id"),
		field.Text("input").
			NotEmpty(),
		field.Text("expected_output").
			NotEmpty(),
		field.Bool("is_sample").
			Default(false),
		field.Int("order_index"),
	}
}

// Edges of the TestCase.
func (TestCase) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("problem", Problem.Type).Ref("test_cases").Field("problem_id").Unique().Required(),
	}
}
