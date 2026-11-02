package schema

import (
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

const judgeWorkerNilUUIDSQL = "'00000000-0000-0000-0000-000000000000'::uuid"

// JudgeWorkerInstance is one durable boot identity. Every boot starts with
// claims disabled and uses heartbeat time, never UUID order, for freshness.
type JudgeWorkerInstance struct{ ent.Schema }

func (JudgeWorkerInstance) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (JudgeWorkerInstance) Fields() []ent.Field {
	return []ent.Field{
		field.String("worker_key").
			NotEmpty().
			MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.UUID("boot_id", uuid.UUID{}).Immutable(),
		field.Time("boot_started_at").Immutable(),
		field.Time("heartbeat_at"),
		field.Bool("claims_enabled").Default(false),
		field.Int("total_capacity").Min(1).Default(1),
		field.Int("free_capacity").Min(0).Default(0),
	}
}

func (JudgeWorkerInstance) Edges() []ent.Edge {
	return []ent.Edge{edge.To("capabilities", JudgeWorkerCapability.Type)}
}

func (JudgeWorkerInstance) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("boot_id").Unique(),
		index.Fields("worker_key", "boot_id").Unique(),
		index.Fields("claims_enabled", "heartbeat_at"),
		index.Fields("id", "boot_id").Unique(),
	}
}

func (JudgeWorkerInstance) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"judge_worker_instance_nonzero_identity": "id <> " + judgeWorkerNilUUIDSQL + " AND boot_id <> " + judgeWorkerNilUUIDSQL,
			"judge_worker_instance_key_shape":        "worker_key ~ '^[a-z0-9][a-z0-9._-]{0,127}$'",
			"judge_worker_instance_heartbeat_order":  "heartbeat_at >= boot_started_at",
			"judge_worker_instance_capacity_bound":   "total_capacity >= 1 AND free_capacity BETWEEN 0 AND total_capacity",
		}),
	}
}
