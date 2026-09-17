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

// Tag holds the schema definition for the Tag entity.
type Tag struct {
	ent.Schema
}

// Mixin of the Tag.
func (Tag) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Tag.
func (Tag) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Unique().
			NotEmpty().
			MaxLen(50),
		field.Int64("version").
			Min(1).
			Default(1),
	}
}

// Edges of the Tag.
func (Tag) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("revisions", TagRevision.Type),
		edge.From("problems", Problem.Type).Ref("tags"),
		edge.To("elements", Element.Type),
		edge.To("user_tag_stats", UserTagStats.Type),
		edge.To("material_revision_tags", MaterialRevisionTag.Type),
		edge.From("materials", Material.Type).Ref("tags"),
	}
}

func (Tag) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"tag_positive_version": "version >= 1",
		}),
	}
}
