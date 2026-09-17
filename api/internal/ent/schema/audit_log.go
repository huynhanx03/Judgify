package schema

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// AuditLog is the immutable, append-only record of a security-sensitive or
// administrative action. Application code writes it only through the shared
// transaction-bound audit repository.
type AuditLog struct {
	ent.Schema
}

func (AuditLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		commonent.AppendOnlyMixin{},
	}
}

// Fields intentionally avoids TimeMixin because audit rows have no mutable
// updated_at column. Every persisted field is immutable at the Ent boundary.
func (AuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(commonent.NewUUID).
			Immutable(),
		field.UUID("actor_user_id", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
		field.UUID("actor_session_id", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
		field.String("action").
			NotEmpty().
			MaxLen(128).
			Immutable(),
		field.String("resource").
			NotEmpty().
			MaxLen(128).
			Immutable(),
		field.String("resource_id").
			MaxLen(256).
			Immutable(),
		field.String("reason").
			NotEmpty().
			MaxLen(1024).
			Immutable(),
		field.JSON("before_data", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}).
			Immutable(),
		field.JSON("after_data", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}).
			Immutable(),
		field.String("correlation_id").
			NotEmpty().
			MaxLen(128).
			Immutable(),
		field.Time("occurred_at").
			Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

// Indexes support authorized audit search without allowing an unbounded scan.
func (AuditLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("occurred_at", "id"),
		index.Fields("action", "occurred_at", "id"),
		index.Fields("resource", "resource_id", "occurred_at", "id"),
		index.Fields("resource_id", "occurred_at", "id"),
		index.Fields("actor_user_id", "occurred_at", "id"),
		index.Fields("correlation_id", "occurred_at", "id"),
	}
}
