package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Trait holds the schema definition for the Trait entity (root bone / talent pool).
type Trait struct {
	ent.Schema
}

// Mixin of the Trait.
func (Trait) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Trait.
func (Trait) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("type").
			Values("root_bone", "talent").
			Comment("ROOT_BONE = 1 unique slot, TALENT = gacha pool"),
		field.String("name").
			Unique().
			NotEmpty().
			MaxLen(100),
		field.Enum("rarity").
			Values("mortal", "earth", "heaven", "divine").
			Default("mortal"),
		field.Int("weight").
			Default(100).
			Positive().
			Comment("Gacha weight, higher = more likely"),
		field.String("description").
			Optional().
			MaxLen(500),
		field.JSON("metadata", map[string]interface{}{}).
			Optional().
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
			}).
			Comment("Buff config: {type, value, target_scope, target_elements, daily_limit}"),
	}
}

// Edges of the Trait.
func (Trait) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user_traits", UserTrait.Type),
	}
}
