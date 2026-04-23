package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// MaterialCategory holds the schema definition for the MaterialCategory entity.
type MaterialCategory struct {
	ent.Schema
}

// Mixin of the MaterialCategory.
func (MaterialCategory) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the MaterialCategory.
func (MaterialCategory) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Unique().
			NotEmpty().
			MaxLen(100),
		field.String("description").
			Optional().
			MaxLen(500),
	}
}

// Edges of the MaterialCategory.
func (MaterialCategory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("materials", Material.Type),
	}
}
