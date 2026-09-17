package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// UserStats holds the schema definition for user cultivation state.
type UserStats struct {
	ent.Schema
}

// Mixin of the UserStats.
func (UserStats) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the UserStats.
func (UserStats) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).
			Unique(),
		field.Int64("total_exp").
			Default(0).
			NonNegative(),
		field.Int("rating").
			Default(1200).
			NonNegative(),
		field.Int("total_submissions").
			Default(0).
			NonNegative(),
		field.Int("accepted_count").
			Default(0).
			NonNegative(),
		field.Int64("progression_version").
			Default(0).
			NonNegative(),
		field.Int64("rating_version").
			Default(0).
			NonNegative(),
		field.Time("progression_updated_at").
			Optional().
			Nillable(),
		field.Time("rating_updated_at").
			Optional().
			Nillable(),
	}
}

// Edges of the UserStats.
func (UserStats) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("user_stats").Field("user_id").Unique().Required(),
	}
}

// Indexes of the UserStats.
func (UserStats) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
	}
}
