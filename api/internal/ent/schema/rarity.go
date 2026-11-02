package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Rarity holds the schema definition for the Rarity entity.
type Rarity struct {
	ent.Schema
}

// Mixin of the Rarity.
func (Rarity) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Rarity.
func (Rarity) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			MaxLen(50),
		field.String("code").
			NotEmpty().
			MaxLen(20).
			Comment("Unique code: mortal, earth, heaven, divine"),
		field.Int("weight").
			Default(100).
			Positive().
			Comment("Trait-offer weight; higher values increase draw probability"),
		field.String("description").
			Optional().
			MaxLen(255),
	}
}

func (Rarity) Indexes() []ent.Index {
	return []ent.Index{index.Fields("name").Unique(), index.Fields("code").Unique()}
}

// Edges of the Rarity.
func (Rarity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("traits", Trait.Type),
	}
}
