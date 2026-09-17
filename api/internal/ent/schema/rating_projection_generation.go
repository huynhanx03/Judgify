package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// RatingProjectionGeneration is one complete, immutable-addressed user rating
// projection. Rows may be built in shadow and become readable only after the
// singleton RatingState pointer changes atomically.
type RatingProjectionGeneration struct{ ent.Schema }

func (RatingProjectionGeneration) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (RatingProjectionGeneration) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("ratings", ContestUserRating.Type),
		edge.To("active_states", RatingState.Type),
	}
}

func (RatingProjectionGeneration) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("status").
			Values("building", "verified", "active", "retired").
			Default("building"),
		field.Int64("based_on_projection_revision").Min(0).Immutable(),
		field.Int64("through_sequence").Min(0).Default(0),
		field.Bytes("checksum").MaxLen(32).Optional().Nillable(),
		field.Time("verified_at").Optional().Nillable(),
		field.Time("activated_at").Optional().Nillable(),
	}
}

func (RatingProjectionGeneration) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "created_at", "id"),
		index.Fields("through_sequence", "id"),
	}
}

func (RatingProjectionGeneration) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"rating_projection_checksum": "checksum IS NULL OR octet_length(checksum) = 32",
			"rating_projection_verified": "(status = 'building' AND checksum IS NULL AND verified_at IS NULL AND activated_at IS NULL) OR (status = 'verified' AND checksum IS NOT NULL AND verified_at IS NOT NULL AND activated_at IS NULL) OR (status IN ('active', 'retired') AND checksum IS NOT NULL AND verified_at IS NOT NULL AND activated_at IS NOT NULL)",
		}),
	}
}
