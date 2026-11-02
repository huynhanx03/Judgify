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

// RecoveryContactVerificationToken stores only a one-time keyed token hash
// and the exact optimistic contact version it may activate.
type RecoveryContactVerificationToken struct{ ent.Schema }

func (RecoveryContactVerificationToken) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (RecoveryContactVerificationToken) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("contact_id", uuid.UUID{}).Immutable(),
		field.Int64("contact_version").Min(1).Immutable(),
		field.String("hash_key_id").NotEmpty().MaxLen(128).Immutable(),
		field.Bytes("token_hash").NotEmpty().MaxLen(32).Immutable().Sensitive(),
		field.Enum("purpose").Values("registration", "replacement", "invitation").Immutable(),
		field.Enum("status").
			Values(oneTimeStatusIssued, oneTimeStatusConsumed, oneTimeStatusSuperseded, oneTimeStatusExpired).
			Default(oneTimeStatusIssued),
		field.Time("expires_at").Immutable(),
		field.Time("consumed_at").Optional().Nillable(),
		field.Time("superseded_at").Optional().Nillable(),
		field.Uint32("attempt_count").Default(0),
		field.Time("last_attempt_at").Optional().Nillable(),
	}
}

func (RecoveryContactVerificationToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contact", UserRecoveryContact.Type).
			Ref("verification_tokens").
			Field("contact_id").
			Unique().
			Immutable().
			Required(),
		edge.To("email_deliveries", SecurityEmailDelivery.Type),
	}
}

func (RecoveryContactVerificationToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("hash_key_id", "token_hash").Unique(),
		index.Fields("contact_id", "contact_version", "status"),
		index.Fields("status", "expires_at"),
	}
}

func (RecoveryContactVerificationToken) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"recovery_verification_purpose_closed": "purpose IN ('registration', 'replacement', 'invitation')",
			"recovery_verification_status_closed":  "status IN ('issued', 'consumed', 'superseded', 'expired')",
		}),
	}
}
