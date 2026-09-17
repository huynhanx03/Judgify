package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// AuthRefreshReceipt recovers the exact committed replacement after an
// unknown HTTP response outcome. Its ciphertext has a deliberately short
// retention window and is erased independently of the audit trail.
type AuthRefreshReceipt struct{ ent.Schema }

func (AuthRefreshReceipt) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (AuthRefreshReceipt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("session_id", uuid.UUID{}).Immutable(),
		field.UUID("presented_token_id", uuid.UUID{}).Immutable(),
		field.UUID("idempotency_key", uuid.UUID{}).Immutable(),
		field.UUID("replacement_token_id", uuid.UUID{}).Immutable(),
		field.Bytes("recovery_ciphertext").NotEmpty().Immutable().Sensitive(),
		field.String("key_id").NotEmpty().MaxLen(128).Immutable(),
		field.Time("expires_at").Immutable(),
		field.Time("recovered_at").Optional().Nillable(),
		field.Time("erased_at").Optional().Nillable(),
	}
}

func (AuthRefreshReceipt) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", AuthSession.Type).
			Ref("refresh_receipts").
			Field("session_id").
			Unique().
			Immutable().
			Required(),
		edge.From("presented_token", AuthRefreshToken.Type).
			Ref("presented_receipts").
			Field("presented_token_id").
			Unique().
			Immutable().
			Required(),
		edge.From("replacement_token", AuthRefreshToken.Type).
			Ref("replacement_receipts").
			Field("replacement_token_id").
			Unique().
			Immutable().
			Required(),
	}
}

func (AuthRefreshReceipt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id", "presented_token_id", "idempotency_key").Unique(),
		index.Fields("expires_at"),
		index.Fields("replacement_token_id").Unique(),
	}
}
