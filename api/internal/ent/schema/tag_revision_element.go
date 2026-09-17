package schema

import (
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

// TagRevisionElement pins one tag revision to the exact element revision that
// contributed to that immutable taxonomy snapshot.
type TagRevisionElement struct{ ent.Schema }

func (TagRevisionElement) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (TagRevisionElement) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("tag_revision_id", uuid.UUID{}).Immutable(),
		field.UUID("element_id", uuid.UUID{}).Immutable(),
		field.Int64("element_revision").
			Min(1).
			Max(problemcontract.MaximumRewardMappingRevision).
			Immutable(),
	)
}

func (TagRevisionElement) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tag_revision", TagRevision.Type).
			Ref("elements").
			Field("tag_revision_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("element", Element.Type).
			Ref("tag_revision_elements").
			Field("element_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (TagRevisionElement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tag_revision_id", "element_id").Unique(),
		index.Fields("element_id", "element_revision"),
	}
}

func (TagRevisionElement) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"tag_revision_element_nonzero_identity": "id <> " + problemNilUUIDSQL +
				" AND tag_revision_id <> " + problemNilUUIDSQL +
				" AND element_id <> " + problemNilUUIDSQL,
			"tag_revision_element_revision_bound": fmt.Sprintf(
				"element_revision BETWEEN 1 AND %d",
				problemcontract.MaximumRewardMappingRevision,
			),
		}),
	}
}
