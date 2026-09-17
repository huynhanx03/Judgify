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

// AuthSessionCommandReceipt is the durable idempotency boundary for
// user-initiated session revocation. It stores identifiers and the resulting
// monotonic security version only; credentials, IP metadata, user agents, and
// revoke reasons never enter this table.
type AuthSessionCommandReceipt struct{ ent.Schema }

func (AuthSessionCommandReceipt) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (AuthSessionCommandReceipt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("actor_session_id", uuid.UUID{}).Immutable(),
		field.UUID("target_session_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("idempotency_key", uuid.UUID{}).Immutable(),
		field.Enum("kind").Values("revoke", "logout_all").Immutable(),
		field.Int64("result_security_version").Min(0).Default(0).Immutable(),
		field.Time("expires_at").Immutable(),
	}
}

func (AuthSessionCommandReceipt) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("session_commands").
			Field("user_id").
			Unique().
			Immutable().
			Required(),
		edge.From("actor_session", AuthSession.Type).
			Ref("actor_commands").
			Field("actor_session_id").
			Unique().
			Immutable().
			Required(),
		edge.From("target_session", AuthSession.Type).
			Ref("targeted_commands").
			Field("target_session_id").
			Unique().
			Immutable(),
	}
}

func (AuthSessionCommandReceipt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "idempotency_key").Unique(),
		index.Fields("expires_at"),
	}
}

func (AuthSessionCommandReceipt) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"auth_session_command_shape": "(kind = 'revoke' AND target_session_id IS NOT NULL AND result_security_version = 0) OR (kind = 'logout_all' AND target_session_id IS NULL AND result_security_version > 0)",
		}),
	}
}
