package schema

import (
	"fmt"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

// Difficulty holds the schema definition for the Difficulty entity.
type Difficulty struct {
	ent.Schema
}

// Mixin of the Difficulty.
func (Difficulty) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Difficulty.
func (Difficulty) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Unique().
			NotEmpty().
			MaxLen(50),
		field.Int("level").
			Unique().
			Positive(),
		field.Int64("exp_reward").
			Default(0).
			NonNegative().
			Max(problemcontract.MaximumBaseExperience),
		field.String("description").
			Optional().
			MaxLen(255),
		field.Int64("version").
			Min(1).
			Default(1),
	}
}

// Edges of the Difficulty.
func (Difficulty) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("revisions", DifficultyRevision.Type),
		edge.To("problems", Problem.Type),
		edge.To("user_difficulty_stats", UserDifficultyStats.Type),
		edge.To("materials", Material.Type),
		edge.To("material_revisions", MaterialRevision.Type),
	}
}

func (Difficulty) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"difficulty_positive_version": "version >= 1",
			"difficulty_exp_reward_bound": fmt.Sprintf(
				"exp_reward BETWEEN 0 AND %d",
				problemcontract.MaximumBaseExperience,
			),
		}),
	}
}
