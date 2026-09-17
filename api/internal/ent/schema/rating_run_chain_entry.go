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

// RatingRunChainEntry selects one immutable run for one sequence in a chain.
type RatingRunChainEntry struct{ ent.Schema }

func (RatingRunChainEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (RatingRunChainEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("chain", RatingRunChain.Type).
			Ref("entries").
			Field("chain_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("rating_run", ContestRatingRun.Type).
			Ref("chain_entries").
			Field("rating_run_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (RatingRunChainEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("chain_id", uuid.UUID{}).Immutable(),
		field.Int64("rating_sequence").Min(1).Immutable(),
		field.UUID("rating_run_id", uuid.UUID{}).Immutable(),
	}
}

func (RatingRunChainEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("chain_id", "rating_sequence").Unique(),
		index.Fields("chain_id", "rating_run_id").Unique(),
		index.Fields("rating_run_id"),
	}
}

func (RatingRunChainEntry) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Check("chain_id <> '00000000-0000-0000-0000-000000000000'::uuid AND rating_run_id <> '00000000-0000-0000-0000-000000000000'::uuid"),
	}
}
