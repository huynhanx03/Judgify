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

// CultivationRankingProjection is the public leaderboard read model. It owns
// a random public identity and privacy policy, while source versions remain
// tied to the authoritative progression and Contest rating projections.
type CultivationRankingProjection struct{ ent.Schema }

func (CultivationRankingProjection) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("public_id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.Enum("visibility").Values("public", "anonymous", "hidden").Default("public"),
		field.Int64("privacy_version").Min(1).Default(1),
		field.Int64("projection_revision").Min(1),
		field.Int64("progression_source_version").Min(0).Default(0),
		field.Int64("rating_source_version").Min(0).Default(0),
		field.Int64("total_exp").NonNegative().Default(0),
		field.Int("rating").NonNegative().Default(1200),
		field.Time("progression_updated_at").Optional().Nillable(),
		field.Time("rating_updated_at").Optional().Nillable(),
		field.Time("created_at").Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
		field.Time("updated_at").Default(func() time.Time { return time.Now().UTC() }).
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (CultivationRankingProjection) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("cultivation_ranking_projection").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (CultivationRankingProjection) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
		index.Fields("public_id").Unique(),
		index.Fields("visibility", "rating", "rating_updated_at", "user_id"),
		index.Fields("visibility", "total_exp", "progression_updated_at", "user_id"),
		index.Fields("projection_revision"),
	}
}

func (CultivationRankingProjection) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"cultivation_ranking_projection_identity": `id <> ` + problemNilUUIDSQL + `
				AND user_id <> ` + problemNilUUIDSQL + `
				AND public_id <> ` + problemNilUUIDSQL,
		}),
	}
}
