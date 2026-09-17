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

// UserRecoveryContact stores an encrypted recovery destination. Hashes are
// keyed and ciphertext is versioned so neither DB access nor logs disclose the
// address.
type UserRecoveryContact struct{ ent.Schema }

func (UserRecoveryContact) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (UserRecoveryContact) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.Enum("kind").Values("email").Immutable(),
		field.Enum("purpose").
			Values(contactPurposeRegistration, contactPurposeReplacement, contactPurposeInvitation).
			Immutable(),
		field.Bytes("lookup_hash").NotEmpty().MaxLen(32).Immutable().Sensitive(),
		field.Bytes("fingerprint_hash").NotEmpty().MaxLen(32).Immutable().Sensitive(),
		field.Bytes("address_ciphertext").NotEmpty().Sensitive(),
		field.String("key_id").NotEmpty().MaxLen(128),
		field.Enum("status").
			Values(contactStatusPending, contactStatusVerified, contactStatusRevoked).
			Default(contactStatusPending),
		field.Int64("version").Min(1).Default(1),
		field.Time("verified_at").Optional().Nillable(),
		field.Time("revoked_at").Optional().Nillable(),
		field.UUID("replaced_contact_id", uuid.UUID{}).Optional().Nillable(),
	}
}

func (UserRecoveryContact) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("recovery_contacts").
			Field("user_id").
			Unique().
			Immutable().
			Required(),
		edge.To("verification_tokens", RecoveryContactVerificationToken.Type),
		edge.To("password_reset_tokens", PasswordResetToken.Type),
	}
}

func (UserRecoveryContact) Indexes() []ent.Index {
	return []ent.Index{
		// A recovery destination is not a login identifier. The same household
		// mailbox may legitimately protect multiple accounts, so this lookup is
		// indexed but deliberately not globally unique.
		index.Fields("lookup_hash"),
		index.Fields("fingerprint_hash"),
		index.Fields("user_id", "kind").
			Unique().
			StorageKey("userrecoverycontact_user_kind_verified").
			Annotations(entsql.IndexWhere("status = 'verified'")),
		index.Fields("user_id", "kind").
			Unique().
			StorageKey("userrecoverycontact_user_kind_pending").
			Annotations(entsql.IndexWhere("status = 'pending'")),
		index.Fields("user_id", "status"),
	}
}

func (UserRecoveryContact) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"recovery_contact_kind_closed":    "kind = 'email'",
			"recovery_contact_purpose_closed": "purpose IN ('registration', 'replacement', 'invitation')",
			"recovery_contact_status_closed":  "status IN ('pending', 'verified', 'revoked')",
			"recovery_contact_verified_at":    "(status = 'verified' AND verified_at IS NOT NULL) OR status <> 'verified'",
			"recovery_contact_revoked_at":     "(status = 'revoked' AND revoked_at IS NOT NULL) OR status <> 'revoked'",
		}),
	}
}
