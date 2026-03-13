package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
)

// FederatedIdentity holds the schema definition for the FederatedIdentity entity.
type FederatedIdentity struct {
	ent.Schema
}

// Mixin of the FederatedIdentity.
func (FederatedIdentity) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		e.SoftDeleteMixin{},
	}
}

// Fields of the FederatedIdentity.
func (FederatedIdentity) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.String("provider").
			NotEmpty().
			Comment("google, facebook, github, etc."),
		field.String("external_id").
			NotEmpty().
			Comment("User ID from external provider"),
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
		index.Fields("provider", "external_id").
			Unique(),
	}
}
