package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Role holds the schema definition for the Role entity.
type Role struct {
	ent.Schema
}

// Mixin of the Role.
func (Role) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Role.
func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Unique().
			NotEmpty(),
		field.Int("level").
			Default(0).
			Comment("Role hierarchy level, lower = more privileged"),
		field.Int("parent_id").
			Default(-1).
			Comment("Parent role ID for hierarchy (root = -1)"),
		field.Int("lft").
			Default(0).
			Comment("Nested Set left value for hierarchy"),
		field.Int("rgt").
			Default(0).
			Comment("Nested Set right value for hierarchy"),
	}
}

// Edges of the Role.
func (Role) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("users", User.Type),
		edge.To("permissions", Permission.Type),
	}
}
