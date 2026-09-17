package schema

import (
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// SolveRewardCursor serializes one canonical solve aggregate into the generic
// progression ledger. It is mutable coordination state; immutable accounting
// and replay evidence live in ledger entries and processed transitions.
type SolveRewardCursor struct{ ent.Schema }

func (SolveRewardCursor) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("solve_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.Int64("last_applied_version").Min(0).Default(0),
		field.Enum("current_status").Values("solved", "unsolved").Default("unsolved"),
		field.UUID("active_ledger_entry_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("last_transition_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("last_event_id", uuid.UUID{}).Optional().Nillable(),
		field.Bytes("last_transition_hash").Optional().Nillable(),
		field.Enum("reconciliation_state").Values("ready", "blocked_gap").Default("ready"),
		field.Int64("missing_from_version").Min(1).Optional().Nillable(),
		field.Int64("missing_to_version").Min(1).Optional().Nillable(),
		field.Time("created_at").Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
		field.Time("updated_at").Default(func() time.Time { return time.Now().UTC() }).
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (SolveRewardCursor) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("solve_id").Unique(),
		index.Fields("user_id", "problem_id").Unique(),
		index.Fields("reconciliation_state", "updated_at", "solve_id"),
		index.Fields("last_event_id"),
		index.Fields("active_ledger_entry_id"),
	}
}

func (SolveRewardCursor) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"solve_reward_cursor_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
				AND solve_id <> ` + problemNilUUIDSQL + `
				AND user_id <> ` + problemNilUUIDSQL + `
				AND problem_id <> ` + problemNilUUIDSQL,
			"solve_reward_cursor_active_entry": `(
				current_status = 'unsolved' AND active_ledger_entry_id IS NULL
			) OR (
				current_status = 'solved' AND active_ledger_entry_id IS NOT NULL
			)`,
			"solve_reward_cursor_last_identity": `(
				last_applied_version = 0
				AND last_transition_id IS NULL
				AND last_event_id IS NULL
				AND last_transition_hash IS NULL
			) OR (
				last_applied_version > 0
				AND last_transition_id IS NOT NULL
				AND last_event_id IS NOT NULL
				AND octet_length(last_transition_hash) = 32
				AND last_transition_hash <> ` + problemZeroDigestSQL + `
			)`,
			"solve_reward_cursor_gap": `(
				reconciliation_state = 'ready'
				AND missing_from_version IS NULL
				AND missing_to_version IS NULL
			) OR (
				reconciliation_state = 'blocked_gap'
				AND missing_from_version = last_applied_version + 1
				AND missing_to_version >= missing_from_version
			)`,
		}),
	}
}
