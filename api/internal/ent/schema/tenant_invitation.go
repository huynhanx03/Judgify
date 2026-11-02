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

// TenantInvitation invites an existing global account into one tenant.
type TenantInvitation struct{ ent.Schema }

func (TenantInvitation) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (TenantInvitation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("tenant_id", uuid.UUID{}),
		field.UUID("invited_user_id", uuid.UUID{}),
		field.UUID("invited_by", uuid.UUID{}),
		field.Enum("status").Values("pending", "accepted", "declined", "revoked", "expired").Default("pending"),
		field.Time("expires_at"),
		field.Time("responded_at").Optional().Nillable(),
	}
}

func (TenantInvitation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Tenant.Type).Ref("invitations").Field("tenant_id").Unique().Required(),
		edge.From("invitee", User.Type).Ref("tenant_invitations").Field("invited_user_id").Unique().Required(),
		edge.From("inviter", User.Type).Ref("sent_tenant_invitations").Field("invited_by").Unique().Required(),
	}
}

func (TenantInvitation) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "invited_user_id", "status"), index.Fields("invited_user_id", "status")}
}

func (TenantInvitation) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Check("status IN ('pending', 'accepted', 'declined', 'revoked', 'expired')")}
}
