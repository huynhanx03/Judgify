package schema

import (
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// CultivationRankingPrivacyChange is the immutable command receipt and
// provenance ledger for user-owned leaderboard privacy changes.
type CultivationRankingPrivacyChange struct{ ent.Schema }

func (CultivationRankingPrivacyChange) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (CultivationRankingPrivacyChange) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.Enum("from_visibility").Values("public", "anonymous", "hidden").Immutable(),
		field.Enum("to_visibility").Values("public", "anonymous", "hidden").Immutable(),
		field.Int64("from_version").Min(1).Immutable(),
		field.Int64("to_version").Min(2).Immutable(),
		field.Int64("projection_revision").Min(1).Immutable(),
		field.Bytes("request_checksum").Immutable(),
		field.String("reason").NotEmpty().MaxLen(512).Immutable(),
		field.String("correlation_id").NotEmpty().MaxLen(128).Immutable(),
		field.Time("occurred_at").Immutable(),
		field.Time("created_at").Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (CultivationRankingPrivacyChange) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "to_version").Unique(),
		index.Fields("user_id", "occurred_at", "id"),
		index.Fields("request_checksum"),
	}
}

func (CultivationRankingPrivacyChange) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"cultivation_ranking_privacy_identity": `id <> ` + problemNilUUIDSQL + `
				AND user_id <> ` + problemNilUUIDSQL,
			"cultivation_ranking_privacy_transition": `from_visibility <> to_visibility
				AND to_version = from_version + 1`,
			"cultivation_ranking_privacy_checksum": `octet_length(request_checksum) = 32
				AND request_checksum <> ` + problemZeroDigestSQL,
		}),
	}
}
