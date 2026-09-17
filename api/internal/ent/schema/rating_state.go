package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

const defaultRatingStateID = "global"

// RatingState is the single-tenant serialization fence for every canonical
// rating decision. Public rating reads resolve both active pointers through
// this row, so a shadow rerating is never partially visible.
type RatingState struct{ ent.Schema }

func (RatingState) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.TimeMixin{}}
}

func (RatingState) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("active_projection_generation", RatingProjectionGeneration.Type).
			Ref("active_states").
			Field("active_projection_generation_id").
			Unique().
			Required(),
		edge.From("active_run_chain", RatingRunChain.Type).
			Ref("active_states").
			Field("active_run_chain_id").
			Unique().
			Required(),
	}
}

func (RatingState) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").Default(defaultRatingStateID).Immutable().MaxLen(32),
		field.Int64("next_sequence").Min(1).Default(1),
		field.Int64("applied_sequence").Min(0).Default(0),
		field.Int64("projection_revision").Min(1).Default(1),
		field.UUID("active_projection_generation_id", uuid.UUID{}),
		field.UUID("active_run_chain_id", uuid.UUID{}),
	}
}

func (RatingState) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"rating_state_singleton": "id = 'global'",
			"rating_state_sequence":  "next_sequence > applied_sequence",
			"rating_state_pointers":  "active_projection_generation_id <> '00000000-0000-0000-0000-000000000000'::uuid AND active_run_chain_id <> '00000000-0000-0000-0000-000000000000'::uuid",
		}),
	}
}
