package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// AuthorizationResource is the persisted projection of the code-owned
// capability catalog. Admins assign capabilities but cannot invent resources.
type AuthorizationResource struct{ ent.Schema }

func (AuthorizationResource) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (AuthorizationResource) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").NotEmpty().MaxLen(64).Unique().Immutable(),
		field.String("label").NotEmpty().MaxLen(128),
		field.String("description").MaxLen(1024).Optional(),
		field.Int("display_order").Min(0).Default(0),
		field.Int64("catalog_revision").Min(1).Default(1),
	}
}

func (AuthorizationResource) Edges() []ent.Edge {
	return []ent.Edge{edge.To("actions", AuthorizationAction.Type)}
}
