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

// ContestUserRating is the canonical current-rating projection inside one
// generation. A missing row means the virtual new-user state (1200, zero
// rated contests); it never means legacy rating zero.
type ContestUserRating struct{ ent.Schema }

func (ContestUserRating) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (ContestUserRating) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("projection_generation", RatingProjectionGeneration.Type).
			Ref("ratings").
			Field("projection_generation_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("user", User.Type).
			Ref("contest_user_ratings").
			Field("user_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContestUserRating) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("projection_generation_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.Int("rating").NonNegative(),
		field.Int("rated_contest_count").NonNegative(),
		field.Int64("applied_sequence").Min(1),
		field.Int64("revision").Min(1),
	}
}

func (ContestUserRating) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("projection_generation_id", "user_id").Unique(),
		index.Fields("projection_generation_id", "rating", "user_id"),
		index.Fields("user_id", "applied_sequence"),
	}
}

func (ContestUserRating) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Check("projection_generation_id <> '00000000-0000-0000-0000-000000000000'::uuid AND user_id <> '00000000-0000-0000-0000-000000000000'::uuid"),
	}
}
