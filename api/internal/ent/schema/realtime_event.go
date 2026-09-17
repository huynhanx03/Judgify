package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// RealtimeEvent is a bounded, immutable delivery projection. PostgreSQL is
// authoritative; the in-memory hub is only the live fan-out path.
type RealtimeEvent struct{ ent.Schema }

func (RealtimeEvent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.String("event_id").NotEmpty().MaxLen(128).Immutable(),
		field.String("topic").NotEmpty().MaxLen(256).Immutable(),
		field.Int64("sequence").Positive().Immutable(),
		field.String("type").NotEmpty().MaxLen(128).Immutable(),
		field.Int64("aggregate_version").Min(0).Immutable(),
		field.Time("occurred_at").Immutable(),
		field.JSON("data", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).
			Immutable(),
		field.Enum("delivery_class").
			Values("latest_state", "ephemeral", "critical").Immutable(),
		field.String("coalesce_key").MaxLen(256).Optional().Nillable().Immutable(),
		field.Bytes("evidence_hash").MaxLen(32).Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (RealtimeEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sequence").Unique(),
		index.Fields("topic", "event_id").Unique(),
		index.Fields("topic", "sequence"),
		index.Fields("created_at", "sequence"),
		index.Fields("created_at", "id"),
	}
}

func (RealtimeEvent) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"realtime_event_sequence_positive": "sequence > 0",
		"realtime_event_hash_length":       "octet_length(evidence_hash) = 32",
	})}
}
