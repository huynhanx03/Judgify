package schema

import (
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// UserRewardProfileTrait is one normalized row of a profile revision. The
// exact effect-revision foreign key prevents a profile from being reinterpreted
// through a later trait edit.
type UserRewardProfileTrait struct{ ent.Schema }

func (UserRewardProfileTrait) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (UserRewardProfileTrait) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("profile_revision_id", uuid.UUID{}).Immutable(),
		field.UUID("trait_id", uuid.UUID{}).Immutable(),
		field.UUID("trait_effect_revision_id", uuid.UUID{}).Immutable(),
		field.String("trait_code").NotEmpty().MaxLen(64).Immutable(),
		field.Enum("trait_type").Values("root_bone", "talent").Immutable(),
		field.String("trait_name").NotEmpty().MaxLen(255).Immutable(),
		field.String("trait_description").Optional().MaxLen(2048).Immutable(),
		field.String("rarity_code").NotEmpty().MaxLen(64).Immutable(),
		field.String("rarity_name").NotEmpty().MaxLen(255).Immutable(),
		field.Enum("slot").Values("root_bone", "talent").Immutable(),
		field.Int("slot_order").Min(0).Max(2).Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (UserRewardProfileTrait) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("profile_revision", UserRewardProfileRevision.Type).
			Ref("traits").Field("profile_revision_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("trait", Trait.Type).
			Ref("reward_profile_traits").Field("trait_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("trait_effect_revision", TraitEffectRevision.Type).
			Ref("profile_traits").Field("trait_effect_revision_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (UserRewardProfileTrait) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("profile_revision_id", "trait_id").Unique(),
		index.Fields("profile_revision_id", "slot", "slot_order").Unique(),
		index.Fields("profile_revision_id", "trait_effect_revision_id").Unique(),
		index.Fields("id", "profile_revision_id", "trait_id", "trait_effect_revision_id").Unique(),
	}
}

func (UserRewardProfileTrait) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"user_reward_profile_trait_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
			AND profile_revision_id <> ` + problemNilUUIDSQL + `
			AND trait_id <> ` + problemNilUUIDSQL + `
			AND trait_effect_revision_id <> ` + problemNilUUIDSQL,
		"user_reward_profile_trait_slot_shape": `(slot = 'root_bone' AND slot_order = 0)
			OR (slot = 'talent' AND slot_order BETWEEN 0 AND 2)`,
		"user_reward_profile_trait_type_shape": `trait_type = slot`,
	})}
}
