package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// Tenant is an independently owned Judgify community.
type Tenant struct{ ent.Schema }

func (Tenant) Mixin() []ent.Mixin { return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}} }

func (Tenant) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty().MaxLen(120),
		field.String("slug").NotEmpty().MaxLen(80).Immutable(),
		field.Enum("status").Values("active", "suspended", "archived").Default("active"),
		field.Time("name_changed_at").Optional().Nillable(),
	}
}

func (Tenant) Indexes() []ent.Index {
	return []ent.Index{index.Fields("slug").Unique(), index.Fields("status")}
}

func (Tenant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("memberships", TenantMembership.Type),
		edge.To("invitations", TenantInvitation.Type),
		edge.To("roles", TenantRole.Type),
		edge.To("outbox_events", OutboxEvent.Type),
		edge.To("audit_logs", AuditLog.Type),
	}
}

func (Tenant) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"tenant_name_nonempty":     "length(btrim(name)) > 0",
		"tenant_slug_nonempty":     "length(btrim(slug)) > 0",
		"tenant_rename_not_future": "name_changed_at IS NULL OR name_changed_at <= CURRENT_TIMESTAMP",
	})}
}

const tenantRenameCooldown = 365 * 24 * time.Hour
