package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// UserTrait holds one root-bone or talent assignment selected from a durable
// onboarding offer or changed through an audited administration command.
type UserTrait struct {
	ent.Schema
}

// Mixin of the UserTrait.
func (UserTrait) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the UserTrait.
func (UserTrait) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("trait_id", uuid.UUID{}),
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
