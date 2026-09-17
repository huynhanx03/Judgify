package schema

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// OperationControlReceipt makes pause/resume/cancel exact-replay safe even
// after newer control commands advance the operation version.
type OperationControlReceipt struct{ ent.Schema }

func (OperationControlReceipt) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (OperationControlReceipt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("operation_id", uuid.UUID{}).Immutable(),
		field.UUID("idempotency_key", uuid.UUID{}).Immutable(),
		field.Bytes("request_hash").NotEmpty().MaxLen(32).Immutable().Sensitive(),
		field.Enum("action").Values("pause", "resume", "cancel").Immutable(),
		field.Int64("expected_version").Min(1).Immutable(),
		field.Int64("resulting_version").Min(1).Immutable(),
		field.Enum("resulting_status").
			Values("pending", "running", "paused", "completed", "failed", "cancelled").
			Immutable(),
		field.JSON("result_projection", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}).
			Immutable(),
		field.UUID("actor_user_id", uuid.UUID{}).Immutable(),
		field.UUID("actor_session_id", uuid.UUID{}).Immutable(),
		field.UUID("audit_id", uuid.UUID{}).Unique().Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (OperationControlReceipt) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("operation", Operation.Type).
			Ref("control_receipts").
			Field("operation_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (OperationControlReceipt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("operation_id", "idempotency_key").Unique(),
		index.Fields("operation_id", "created_at", "id"),
	}
}

func (OperationControlReceipt) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"operation_control_request_hash_size": "octet_length(request_hash) = 32",
			"operation_control_version_advanced":  "resulting_version > expected_version",
			"operation_control_result_object":     "jsonb_typeof(result_projection) = 'object'",
		}),
	}
}
