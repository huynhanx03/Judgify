package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Level holds the schema definition for cultivation levels (realms).
type Level struct {
	ent.Schema
}

// Mixin of the Level.
func (Level) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Level.
func (Level) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			MaxLen(100).
			Comment("Realm name: Luyen Khi, Truc Co, Kim Dan..."),
		field.Int64("min_exp").
			Default(0).
			NonNegative().
			Comment("Minimum EXP required to attempt breakthrough"),
		field.String("description").
			Optional().
			MaxLen(500),
	}
}

func (Level) Indexes() []ent.Index {
	return []ent.Index{index.Fields("name").Unique()}
}

// Edges of the Level.
func (Level) Edges() []ent.Edge {
	return []ent.Edge{}
}
