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

// ElementRevision stores one immutable, server-owned element snapshot.
type ElementRevision struct{ ent.Schema }

func (ElementRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (ElementRevision) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("element_id", uuid.UUID{}).Immutable(),
		field.Int64("revision").
			Min(1).
			Max(problemcontract.MaximumRewardMappingRevision).
			Immutable(),
		field.String("name").
			NotEmpty().
			MaxLen(50).
			Immutable(),
		field.String("code").
			NotEmpty().
			MaxLen(20).
			Immutable(),
		field.String("description").
			Optional().
			MaxLen(255).
			Immutable(),
		field.Bytes("checksum").
			NotEmpty().
			Immutable(),
	)
}

func (ElementRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("element", Element.Type).
			Ref("revisions").
			Field("element_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ElementRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("element_id", "revision").Unique(),
		index.Fields("element_id", "created_at", "id"),
	}
}

func (ElementRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"element_revision_nonzero_identity": "id <> " + problemNilUUIDSQL +
				" AND element_id <> " + problemNilUUIDSQL,
			"element_revision_checksum_shape": "octet_length(checksum) = 32 AND checksum <> " + problemZeroDigestSQL,
			"element_revision_number_bound": fmt.Sprintf(
				"revision BETWEEN 1 AND %d",
				problemcontract.MaximumRewardMappingRevision,
			),
		}),
	}
}
