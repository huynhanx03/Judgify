package schema

import (
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// ContestTransition is immutable command history and exact replay evidence.
type ContestTransition struct{ ent.Schema }

func (ContestTransition) Mixin() []ent.Mixin { return []ent.Mixin{commonent.AppendOnlyMixin{}} }

func (ContestTransition) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.UUID("command_id", uuid.UUID{}).Immutable(),
		field.Bytes("command_hash").NotEmpty().Immutable(),
		field.Int64("version").Min(2).Immutable(),
		field.Enum("from_state").Values("draft", "upcoming", "running").Immutable(),
		field.Enum("to_state").Values("upcoming", "running", "ended", "cancelled").Immutable(),
		field.UUID("actor_user_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("actor_session_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("reason").NotEmpty().MaxLen(1024).Sensitive().Immutable(),
		field.Time("occurred_at").Immutable(),
	)
}

func (ContestTransition) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "version").Unique(),
		index.Fields("contest_id", "command_id").Unique(),
		index.Fields("contest_id", "occurred_at", "id"),
	}
}

func (ContestTransition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).Ref("transitions").Field("contest_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContestTransition) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"contest_transition_command_hash": "octet_length(command_hash) = 32 AND command_hash <> decode(repeat('00', 32), 'hex')",
			"contest_transition_reason_bound": fmt.Sprintf("octet_length(reason) BETWEEN 1 AND %d", 1024),
			"contest_transition_graph": `
				(from_state = 'draft' AND to_state IN ('upcoming', 'cancelled')) OR
				(from_state = 'upcoming' AND to_state IN ('running', 'cancelled')) OR
				(from_state = 'running' AND to_state IN ('ended', 'cancelled'))`,
		}),
	}
}
