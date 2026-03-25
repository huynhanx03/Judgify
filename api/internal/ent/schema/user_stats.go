package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// UserStats holds the schema definition for user cultivation state.
type UserStats struct {
	ent.Schema
}

// Mixin of the UserStats.
func (UserStats) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the UserStats.
func (UserStats) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").
			Unique(),
		field.Int64("total_exp").
			Default(0).
			NonNegative(),
		field.Int("current_level_id"),
		field.Int("rating").
			Default(0),
	}
}

// Edges of the UserStats.
func (UserStats) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("user_stats").Field("user_id").Unique().Required(),
		edge.From("current_level", Level.Type).Ref("user_stats").Field("current_level_id").Unique().Required(),
	}
}

// Indexes of the UserStats.
func (UserStats) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
	}
}
