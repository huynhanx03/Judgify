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

// OAuthCompletionTicket is the only artifact produced by a successful OAuth
// callback. Finalization consumes it for exactly one declared purpose; the
// callback itself never creates or links an account.
type OAuthCompletionTicket struct{ ent.Schema }

func (OAuthCompletionTicket) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (OAuthCompletionTicket) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("flow_id", uuid.UUID{}).Unique().Immutable(),
		field.Bytes("ticket_hash").NotEmpty().MaxLen(32).Unique().Immutable().Sensitive(),
		field.String("provider").NotEmpty().MaxLen(64).Immutable(),
		field.String("provider_subject").NotEmpty().MaxLen(256).Immutable().Sensitive(),
		field.Enum("purpose").Values("login", "link").Immutable(),
		field.UUID("pre_session_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("bound_user_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("bound_session_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Bytes("provider_profile_ciphertext").NotEmpty().Immutable().Sensitive(),
		field.String("key_id").NotEmpty().MaxLen(128).Immutable(),
		field.Enum("status").
			Values(oneTimeStatusIssued, oneTimeStatusConsumed, oneTimeStatusExpired).
			Default(oneTimeStatusIssued),
		field.Time("expires_at").Immutable(),
		field.Time("consumed_at").Optional().Nillable(),
	}
}

func (OAuthCompletionTicket) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("flow", OAuthFlow.Type).
			Ref("completion_tickets").
			Field("flow_id").
			Unique().
			Immutable().
			Required(),
		edge.From("bound_user", User.Type).
			Ref("oauth_completion_tickets").
			Field("bound_user_id").
			Unique().
			Immutable(),
		edge.From("bound_session", AuthSession.Type).
			Ref("oauth_completion_tickets").
			Field("bound_session_id").
			Unique().
			Immutable(),
	}
}

func (OAuthCompletionTicket) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider", "provider_subject"),
		index.Fields("bound_session_id", "purpose", "status"),
		index.Fields("pre_session_id", "purpose", "status"),
		index.Fields("status", "expires_at"),
	}
}

func (OAuthCompletionTicket) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		&entsql.Annotation{Table: "oauth_completion_tickets", Checks: map[string]string{
			"oauth_ticket_purpose_closed":  "purpose IN ('login', 'link')",
			"oauth_ticket_status_closed":   "status IN ('issued', 'consumed', 'expired')",
			"oauth_ticket_purpose_binding": "(purpose = 'login' AND pre_session_id IS NOT NULL AND bound_session_id IS NULL) OR (purpose = 'link' AND bound_user_id IS NOT NULL AND bound_session_id IS NOT NULL)",
		}},
	}
}
