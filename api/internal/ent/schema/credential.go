package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Credential holds the schema definition for the Credential entity.
type Credential struct {
	ent.Schema
}

// Mixin of the Credential.
func (Credential) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Credential.
func (Credential) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.String("type").
			NotEmpty(),
		field.JSON("credential_data", map[string]interface{}{}),
	}
}

// Edges of the Credential.
func (Credential) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("credentials").
			Field("user_id").
			Unique().
			Required(),
	}
}
