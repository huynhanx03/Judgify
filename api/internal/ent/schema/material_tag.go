package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// MaterialRevisionTag is the normalized, ordered tag relation for one
// immutable material revision.
type MaterialRevisionTag struct{ ent.Schema }

func (MaterialRevisionTag) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (MaterialRevisionTag) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("revision_id", uuid.UUID{}).Immutable(),
		field.UUID("tag_id", uuid.UUID{}).Immutable(),
		field.Int("display_order").Min(0).Max(63).Immutable(),
	}
}

func (MaterialRevisionTag) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("revision", MaterialRevision.Type).
			Ref("tags").
			Field("revision_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("tag", Tag.Type).
			Ref("material_revision_tags").
			Field("tag_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (MaterialRevisionTag) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("revision_id", "tag_id").Unique(),
		index.Fields("revision_id", "display_order").Unique(),
		index.Fields("tag_id", "revision_id"),
	}
}
