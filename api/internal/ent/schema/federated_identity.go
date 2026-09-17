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

// FederatedIdentity holds the schema definition for the FederatedIdentity entity.
type FederatedIdentity struct {
	ent.Schema
}

// Mixin of the FederatedIdentity.
func (FederatedIdentity) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the FederatedIdentity.
func (FederatedIdentity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}),
		field.String("provider").
			NotEmpty().
			MaxLen(64).
			Comment("Stable provider key from the configured OAuth registry"),
		field.String("provider_subject").
			NotEmpty().
			MaxLen(256).
			Sensitive().
			Comment("Immutable subject asserted by the provider"),
		field.String("display_name").
			MaxLen(256).
			Optional().
			Nillable().
			Sensitive(),
		field.String("avatar_url").
			MaxLen(2048).
			Optional().
			Nillable().
			Sensitive(),
		field.Time("last_authenticated_at").
			Optional().
			Nillable(),
	}
}

// Edges of the FederatedIdentity.
func (FederatedIdentity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("federated_identities").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes of the FederatedIdentity.
func (FederatedIdentity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "provider").
			Unique(),
		index.Fields("provider", "provider_subject").
			Unique(),
	}
}
