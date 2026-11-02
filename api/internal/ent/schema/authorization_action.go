package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// AuthorizationAction is a code-owned action scoped to one resource.
type AuthorizationAction struct{ ent.Schema }

func (AuthorizationAction) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (AuthorizationAction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("resource_id", uuid.UUID{}).Immutable(),
		field.String("key").NotEmpty().MaxLen(64).Immutable(),
		field.String("label").NotEmpty().MaxLen(128),
		field.String("description").MaxLen(1024).Optional(),
		field.Int("display_order").Min(0).Default(0),
		field.Int64("catalog_revision").Min(1).Default(1),
	}
}

func (AuthorizationAction) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("resource", AuthorizationResource.Type).
			Ref("actions").
			Field("resource_id").
			Unique().
			Immutable().
			Required(),
	}
}

func (AuthorizationAction) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("resource_id", "key").Unique(),
		index.Fields("resource_id", "display_order", "key"),
	}
}
