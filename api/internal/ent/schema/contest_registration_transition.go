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

// ContestRegistrationTransition retains every logical command so an exact
// retry remains recoverable after later registration mutations.
type ContestRegistrationTransition struct{ ent.Schema }

func (ContestRegistrationTransition) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (ContestRegistrationTransition) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.UUID("registration_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("command_id", uuid.UUID{}).Immutable(),
		field.Bytes("command_hash").NotEmpty().Immutable(),
		field.Int64("registration_version").Min(1).Immutable(),
		field.Int64("official_submission_count").Min(0).Immutable(),
		field.Enum("mutation").Values("register", "withdraw", "ban").Immutable(),
		field.Enum("from_status").Values("absent", "registered", "withdrawn", "banned").Immutable(),
		field.Enum("to_status").Values("registered", "withdrawn", "banned").Immutable(),
		field.Enum("source").Values("explicit", "implicit_open", "admin").Immutable(),
		field.UUID("actor_user_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("actor_session_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("reason").MaxLen(1024).Optional().Nillable().Sensitive().Immutable(),
		field.Time("occurred_at").Immutable(),
	)
}

func (ContestRegistrationTransition) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "user_id", "command_id").Unique(),
		index.Fields("registration_id", "registration_version").Unique(),
		index.Fields("contest_id", "occurred_at", "id"),
	}
}

func (ContestRegistrationTransition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("registration", ContestRegistration.Type).
			Ref("transitions").Field("registration_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContestRegistrationTransition) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"contest_registration_transition_hash": "octet_length(command_hash) = 32 AND command_hash <> decode(repeat('00', 32), 'hex')",
			"contest_registration_transition_reason": fmt.Sprintf(
				"reason IS NULL OR octet_length(reason) BETWEEN 1 AND %d", 1024,
			),
			"contest_registration_transition_graph": `
				(mutation = 'register' AND from_status IN ('absent', 'withdrawn') AND to_status = 'registered') OR
				(mutation = 'withdraw' AND from_status = 'registered' AND to_status = 'withdrawn') OR
				(mutation = 'ban' AND from_status IN ('absent', 'registered', 'withdrawn') AND to_status = 'banned')`,
		}),
	}
}
