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
	e "github.com/huynhanx03/go-common/pkg/database/ent"
)

// ContestRegistration holds the schema definition for the ContestRegistration entity.
type ContestRegistration struct {
	ent.Schema
}

// Mixin of the ContestRegistration.
func (ContestRegistration) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
	}
}

func (ContestRegistration) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("contest_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Enum("status").Values("registered", "withdrawn", "banned").Default("registered"),
		field.Enum("source").Values("explicit", "implicit_open", "admin"),
		field.Int64("version").Min(1).Default(1),
		field.Int64("official_submission_count").Min(0).Default(0),
		field.Time("registered_at").Optional().Nillable(),
		field.Time("withdrawn_at").Optional().Nillable(),
		field.Time("banned_at").Optional().Nillable(),
		field.String("reason").MaxLen(1024).Optional().Nillable().Sensitive(),
	}
}

func (ContestRegistration) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "user_id").Unique(),
		index.Fields("id", "contest_id", "user_id").Unique(),
		index.Fields("user_id"),
		index.Fields("contest_id", "status", "id"),
	}
}

func (ContestRegistration) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"contest_registration_positive_version":              "version >= 1",
			"contest_registration_submission_count":              "official_submission_count >= 0",
			"contest_registration_withdrawal_has_no_submissions": "status <> 'withdrawn' OR official_submission_count = 0",
			"contest_registration_reason_bound": fmt.Sprintf(
				"reason IS NULL OR octet_length(reason) BETWEEN 1 AND %d",
				1024,
			),
			"contest_registration_state_complete": `(
				status = 'registered' AND registered_at IS NOT NULL AND withdrawn_at IS NULL AND banned_at IS NULL
			) OR (
				status = 'withdrawn' AND registered_at IS NOT NULL AND withdrawn_at IS NOT NULL AND banned_at IS NULL
			) OR (
				status = 'banned' AND banned_at IS NOT NULL
			)`,
		}),
	}
}

func (ContestRegistration) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).Ref("registrations").Field("contest_id").Unique().Required(),
		edge.From("user", User.Type).Ref("contest_registrations").Field("user_id").Unique().Required(),
		edge.To("transitions", ContestRegistrationTransition.Type),
	}
}
