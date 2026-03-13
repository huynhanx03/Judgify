package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
)

// UserAttributeValue holds the schema definition for the UserAttributeValue entity.
type UserAttributeValue struct {
	ent.Schema
}

// Mixin of the UserAttributeValue.
func (UserAttributeValue) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		e.SoftDeleteMixin{},
	}
}

// Fields of the UserAttributeValue.
func (UserAttributeValue) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Int("attribute_id"),
		field.Text("value").
			NotEmpty(),
	}
}

// Edges of the UserAttributeValue.
func (UserAttributeValue) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("attributes").
			Field("user_id").
			Unique().
			Required(),
		edge.From("definition", AttributeDefinition.Type).
			Ref("values").
			Field("attribute_id").
			Unique().
			Required(),
	}
}

// Indexes of the UserAttributeValue.
func (UserAttributeValue) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "attribute_id").
			Unique(),
	}
}
