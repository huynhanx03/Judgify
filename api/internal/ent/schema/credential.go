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

// Credential holds the schema definition for the Credential entity.
type Credential struct {
	ent.Schema
}

// Mixin of the Credential.
func (Credential) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Credential.
func (Credential) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}),
		field.String("type").
			NotEmpty().
			MaxLen(32),
		field.JSON("credential_data", map[string]interface{}{}),
		field.Int64("version").
			Min(1).
			Default(1),
		field.Bool("active").
			Default(true),
		field.Time("last_used_at").
			Optional().
			Nillable(),
	}
}

// Indexes prevent parallel usable credentials of the same kind from being
// created for one account.
func (Credential) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "type").Unique(),
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
