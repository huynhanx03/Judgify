package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Difficulty holds the schema definition for the Difficulty entity.
type Difficulty struct {
	ent.Schema
}

// Mixin of the Difficulty.
func (Difficulty) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Difficulty.
func (Difficulty) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Unique().
			NotEmpty().
			MaxLen(50),
		field.Int("level").
			Unique().
			Positive(),
		field.Int64("exp_reward").
			Default(0).
			NonNegative(),
		field.String("description").
			Optional().
			MaxLen(255),
	}
}

// Edges of the Difficulty.
func (Difficulty) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("problems", Problem.Type),
	}
}
