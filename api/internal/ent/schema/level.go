package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Level holds the schema definition for cultivation levels (realms).
type Level struct {
	ent.Schema
}

// Mixin of the Level.
func (Level) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Level.
func (Level) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Unique().
			NotEmpty().
			MaxLen(100).
			Comment("Realm name: Luyen Khi, Truc Co, Kim Dan..."),
		field.Int("order").
			Unique().
			NonNegative().
			Comment("Progression order, lower = earlier realm"),
		field.Int64("min_exp").
			Default(0).
			NonNegative().
			Comment("Minimum EXP required to attempt breakthrough"),
		field.String("description").
			Optional().
			MaxLen(500),
	}
}

// Edges of the Level.
func (Level) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user_stats", UserStats.Type),
	}
}
