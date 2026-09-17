package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// OnboardingTraitOfferTalent is an immutable ordered member of one offer.
type OnboardingTraitOfferTalent struct{ ent.Schema }

func (OnboardingTraitOfferTalent) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (OnboardingTraitOfferTalent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("offer_id", uuid.UUID{}).Immutable(),
		field.UUID("trait_id", uuid.UUID{}).Immutable(),
		field.Int64("trait_revision").Min(1).Immutable(),
		field.Int("display_order").Min(0).Max(5).Immutable(),
	}
}

func (OnboardingTraitOfferTalent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("offer", OnboardingTraitOffer.Type).
			Ref("talents").Field("offer_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("trait", Trait.Type).
			Ref("onboarding_offer_talents").Field("trait_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (OnboardingTraitOfferTalent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("offer_id", "trait_id").Unique(),
		index.Fields("offer_id", "display_order").Unique(),
	}
}
