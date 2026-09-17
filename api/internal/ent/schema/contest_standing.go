package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	e "github.com/huynhanx03/go-common/pkg/database/ent"
)

// ContestStanding holds the schema definition for the ContestStanding entity.
type ContestStanding struct {
	ent.Schema
}

// Mixin of the ContestStanding.
func (ContestStanding) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
	}
}

func (ContestStanding) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("contest_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Int("solved_count").Default(0),
		field.Int("penalty").Default(0).Comment("Total penalty in seconds"),
		field.JSON("problem_results", map[string]any{}).Optional().Comment("Per-problem results"),
		field.Int64("last_accepted_elapsed_ms").Min(0).Optional().Nillable(),
		field.Int64("version").Min(1).Default(1),
		field.Int("frozen_solved_count").NonNegative().Default(0),
		field.Int("frozen_penalty").NonNegative().Default(0),
		field.JSON("frozen_problem_results", map[string]any{}).Optional().
			Comment("Freeze-safe per-problem projection; never contains post-freeze verdicts"),
		field.JSON("frozen_pending_problem_attempts", map[string]int{}).Optional().
			Comment("Post-freeze intake counts by problem; contains no verdict information"),
		field.Int("frozen_pending_attempts").NonNegative().Default(0),
		field.Int64("frozen_last_accepted_elapsed_ms").Min(0).Optional().Nillable(),
		field.Int64("frozen_version").Min(1).Default(1),
	}
}

func (ContestStanding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "user_id").Unique(),
		index.Fields("contest_id", "solved_count", "penalty", "last_accepted_elapsed_ms", "user_id"),
		index.Fields("contest_id", "frozen_solved_count", "frozen_penalty", "user_id"),
	}
}

func (ContestStanding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).Ref("standings").Field("contest_id").Unique().Required(),
		edge.From("user", User.Type).Ref("contest_standings").Field("user_id").Unique().Required(),
	}
}
