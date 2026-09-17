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

// UserRewardProfileRevision is an immutable, effective-dated assignment set.
// It is the sole user-trait input accepted by automatic reward calculation.
type UserRewardProfileRevision struct{ ent.Schema }

func (UserRewardProfileRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (UserRewardProfileRevision) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.Int64("revision").Min(1).Immutable(),
		field.Bytes("assignment_checksum").NotEmpty().Immutable(),
		field.Enum("source").Values("onboarding", "administrative").Immutable(),
		field.UUID("source_offer_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("command_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Bytes("request_checksum").NotEmpty().Immutable().Sensitive(),
		field.Time("effective_at").Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (UserRewardProfileRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("reward_profile_revisions").Field("user_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("source_offer", OnboardingTraitOffer.Type).
			Ref("reward_profile_revision").Field("source_offer_id").Unique().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("traits", UserRewardProfileTrait.Type),
		edge.To("ledger_entries", ProgressionLedgerEntry.Type),
	}
}

func (UserRewardProfileRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "revision").Unique(),
		index.Fields("user_id", "effective_at", "revision", "id").Unique(),
		index.Fields("source_offer_id").Unique().
			Annotations(entsql.IndexWhere("source_offer_id IS NOT NULL")),
		index.Fields("command_id").Unique().
			Annotations(entsql.IndexWhere("command_id IS NOT NULL")),
	}
}

func (UserRewardProfileRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"user_reward_profile_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
			AND user_id <> ` + problemNilUUIDSQL + `
			AND (source_offer_id IS NULL OR source_offer_id <> ` + problemNilUUIDSQL + `)
			AND (command_id IS NULL OR command_id <> ` + problemNilUUIDSQL + `)`,
		"user_reward_profile_checksum_shape": `octet_length(assignment_checksum) = 32
			AND assignment_checksum <> ` + problemZeroDigestSQL + `
			AND octet_length(request_checksum) = 32
			AND request_checksum <> ` + problemZeroDigestSQL,
		"user_reward_profile_source_shape": `(source = 'onboarding'
			AND source_offer_id IS NOT NULL AND command_id IS NULL)
			OR (source = 'administrative'
			AND source_offer_id IS NULL AND command_id IS NOT NULL)`,
	})}
}
