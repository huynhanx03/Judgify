package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
)

// AttributeDefinition holds the schema definition for the AttributeDefinition entity.
type AttributeDefinition struct {
	ent.Schema
}

// Mixin of the AttributeDefinition.
func (AttributeDefinition) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		e.SoftDeleteMixin{},
	}
}

// Fields of the AttributeDefinition.
func (AttributeDefinition) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			Unique().
			NotEmpty(),
		field.String("data_type").
			Default("string"),
		field.String("description").
			Optional(),
	}
}

// Edges of the AttributeDefinition.
func (AttributeDefinition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("values", UserAttributeValue.Type),
	}
}
