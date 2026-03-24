package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Rarity holds the schema definition for the Rarity entity.
type Rarity struct {
	ent.Schema
}

// Mixin of the Rarity.
func (Rarity) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Rarity.
func (Rarity) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Unique().
			NotEmpty().
			MaxLen(50),
		field.String("code").
			Unique().
			NotEmpty().
			MaxLen(20).
			Comment("Unique code: mortal, earth, heaven, divine"),
		field.Int("weight").
			Default(100).
			Positive().
			Comment("Gacha weight, higher = more likely"),
		field.String("description").
			Optional().
			MaxLen(255),
	}
}

// Edges of the Rarity.
func (Rarity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("traits", Trait.Type),
	}
}
