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

// RatingSequenceSlot gives every ended rated contest one immutable position
// in the global rating history. A slot is never deleted to make a gap vanish;
// an authorized skip records an explicit terminal decision instead.
type RatingSequenceSlot struct{ ent.Schema }

func (RatingSequenceSlot) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (RatingSequenceSlot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).
			Ref("rating_sequence_slots").
			Field("contest_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("runs", ContestRatingRun.Type),
	}
}

func (RatingSequenceSlot) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("rating_sequence").Min(1).Immutable(),
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.Time("effective_end_at").Immutable(),
		field.Time("assigned_at").Immutable(),
	}
}

func (RatingSequenceSlot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("rating_sequence").Unique(),
		index.Fields("contest_id").Unique(),
		index.Fields("effective_end_at", "contest_id").Unique(),
	}
}

func (RatingSequenceSlot) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Check("contest_id <> '00000000-0000-0000-0000-000000000000'::uuid"),
	}
}
