package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Material holds the schema definition for the Material entity.
type Material struct {
	ent.Schema
}

// Mixin of the Material.
func (Material) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Material.
func (Material) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").
			NotEmpty().
			MaxLen(255),
		field.String("description").
			Optional().
			MaxLen(1000),
		field.Text("content").
			Optional(),
		field.Int("difficulty_id"),
		field.Int("author_id"),
		field.Int("category_id"),
		field.Enum("status").
			Values("draft", "published").
			Default("draft"),
		field.Enum("visibility").
			Values("public", "group").
			Default("public"),
		field.Int("group_id").
			Optional().
			Nillable().
			Comment("Null = public, set = group-scoped (deferred until groups exist)"),
		field.Int("view_count").
			Default(0).
			NonNegative(),
		field.Int("estimated_read_time").
			Default(0).
			NonNegative().
			Comment("Minutes, auto-calculated from content length"),
	}
}

// Indexes of the Material.
func (Material) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("category_id"),
		index.Fields("difficulty_id"),
	}
}

// Edges of the Material.
func (Material) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("category", MaterialCategory.Type).Ref("materials").Field("category_id").Unique().Required(),
		edge.From("author", User.Type).Ref("materials").Field("author_id").Unique().Required(),
		edge.From("difficulty", Difficulty.Type).Ref("materials").Field("difficulty_id").Unique().Required(),
		edge.To("tags", Tag.Type),
	}
}
