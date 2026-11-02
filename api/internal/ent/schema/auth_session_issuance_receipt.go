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
	issuancePurposePasswordLogin        = "password_login"
	issuancePurposePasswordRegistration = "password_registration"
	issuancePurposeOAuthLogin           = "oauth_login"
	issuancePurposeOAuthRegistration    = "oauth_registration"
)

// AuthSessionIssuanceReceipt makes account/session establishment recoverable
// across commit followed by a lost HTTP response. Each row binds the complete
// logical command identity; the raw refresh token and CSRF nonce exist only in
// short-lived AEAD ciphertext.
type AuthSessionIssuanceReceipt struct{ ent.Schema }

func (AuthSessionIssuanceReceipt) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (AuthSessionIssuanceReceipt) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("purpose").Values(
			issuancePurposePasswordLogin,
			issuancePurposePasswordRegistration,
			issuancePurposeOAuthLogin,
			issuancePurposeOAuthRegistration,
		).Immutable(),
		field.UUID("request_or_ticket_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("user_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("pre_session_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Bytes("request_fingerprint").MaxLen(32).Optional().Nillable().Immutable().Sensitive(),
		field.String("fingerprint_key_id").MaxLen(128).Optional().Nillable().Immutable(),
		field.UUID("idempotency_key", uuid.UUID{}).Immutable(),
		field.UUID("session_id", uuid.UUID{}).Immutable(),
		field.UUID("refresh_token_id", uuid.UUID{}).Immutable(),
		field.UUID("trait_offer_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Bytes("offer_checksum").MaxLen(32).Optional().Nillable().Immutable(),
		field.Bytes("recovery_ciphertext").NotEmpty().Immutable().Sensitive(),
		field.String("key_id").NotEmpty().MaxLen(128).Immutable(),
		field.Time("expires_at").Immutable(),
		field.Time("recovered_at").Optional().Nillable(),
		field.Time("erased_at").Optional().Nillable(),
	}
}

func (AuthSessionIssuanceReceipt) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("session_issuance_receipts").
			Field("user_id").
			Unique().
			Immutable(),
		edge.From("session", AuthSession.Type).
			Ref("issuance_receipts").
			Field("session_id").
			Unique().
			Immutable().
			Required(),
		edge.From("refresh_token", AuthRefreshToken.Type).
			Ref("issuance_receipts").
			Field("refresh_token_id").
			Unique().
			Immutable().
			Required(),
		edge.From("trait_offer", OnboardingTraitOffer.Type).
			Ref("issuance_receipts").
			Field("trait_offer_id").
			Unique().
			Immutable(),
	}
}

func (AuthSessionIssuanceReceipt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "pre_session_id", "idempotency_key").
			Unique().
			Annotations(entsql.IndexWhere("purpose = 'password_login'")),
		index.Fields("purpose", "request_or_ticket_id", "idempotency_key").
			Unique().
			Annotations(entsql.IndexWhere("purpose <> 'password_login'")),
		index.Fields("session_id").Unique(),
		index.Fields("refresh_token_id").Unique(),
		index.Fields("trait_offer_id").Unique().
			Annotations(entsql.IndexWhere("trait_offer_id IS NOT NULL")),
		index.Fields("expires_at"),
	}
}

func (AuthSessionIssuanceReceipt) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"auth_issuance_purpose_closed":          "purpose IN ('password_login', 'password_registration', 'oauth_login', 'oauth_registration')",
			"auth_issuance_password_login_identity": "(purpose = 'password_login' AND user_id IS NOT NULL AND pre_session_id IS NOT NULL) OR purpose <> 'password_login'",
			"auth_issuance_external_identity":       "(purpose = 'password_login') OR request_or_ticket_id IS NOT NULL",
			"auth_issuance_fingerprint_pair":        "(request_fingerprint IS NULL AND fingerprint_key_id IS NULL) OR (request_fingerprint IS NOT NULL AND fingerprint_key_id IS NOT NULL)",
			"auth_issuance_offer_pair":              "(trait_offer_id IS NULL AND offer_checksum IS NULL) OR (trait_offer_id IS NOT NULL AND offer_checksum IS NOT NULL)",
			"auth_issuance_offer_checksum_shape":    "offer_checksum IS NULL OR (octet_length(offer_checksum) = 32 AND offer_checksum <> decode(repeat('00', 32), 'hex'))",
			"auth_issuance_registration_offer":      "(purpose IN ('password_registration', 'oauth_registration') AND trait_offer_id IS NOT NULL) OR (purpose NOT IN ('password_registration', 'oauth_registration') AND trait_offer_id IS NULL)",
		}),
	}
}
