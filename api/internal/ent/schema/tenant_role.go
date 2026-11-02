package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// TenantRole stores tenant-local role definitions; Casbin grouping rules assign users to roles.
type TenantRole struct{ ent.Schema }

func (TenantRole) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (TenantRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("tenant_id", uuid.UUID{}),
		field.String("key").NotEmpty().MaxLen(64),
		field.String("name").NotEmpty().MaxLen(120),
		field.String("description").Optional().Nillable(),
		field.Bool("is_system").Default(false),
		field.Enum("status").Values("active", "archived").Default("active"),
	}
}

func (TenantRole) Edges() []ent.Edge {
	return []ent.Edge{edge.From("tenant", Tenant.Type).Ref("roles").Field("tenant_id").Unique().Required()}
}

func (TenantRole) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "key").Unique(), index.Fields("tenant_id", "status")}
}
