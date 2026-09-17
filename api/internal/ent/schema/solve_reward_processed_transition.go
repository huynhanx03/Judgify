package schema

import (
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

// SolveRewardProcessedTransition is the immutable inbox/evidence record for a
// cursor version. It distinguishes a true duplicate from a stale delivery that
// reuses a version with conflicting identity or provenance.
type SolveRewardProcessedTransition struct{ ent.Schema }

func (SolveRewardProcessedTransition) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (SolveRewardProcessedTransition) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("solve_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.Int64("aggregate_version").Min(1).Immutable(),
		field.UUID("transition_id", uuid.UUID{}).Immutable(),
		field.UUID("event_id", uuid.UUID{}).Immutable(),
		field.Bytes("transition_hash").Immutable(),
		field.Enum("outcome").Values("grant", "reversal", "pointer_move").Immutable(),
		field.UUID("ledger_entry_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("correlation_id").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.Time("occurred_at").Immutable(),
		field.Time("processed_at").Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (SolveRewardProcessedTransition) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("solve_id", "aggregate_version").Unique(),
		index.Fields("transition_id").Unique(),
		index.Fields("event_id").Unique(),
		index.Fields("ledger_entry_id"),
		index.Fields("user_id", "processed_at", "id"),
	}
}

func (SolveRewardProcessedTransition) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"solve_reward_processed_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
				AND solve_id <> ` + problemNilUUIDSQL + `
				AND user_id <> ` + problemNilUUIDSQL + `
				AND problem_id <> ` + problemNilUUIDSQL + `
				AND transition_id <> ` + problemNilUUIDSQL + `
				AND event_id <> ` + problemNilUUIDSQL,
			"solve_reward_processed_hash": `octet_length(transition_hash) = 32
				AND transition_hash <> ` + problemZeroDigestSQL,
			"solve_reward_processed_outcome": `(
				outcome IN ('grant', 'reversal') AND ledger_entry_id IS NOT NULL
			) OR (
				outcome = 'pointer_move' AND ledger_entry_id IS NULL
			)`,
		}),
	}
}
