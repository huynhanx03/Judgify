package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Permission holds the schema definition for the Permission entity.
type Permission struct {
	ent.Schema
}

// Mixin of the Permission.
func (Permission) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Permission.
func (Permission) Fields() []ent.Field {
	return []ent.Field{
		field.Int("role_id"),
		field.Int("resource_id"),
		field.String("description").
			Optional().
			Nillable(),
		field.Int("scopes").
			Default(0).
			Comment("Bitmask of allowed operations"),
	}
}

// Edges of the Permission.
func (Permission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("role", Role.Type).
			Ref("permissions").
			Field("role_id").
			Unique().
			Required(),
		edge.From("resource", Resource.Type).
			Ref("permissions").
			Field("resource_id").
			Unique().
			Required(),
	}
}

// Indexes of the Permission.
func (Permission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role_id", "resource_id").
			Unique(),
	}
}
