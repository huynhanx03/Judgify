package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

const defaultAuthorizationStateID = "default"

// AuthorizationState is the single-tenant policy revision fence. Runtime
// readiness and every authorization read compare their applied snapshot to
// this committed desired revision.
type AuthorizationState struct{ ent.Schema }

func (AuthorizationState) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.TimeMixin{}}
}

func (AuthorizationState) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").Default(defaultAuthorizationStateID).Immutable().MaxLen(32),
		field.Int64("desired_revision").Min(0).Default(0),
		field.UUID("updated_by", uuid.UUID{}).Optional().Nillable(),
	}
}

func (AuthorizationState) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("updater", User.Type).
			Ref("authorization_updates").
			Field("updated_by").
			Unique(),
	}
}

func (AuthorizationState) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Check("id = 'default'"),
	}
}
