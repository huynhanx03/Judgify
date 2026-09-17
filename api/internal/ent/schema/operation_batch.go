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

// OperationBatch is the immutable checkpoint receipt for one committed batch.
// If a domain mutation rolls back, this row rolls back with it, so reclaiming
// an expired lease resumes from the last exact committed checkpoint.
type OperationBatch struct{ ent.Schema }

func (OperationBatch) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (OperationBatch) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("operation_id", uuid.UUID{}).Immutable(),
		field.Int64("sequence").Positive().Immutable(),
		field.UUID("lease_token", uuid.UUID{}).Immutable(),
		field.JSON("input_checkpoint", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}).
			Immutable(),
		field.JSON("output_checkpoint", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}).
			Immutable(),
		field.Int64("processed_count").NonNegative().Immutable(),
		field.Int64("succeeded_count").NonNegative().Immutable(),
		field.Int64("failed_count").NonNegative().Immutable(),
		field.JSON("failure_summary", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}).
			Immutable(),
		field.Time("started_at").Immutable(),
		field.Time("completed_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (OperationBatch) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("operation", Operation.Type).
			Ref("batches").
			Field("operation_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (OperationBatch) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("operation_id", "sequence").Unique(),
		index.Fields("operation_id", "completed_at", "id"),
	}
}

func (OperationBatch) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"operation_batch_progress_exact": "processed_count = succeeded_count + failed_count",
			"operation_batch_time_order":     "completed_at >= started_at",
		}),
	}
}
