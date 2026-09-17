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

const (
	securityEmailKindContactVerification = "recovery_contact_verification"
	securityEmailKindPasswordReset       = "password_reset"
	securityEmailKindAccountInvitation   = "account_invitation"
)

// SecurityEmailDelivery is a durable, leased delivery intent. It deliberately
// exposes only low-cardinality operational state; recipient and raw credential
// bytes exist solely inside short-lived AEAD ciphertext.
type SecurityEmailDelivery struct{ ent.Schema }

func (SecurityEmailDelivery) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (SecurityEmailDelivery) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("kind").Values(
			securityEmailKindContactVerification,
			securityEmailKindPasswordReset,
			securityEmailKindAccountInvitation,
		).Immutable(),
		field.UUID("verification_token_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("password_reset_token_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("provider_idempotency_key").NotEmpty().MaxLen(128).Unique().Immutable(),
		field.Bytes("payload_ciphertext").NotEmpty().Sensitive(),
		field.String("key_id").NotEmpty().MaxLen(128),
		field.Time("payload_expires_at").Immutable(),
		field.Enum("status").
			Values(
				deliveryStatusReady,
				deliveryStatusLeased,
				deliveryStatusSent,
				deliveryStatusDead,
				deliveryStatusCancelled,
			).
			Default(deliveryStatusReady),
		field.Uint32("attempt_count").Default(0),
		field.Time("next_attempt_at"),
		field.String("lease_owner").MaxLen(128).Optional().Nillable(),
		field.UUID("lease_token", uuid.UUID{}).Optional().Nillable(),
		field.Time("leased_until").Optional().Nillable(),
		field.Time("sent_at").Optional().Nillable(),
		field.Time("dead_at").Optional().Nillable(),
		field.Time("cancelled_at").Optional().Nillable(),
		field.String("last_error_class").MaxLen(64).Optional().Nillable(),
		field.Time("ciphertext_erased_at").Optional().Nillable(),
	}
}

func (SecurityEmailDelivery) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("verification_token", RecoveryContactVerificationToken.Type).
			Ref("email_deliveries").
			Field("verification_token_id").
			Unique().
			Immutable(),
		edge.From("password_reset_token", PasswordResetToken.Type).
			Ref("email_deliveries").
			Field("password_reset_token_id").
			Unique().
			Immutable(),
	}
}

func (SecurityEmailDelivery) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "next_attempt_at", "created_at", "id"),
		index.Fields("status", "leased_until"),
		index.Fields("payload_expires_at"),
		index.Fields("verification_token_id"),
		index.Fields("password_reset_token_id"),
	}
}

func (SecurityEmailDelivery) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"security_email_kind_closed":         "kind IN ('recovery_contact_verification', 'password_reset', 'account_invitation')",
			"security_email_status_closed":       "status IN ('ready', 'leased', 'sent', 'dead', 'cancelled')",
			"security_email_intent_matches_kind": "(kind IN ('recovery_contact_verification', 'account_invitation') AND verification_token_id IS NOT NULL AND password_reset_token_id IS NULL) OR (kind = 'password_reset' AND password_reset_token_id IS NOT NULL AND verification_token_id IS NULL)",
			"security_email_lease_complete":      "(lease_owner IS NULL AND lease_token IS NULL AND leased_until IS NULL) OR (lease_owner IS NOT NULL AND lease_token IS NOT NULL AND leased_until IS NOT NULL)",
			"security_email_terminal_timestamp":  "(status = 'sent' AND sent_at IS NOT NULL) OR (status = 'dead' AND dead_at IS NOT NULL) OR (status = 'cancelled' AND cancelled_at IS NOT NULL) OR status IN ('ready', 'leased')",
			"security_email_state_shape": "(" +
				"status = 'ready' AND lease_owner IS NULL AND lease_token IS NULL AND leased_until IS NULL " +
				"AND sent_at IS NULL AND dead_at IS NULL AND cancelled_at IS NULL AND ciphertext_erased_at IS NULL " +
				"AND octet_length(payload_ciphertext) > 1" +
				") OR (" +
				"status = 'leased' AND lease_owner IS NOT NULL AND lease_token IS NOT NULL AND leased_until IS NOT NULL " +
				"AND attempt_count > 0 AND sent_at IS NULL AND dead_at IS NULL AND cancelled_at IS NULL " +
				"AND ciphertext_erased_at IS NULL AND octet_length(payload_ciphertext) > 1" +
				") OR (" +
				"status = 'sent' AND lease_owner IS NULL AND lease_token IS NULL AND leased_until IS NULL " +
				"AND sent_at IS NOT NULL AND dead_at IS NULL AND cancelled_at IS NULL " +
				"AND ciphertext_erased_at IS NOT NULL AND octet_length(payload_ciphertext) = 1" +
				") OR (" +
				"status = 'dead' AND lease_owner IS NULL AND lease_token IS NULL AND leased_until IS NULL " +
				"AND sent_at IS NULL AND dead_at IS NOT NULL AND cancelled_at IS NULL " +
				"AND ciphertext_erased_at IS NOT NULL AND octet_length(payload_ciphertext) = 1" +
				") OR (" +
				"status = 'cancelled' AND lease_owner IS NULL AND lease_token IS NULL AND leased_until IS NULL " +
				"AND sent_at IS NULL AND dead_at IS NULL AND cancelled_at IS NOT NULL " +
				"AND ciphertext_erased_at IS NOT NULL AND octet_length(payload_ciphertext) = 1" +
				")",
		}),
	}
}
