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

// TagRevision stores one immutable, server-owned tag snapshot.
type TagRevision struct{ ent.Schema }

func (TagRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (TagRevision) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("tag_id", uuid.UUID{}).Immutable(),
		field.Int64("revision").
			Min(1).
			Max(problemcontract.MaximumRewardMappingRevision).
			Immutable(),
		field.String("name").
			NotEmpty().
			MaxLen(50).
			Immutable(),
		field.Bytes("checksum").
			NotEmpty().
			Immutable(),
	)
}

func (TagRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tag", Tag.Type).
			Ref("revisions").
			Field("tag_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("elements", TagRevisionElement.Type).
			Immutable(),
	}
}

func (TagRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tag_id", "revision").Unique(),
		index.Fields("tag_id", "created_at", "id"),
	}
}

func (TagRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"tag_revision_nonzero_identity": "id <> " + problemNilUUIDSQL +
				" AND tag_id <> " + problemNilUUIDSQL,
			"tag_revision_checksum_shape": "octet_length(checksum) = 32 AND checksum <> " + problemZeroDigestSQL,
			"tag_revision_number_bound": fmt.Sprintf(
				"revision BETWEEN 1 AND %d",
				problemcontract.MaximumRewardMappingRevision,
			),
		}),
	}
}
