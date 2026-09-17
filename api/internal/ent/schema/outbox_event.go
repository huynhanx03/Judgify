package schema

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// OutboxEvent stores immutable integration-event identity and payload beside
// mutable, fenced delivery state. Domain writes use the transaction-bound
// appender; a later adapter owns claiming and maps rows to go-common outbox
// contracts.
type OutboxEvent struct {
	ent.Schema
}

// Fields separates immutable event facts from the mutable relay state machine.
func (OutboxEvent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(commonent.NewUUID).
			Immutable(),
		field.String("event_type").
			NotEmpty().
			MaxLen(128).
			Immutable(),
		field.String("aggregate_type").
			NotEmpty().
			MaxLen(64).
			Immutable(),
		field.UUID("aggregate_id", uuid.UUID{}).
			Immutable(),
		field.Int64("aggregate_version").
			Positive().
			Immutable(),
		field.String("correlation_id").
			NotEmpty().
			MaxLen(128).
			Immutable(),
		field.UUID("causation_id", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
		field.JSON("payload", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}).
			Immutable(),
		field.Time("occurred_at").
			Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),

		field.Enum("state").
			Values("pending", "leased", "published", "dead").
			Default("pending"),
		field.Uint32("attempt_count").
			Default(0),
		field.String("lease_owner").
			MaxLen(256).
			Optional().
			Nillable(),
		field.String("lease_token").
			MaxLen(256).
			Optional().
			Nillable(),
		field.Time("leased_until").
			Optional().
			Nillable(),
		field.Time("next_attempt_at"),
		field.Time("last_attempt_at").
			Optional().
			Nillable(),
		field.Time("published_at").
			Optional().
			Nillable(),
		field.Time("dead_at").
			Optional().
			Nillable(),
		field.String("publish_reference").
			MaxLen(256).
			Optional().
			Nillable(),
		field.String("last_error_code").
			MaxLen(64).
			Optional().
			Nillable(),
		field.Bool("ambiguous").
			Default(false),
	}
}

// Indexes support deterministic bounded claims, expired-lease recovery,
// terminal retention, aggregate replay, and correlation-based tracing.
func (OutboxEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("event_type", "aggregate_type", "aggregate_id", "aggregate_version").Unique(),
		index.Fields("state", "next_attempt_at", "occurred_at", "id"),
		index.Fields("state", "leased_until"),
		index.Fields("state", "published_at"),
		index.Fields("state", "dead_at"),
		index.Fields("aggregate_type", "aggregate_id", "aggregate_version"),
		index.Fields("correlation_id", "occurred_at", "id"),
	}
}

// Annotations keep invalid relay-state combinations from being committed even
// if a future dispatcher bypasses an application-level transition helper.
func (OutboxEvent) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"outbox_lease_fields_complete": `(
				(lease_owner IS NULL AND lease_token IS NULL AND leased_until IS NULL) OR
				(lease_owner IS NOT NULL AND lease_token IS NOT NULL AND leased_until IS NOT NULL)
			)`,
			"outbox_state_timestamps": `(
				(state = 'pending' AND lease_owner IS NULL AND published_at IS NULL AND dead_at IS NULL) OR
				(state = 'leased' AND lease_owner IS NOT NULL AND published_at IS NULL AND dead_at IS NULL) OR
				(state = 'published' AND lease_owner IS NULL AND published_at IS NOT NULL AND dead_at IS NULL) OR
				(state = 'dead' AND lease_owner IS NULL AND published_at IS NULL AND dead_at IS NOT NULL)
			)`,
		}),
	}
}
