package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Resource holds the schema definition for the Resource entity.
type Resource struct {
	ent.Schema
}

// Mixin of the Resource.
func (Resource) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Resource.
func (Resource) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			Unique().
			NotEmpty(),
		field.String("description").
			Optional().
			Nillable(),
	}
}

// Edges of the Resource.
func (Resource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("permissions", Permission.Type),
	}
}
