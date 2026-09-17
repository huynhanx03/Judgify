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

// AuthSession is the server-owned security state for one browser, device, or
// API client. Access tokens carry its ID but never replace this live state.
type AuthSession struct{ ent.Schema }

func (AuthSession) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (AuthSession) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("family_id", uuid.UUID{}).Immutable(),
		field.Enum("status").
			Values(sessionStatusActive, sessionStatusRevoked, sessionStatusExpired).
			Default(sessionStatusActive),
		field.Int64("user_security_version").Min(1).Immutable(),
		field.Time("last_seen_at"),
		field.Time("last_reauthenticated_at"),
		field.Time("idle_expires_at"),
		field.Time("absolute_expires_at").Immutable(),
		field.Time("revoked_at").Optional().Nillable(),
		field.String("revoke_reason").MaxLen(256).Optional().Nillable(),
		field.Bytes("ip_hash").MaxLen(32).Optional().Nillable().Sensitive(),
		field.String("user_agent").MaxLen(512).Optional().Nillable().Sensitive(),
		field.String("created_cid").NotEmpty().MaxLen(128).Immutable(),
	}
}

func (AuthSession) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("auth_sessions").
			Field("user_id").
			Unique().
			Immutable().
			Required(),
		edge.To("refresh_tokens", AuthRefreshToken.Type),
		edge.To("refresh_receipts", AuthRefreshReceipt.Type),
		edge.To("issuance_receipts", AuthSessionIssuanceReceipt.Type),
		edge.To("oauth_flows", OAuthFlow.Type),
		edge.To("oauth_completion_tickets", OAuthCompletionTicket.Type),
		edge.To("recovery_contact_commands", RecoveryContactCommandReceipt.Type),
		edge.To("actor_commands", AuthSessionCommandReceipt.Type),
		edge.To("targeted_commands", AuthSessionCommandReceipt.Type).
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (AuthSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "status"),
		index.Fields("family_id"),
		index.Fields("status", "idle_expires_at"),
		index.Fields("status", "absolute_expires_at"),
		index.Fields("user_id", "created_at", "id"),
	}
}

func (AuthSession) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"auth_session_status_closed":       "status IN ('active', 'revoked', 'expired')",
			"auth_session_expiry_order":        "idle_expires_at <= absolute_expires_at",
			"auth_session_revocation_complete": "(status = 'revoked' AND revoked_at IS NOT NULL AND revoke_reason IS NOT NULL) OR (status <> 'revoked')",
		}),
	}
}
