package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Trait holds the schema definition for the Trait entity (root bone / talent pool).
type Trait struct {
	ent.Schema
}

// Mixin of the Trait.
func (Trait) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Trait.
func (Trait) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").
			Unique().
			NotEmpty().
			MaxLen(64).
			Immutable(),
		field.Enum("type").
			Values("root_bone", "talent").
			Comment("ROOT_BONE = one offered slot; TALENT = weighted offer pool"),
		field.String("name").
			Unique().
			NotEmpty().
			MaxLen(100),
		field.UUID("rarity_id", uuid.UUID{}),
		field.String("description").
			Optional().
			MaxLen(500),
		field.Int("display_order").
			Default(0).
			NonNegative(),
		field.Bool("active").
			Default(true),
		field.Int64("version").
			Default(1).
			Min(1),
	}
}

// Edges of the Trait.
func (Trait) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("rarity", Rarity.Type).Ref("traits").Field("rarity_id").Unique().Required(),
		edge.To("user_traits", UserTrait.Type),
		edge.To("onboarding_offer_talents", OnboardingTraitOfferTalent.Type),
		edge.To("effect_revisions", TraitEffectRevision.Type),
		edge.To("reward_profile_traits", UserRewardProfileTrait.Type),
	}
}
