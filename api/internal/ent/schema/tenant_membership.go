package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// TenantMembership records participation only. Casbin assigns one or more roles.
type TenantMembership struct{ ent.Schema }

func (TenantMembership) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (TenantMembership) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("tenant_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Enum("status").Values("active", "suspended", "left").Default("active"),
		field.Time("joined_at"),
		field.Time("left_at").Optional().Nillable(),
	}
}

func (TenantMembership) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Tenant.Type).Ref("memberships").Field("tenant_id").Unique().Required(),
		edge.From("user", User.Type).Ref("tenant_memberships").Field("user_id").Unique().Required(),
	}
}

func (TenantMembership) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "user_id").Unique(),
		index.Fields("user_id", "status"),
		index.Fields("tenant_id", "status"),
	}
}

func (TenantMembership) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"tenant_membership_status_closed": "status IN ('active', 'suspended', 'left')",
		"tenant_membership_lifecycle":     "(status = 'left' AND left_at IS NOT NULL) OR (status <> 'left' AND left_at IS NULL)",
	})}
}
