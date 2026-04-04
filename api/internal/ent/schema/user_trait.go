package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// UserTrait holds the schema definition for the UserTrait entity (gacha result).
type UserTrait struct {
	ent.Schema
}

// Mixin of the UserTrait.
func (UserTrait) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the UserTrait.
func (UserTrait) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Int("trait_id"),
	}
}

// Edges of the UserTrait.
func (UserTrait) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("user_traits").Field("user_id").Unique().Required(),
		edge.From("trait", Trait.Type).Ref("user_traits").Field("trait_id").Unique().Required(),
	}
}

// Indexes of the UserTrait.
func (UserTrait) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "trait_id").Unique(),
	}
}
