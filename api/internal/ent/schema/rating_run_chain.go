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

// RatingRunChain names the exact immutable run selected for every applied
// sequence. Shadow chains let historical rerating rebuild a suffix without
// changing canonical history until verification and pointer activation.
type RatingRunChain struct{ ent.Schema }

func (RatingRunChain) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (RatingRunChain) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("entries", RatingRunChainEntry.Type),
		edge.To("active_states", RatingState.Type),
	}
}

func (RatingRunChain) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("status").
			Values("building", "verified", "active", "retired").
			Default("building"),
		field.UUID("based_on_chain_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int64("through_sequence").Min(0).Default(0),
		field.Bytes("checksum").MaxLen(32).Optional().Nillable(),
		field.Time("verified_at").Optional().Nillable(),
		field.Time("activated_at").Optional().Nillable(),
	}
}

func (RatingRunChain) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "created_at", "id"),
		index.Fields("based_on_chain_id"),
		index.Fields("through_sequence", "id"),
	}
}

func (RatingRunChain) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"rating_chain_checksum": "checksum IS NULL OR octet_length(checksum) = 32",
			"rating_chain_verified": "(status = 'building' AND checksum IS NULL AND verified_at IS NULL AND activated_at IS NULL) OR (status = 'verified' AND checksum IS NOT NULL AND verified_at IS NOT NULL AND activated_at IS NULL) OR (status IN ('active', 'retired') AND checksum IS NOT NULL AND verified_at IS NOT NULL AND activated_at IS NOT NULL)",
		}),
	}
}
