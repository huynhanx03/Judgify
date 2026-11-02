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

// OAuthFlow owns a one-time, purpose-bound provider authorization attempt.
// State is hashed and the PKCE verifier is encrypted, never stored raw.
type OAuthFlow struct{ ent.Schema }

func (OAuthFlow) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (OAuthFlow) Fields() []ent.Field {
	return []ent.Field{
		field.String("provider").NotEmpty().MaxLen(64).Immutable(),
		field.Enum("purpose").Values("login", "link").Immutable(),
		field.Bytes("state_hash").NotEmpty().MaxLen(32).Unique().Immutable().Sensitive(),
		field.UUID("pre_session_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("session_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("redirect_uri").NotEmpty().MaxLen(2048).Immutable().Sensitive(),
		field.Bytes("pkce_verifier_ciphertext").NotEmpty().Immutable().Sensitive(),
		field.String("key_id").NotEmpty().MaxLen(128).Immutable(),
		field.Enum("status").
			Values(oauthFlowStatusPending, oauthFlowStatusConsumed, oauthFlowStatusExpired).
			Default(oauthFlowStatusPending),
		field.Time("expires_at").Immutable(),
		field.Time("consumed_at").Optional().Nillable(),
	}
}

func (OAuthFlow) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", AuthSession.Type).
			Ref("oauth_flows").
			Field("session_id").
			Unique().
			Immutable(),
		edge.To("completion_tickets", OAuthCompletionTicket.Type),
	}
}

func (OAuthFlow) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider", "status", "expires_at"),
		index.Fields("session_id", "purpose", "status"),
		index.Fields("pre_session_id", "purpose", "status"),
		index.Fields("status", "expires_at"),
	}
}

func (OAuthFlow) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		&entsql.Annotation{Table: "oauth_flows", Checks: map[string]string{
			"oauth_flow_purpose_closed":  "purpose IN ('login', 'link')",
			"oauth_flow_status_closed":   "status IN ('pending', 'consumed', 'expired')",
			"oauth_flow_purpose_binding": "(purpose = 'login' AND pre_session_id IS NOT NULL AND session_id IS NULL) OR (purpose = 'link' AND session_id IS NOT NULL)",
		}},
	}
}
