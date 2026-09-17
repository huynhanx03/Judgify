package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// OnboardingTraitOffer is the mutable lifecycle shell around one immutable
// catalog selection. Only issued -> consumed/expired/superseded is allowed.
type OnboardingTraitOffer struct{ ent.Schema }

func (OnboardingTraitOffer) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("pre_session_id", uuid.UUID{}).Immutable(),
		field.UUID("idempotency_key", uuid.UUID{}).Immutable(),
		field.Int64("offer_sequence").Min(1).Immutable(),
		field.UUID("catalog_revision_id", uuid.UUID{}).Immutable(),
		field.String("algorithm_version").NotEmpty().MaxLen(64).Immutable(),
		field.UUID("root_bone_trait_id", uuid.UUID{}).Immutable(),
		field.Int64("root_bone_trait_revision").Min(1).Immutable(),
		field.Bytes("offer_checksum").NotEmpty().Immutable(),
		field.Enum("status").Values("issued", "consumed", "expired", "superseded").Default("issued"),
		field.Time("issued_at").Immutable(),
		field.Time("expires_at").Immutable(),
		field.UUID("consumed_by_user_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("consumed_at").Optional().Nillable(),
	}
}

func (OnboardingTraitOffer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("catalog_revision", CultivationCatalogRevision.Type).
			Ref("offers").Field("catalog_revision_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("consumed_by_user", User.Type).
			Ref("consumed_trait_offers").Field("consumed_by_user_id").Unique(),
		edge.To("talents", OnboardingTraitOfferTalent.Type),
		edge.To("reward_profile_revision", UserRewardProfileRevision.Type).Unique(),
		edge.To("issuance_receipts", AuthSessionIssuanceReceipt.Type),
	}
}

func (OnboardingTraitOffer) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("pre_session_id", "idempotency_key").Unique(),
		index.Fields("pre_session_id", "offer_sequence").Unique(),
		index.Fields("pre_session_id", "status", "offer_sequence"),
		index.Fields("consumed_by_user_id").Unique().Annotations(entsql.IndexWhere("consumed_by_user_id IS NOT NULL")),
	}
}

func (OnboardingTraitOffer) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"onboarding_trait_offer_nonzero_identity":  "id <> '00000000-0000-0000-0000-000000000000'::uuid AND pre_session_id <> '00000000-0000-0000-0000-000000000000'::uuid AND idempotency_key <> '00000000-0000-0000-0000-000000000000'::uuid AND root_bone_trait_id <> '00000000-0000-0000-0000-000000000000'::uuid",
		"onboarding_trait_offer_checksum_shape":    "octet_length(offer_checksum) = 32 AND offer_checksum <> decode(repeat('00', 32), 'hex')",
		"onboarding_trait_offer_time_shape":        "expires_at > issued_at",
		"onboarding_trait_offer_consumption_shape": "(status = 'consumed' AND consumed_by_user_id IS NOT NULL AND consumed_at IS NOT NULL) OR (status <> 'consumed' AND consumed_by_user_id IS NULL AND consumed_at IS NULL)",
	})}
}
