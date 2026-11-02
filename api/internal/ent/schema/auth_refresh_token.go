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

// AuthRefreshToken stores only the keyed hash and rotation lineage of an
// opaque refresh credential. Raw credentials never enter PostgreSQL.
type AuthRefreshToken struct{ ent.Schema }

func (AuthRefreshToken) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (AuthRefreshToken) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("session_id", uuid.UUID{}).Immutable(),
		field.Bytes("token_hash").NotEmpty().MaxLen(32).Unique().Immutable().Sensitive(),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Enum("status").
			Values(refreshStatusIssued, refreshStatusConsumed, refreshStatusRevoked).
			Default(refreshStatusIssued),
		field.Time("issued_at").Immutable(),
		field.Time("expires_at").Immutable(),
		field.Time("consumed_at").Optional().Nillable(),
		field.UUID("replaced_by_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("revoked_at").Optional().Nillable(),
	}
}

func (AuthRefreshToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", AuthSession.Type).
			Ref("refresh_tokens").
			Field("session_id").
			Unique().
			Immutable().
			Required(),
		edge.To("parent", AuthRefreshToken.Type).
			Field("parent_id").
			Unique().
			Immutable().
			From("children"),
		edge.To("replacement", AuthRefreshToken.Type).
			Field("replaced_by_id").
			Unique().
			From("replaced_tokens"),
		edge.To("presented_receipts", AuthRefreshReceipt.Type),
		edge.To("replacement_receipts", AuthRefreshReceipt.Type),
		edge.To("issuance_receipts", AuthSessionIssuanceReceipt.Type),
	}
}

func (AuthRefreshToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id", "status", "expires_at"),
		index.Fields("parent_id"),
		index.Fields("replaced_by_id"),
		index.Fields("status", "expires_at"),
	}
}

func (AuthRefreshToken) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"auth_refresh_status_closed":      "status IN ('issued', 'consumed', 'revoked')",
			"auth_refresh_expiry_after_issue": "expires_at > issued_at",
			"auth_refresh_consumed_complete":  "(status = 'consumed' AND consumed_at IS NOT NULL AND replaced_by_id IS NOT NULL) OR (status <> 'consumed')",
			"auth_refresh_revoked_complete":   "(status = 'revoked' AND revoked_at IS NOT NULL) OR (status <> 'revoked')",
		}),
	}
}
