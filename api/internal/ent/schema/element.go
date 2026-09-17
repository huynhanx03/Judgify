package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Element holds the schema definition for the Element entity.
type Element struct {
	ent.Schema
}

// Mixin of the Element.
func (Element) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
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
		field.Int64("version").
			Min(1).
			Default(1),
	}
}

// Edges of the Element.
func (Element) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("revisions", ElementRevision.Type),
		edge.To("tag_revision_elements", TagRevisionElement.Type),
		edge.From("tags", Tag.Type).Ref("elements"),
		edge.To("user_element_exps", UserElementExp.Type),
	}
}

func (Element) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"element_positive_version": "version >= 1",
		}),
	}
}
