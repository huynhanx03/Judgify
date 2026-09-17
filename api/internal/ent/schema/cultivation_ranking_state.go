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

// CultivationRankingState is the one-row revision fence for public ranking
// cursors and bounded caches. Any committed projection/privacy change advances
// revision, so a later page can never silently mix two leaderboard snapshots.
type CultivationRankingState struct{ ent.Schema }

func (CultivationRankingState) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.String("key").NotEmpty().MaxLen(32).Immutable(),
		field.Int64("revision").Min(0).Default(0),
		field.Time("last_reconciled_at").Optional().Nillable(),
		field.Time("created_at").Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
		field.Time("updated_at").Default(func() time.Time { return time.Now().UTC() }).
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (CultivationRankingState) Indexes() []ent.Index {
	return []ent.Index{index.Fields("key").Unique()}
}

func (CultivationRankingState) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"cultivation_ranking_state_identity": `id <> ` + problemNilUUIDSQL,
			"cultivation_ranking_state_key":      `key = 'global'`,
		}),
	}
}
