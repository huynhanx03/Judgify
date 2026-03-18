package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Element holds the schema definition for the Element entity.
type Element struct {
	ent.Schema
}

// Mixin of the Element.
func (Element) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Element.
func (Element) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Unique().
			NotEmpty().
			MaxLen(50),
		field.String("code").
			Unique().
			NotEmpty().
			MaxLen(20).
			Comment("Unique code: fire, water, wood, metal, earth"),
		field.String("description").
			Optional().
			MaxLen(255),
		field.String("color").
			Optional().
			MaxLen(20).
			Comment("Hex color for UI display"),
		field.String("icon").
			Optional().
			MaxLen(100).
			Comment("Icon path or class name"),
		field.Int("order").
			Default(0).
			Comment("Display order"),
	}
}

// Edges of the Element.
func (Element) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user_element_exps", UserElementExp.Type),
	}
}
