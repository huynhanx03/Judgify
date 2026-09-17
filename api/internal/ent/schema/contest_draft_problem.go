package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// ContestDraftProblem is the mutable authoring selection for one contest
// problem. Publication copies these bounded presentation fields together with
// the Problem-owned immutable judge identities into ContestProblem.
type ContestDraftProblem struct{ ent.Schema }

func (ContestDraftProblem) Mixin() []ent.Mixin {
	return []ent.Mixin{
		commonent.UUIDMixin{},
		commonent.TimeMixin{},
	}
}

func (ContestDraftProblem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_id", uuid.UUID{}),
		field.Int("display_order").Positive(),
		field.String("alias").NotEmpty().MaxLen(16),
		field.Int("points").Positive().Optional().Nillable(),
		field.Bool("visible_before_start").Default(true),
	}
}

func (ContestDraftProblem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "problem_id").Unique(),
		index.Fields("contest_id", "display_order").Unique(),
		index.Fields("contest_id", "alias").Unique(),
		index.Fields("problem_id"),
	}
}

func (ContestDraftProblem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).
			Ref("draft_problems").
			Field("contest_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.From("problem", Problem.Type).
			Ref("contest_draft_memberships").
			Field("problem_id").
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContestDraftProblem) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		&entsql.Annotation{
			Table: "contest_draft_problems",
			Checks: map[string]string{
				"contest_draft_problem_alias":  "alias ~ '^[A-Z][A-Z0-9]{0,15}$'",
				"contest_draft_problem_points": "points IS NULL OR points > 0",
			},
		},
	}
}
