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

// PasswordResetToken is a single-use reset intent. Only the keyed token hash
// is stored; successful consumption is coupled to a security-version bump.
type PasswordResetToken struct{ ent.Schema }

func (PasswordResetToken) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (PasswordResetToken) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("contact_id", uuid.UUID{}).Immutable(),
		field.Int64("contact_version").Min(1).Immutable(),
		field.Int64("user_security_version").Min(1).Immutable(),
		field.String("hash_key_id").NotEmpty().MaxLen(128).Immutable(),
		field.Bytes("token_hash").NotEmpty().MaxLen(32).Immutable().Sensitive(),
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

func (PasswordResetToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("password_reset_tokens").
			Field("user_id").
			Unique().
			Immutable().
			Required(),
		edge.From("contact", UserRecoveryContact.Type).
			Ref("password_reset_tokens").
			Field("contact_id").
			Unique().
			Immutable().
			Required(),
		edge.To("email_deliveries", SecurityEmailDelivery.Type),
	}
}

func (PasswordResetToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("hash_key_id", "token_hash").Unique(),
		index.Fields("user_id", "status", "expires_at"),
		index.Fields("contact_id", "contact_version", "status"),
		index.Fields("status", "expires_at"),
	}
}

func (PasswordResetToken) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"password_reset_status_closed": "status IN ('issued', 'consumed', 'superseded', 'expired')",
		}),
	}
}
