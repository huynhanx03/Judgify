package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Mixin of the User.
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("username").
			Unique().
			NotEmpty(),
		field.Int("role_id"),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("role", Role.Type).Ref("users").Field("role_id").Unique().Required(),
		edge.To("credentials", Credential.Type),
		edge.To("attributes", UserAttributeValue.Type),
		edge.To("federated_identities", FederatedIdentity.Type),
		edge.To("problems", Problem.Type),
		edge.To("user_traits", UserTrait.Type),
		edge.To("user_element_exps", UserElementExp.Type),
		edge.To("user_stats", UserStats.Type),
	}
}
