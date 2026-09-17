package schema

import (
	"encoding/json"

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

// Operation is the durable, domain-neutral control record for bounded work.
// It stores data and checkpoints only; executable callbacks are registered by
// application wiring on every process boot and are never persisted.
type Operation struct{ ent.Schema }

func (Operation) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (Operation) Fields() []ent.Field {
	return []ent.Field{
		field.String("kind").NotEmpty().MaxLen(128).Immutable(),
		field.UUID("idempotency_key", uuid.UUID{}).Immutable(),
		field.Bytes("request_hash").NotEmpty().MaxLen(32).Immutable().Sensitive(),
		field.JSON("request_payload", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}).
			Immutable().
			Sensitive(),
		field.UUID("actor_user_id", uuid.UUID{}).Immutable(),
		field.UUID("actor_session_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("audit_id", uuid.UUID{}).Immutable(),
		field.String("reason").NotEmpty().MaxLen(1024).Immutable().Sensitive(),
		field.String("correlation_id").NotEmpty().MaxLen(128).Immutable(),
		field.Enum("status").
			Values("pending", "running", "paused", "completed", "failed", "cancelled").
			Default("pending"),
		field.Enum("requested_action").
			Values("none", "pause", "cancel").
			Default("none"),
		field.Int64("total_count").NonNegative().Immutable(),
		field.Int64("processed_count").NonNegative().Default(0),
		field.Int64("succeeded_count").NonNegative().Default(0),
		field.Int64("failed_count").NonNegative().Default(0),
		field.Int64("next_batch_sequence").Min(1).Default(1),
		field.Int("batch_size").Min(1).Max(1000).Immutable(),
		field.JSON("checkpoint", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}),
		field.Int64("version").Min(1).Default(1),
		field.String("lease_owner").MaxLen(128).Optional().Nillable(),
		field.UUID("lease_token", uuid.UUID{}).Optional().Nillable(),
		field.Time("leased_until").Optional().Nillable(),
		field.Time("not_before").Optional().Nillable().Immutable(),
		field.Time("available_at"),
		field.Uint32("attempt_count").Default(0),
		field.String("last_error_code").MaxLen(64).Optional().Nillable(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("finished_at").Optional().Nillable(),
	}
}

func (Operation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("batches", OperationBatch.Type),
		edge.To("control_receipts", OperationControlReceipt.Type),
	}
}

func (Operation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("kind", "idempotency_key").Unique(),
		index.Fields("not_before", "created_at", "id"),
		index.Fields("status", "available_at", "created_at", "id"),
		index.Fields("status", "leased_until"),
		index.Fields("actor_user_id", "created_at", "id"),
		index.Fields("correlation_id", "created_at", "id"),
		index.Fields("audit_id").Unique(),
	}
}

func (Operation) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"operation_request_hash_size": "octet_length(request_hash) = 32",
			"operation_progress_exact":    "processed_count = succeeded_count + failed_count AND processed_count <= total_count",
			"operation_lease_complete":    "(lease_owner IS NULL AND lease_token IS NULL AND leased_until IS NULL) OR (lease_owner IS NOT NULL AND lease_token IS NOT NULL AND leased_until IS NOT NULL)",
			"operation_lease_state":       "(lease_owner IS NULL) OR status = 'running'",
			"operation_schedule_floor":    "not_before IS NULL OR available_at >= not_before",
			"operation_pending_state":     "status <> 'pending' OR (processed_count = 0 AND lease_owner IS NULL AND started_at IS NULL AND finished_at IS NULL)",
			"operation_paused_state":      "status <> 'paused' OR (lease_owner IS NULL AND requested_action = 'none' AND finished_at IS NULL)",
			"operation_completed_state":   "status <> 'completed' OR (processed_count = total_count AND lease_owner IS NULL AND requested_action = 'none' AND finished_at IS NOT NULL AND last_error_code IS NULL)",
			"operation_failed_state":      "status <> 'failed' OR (lease_owner IS NULL AND requested_action = 'none' AND finished_at IS NOT NULL AND last_error_code IS NOT NULL)",
			"operation_cancelled_state":   "status <> 'cancelled' OR (lease_owner IS NULL AND requested_action = 'none' AND finished_at IS NOT NULL)",
			"operation_nonterminal_state": "status IN ('completed', 'failed', 'cancelled') OR finished_at IS NULL",
		}),
	}
}
