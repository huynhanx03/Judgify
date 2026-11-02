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

// RecoveryContactCommandReceipt makes every authenticated recovery mutation
// durable and replay-safe without storing the plaintext destination or body.
// A key belongs to one user and one exact request fingerprint across all
// command kinds, so cross-operation key reuse fails closed.
type RecoveryContactCommandReceipt struct{ ent.Schema }

func (RecoveryContactCommandReceipt) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (RecoveryContactCommandReceipt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("session_id", uuid.UUID{}).Immutable(),
		field.UUID("idempotency_key", uuid.UUID{}).Immutable(),
		field.Enum("kind").Values("replace", "resend", "cancel_pending", "remove").Immutable(),
		field.Bytes("request_fingerprint").NotEmpty().MaxLen(32).Immutable().Sensitive(),
		field.Time("expires_at").Immutable(),
	}
}

func (RecoveryContactCommandReceipt) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("recovery_contact_commands").
			Field("user_id").
			Unique().
			Immutable().
			Required(),
		edge.From("session", AuthSession.Type).
			Ref("recovery_contact_commands").
			Field("session_id").
			Unique().
			Immutable().
			Required(),
	}
}

func (RecoveryContactCommandReceipt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "idempotency_key").Unique(),
		index.Fields("expires_at"),
	}
}

func (RecoveryContactCommandReceipt) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"recovery_contact_command_kind_closed": "kind IN ('replace', 'resend', 'cancel_pending', 'remove')",
		}),
	}
}
