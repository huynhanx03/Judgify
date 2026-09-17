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

// TestcaseGroup stores immutable group execution behavior for a testset.
type TestcaseGroup struct{ ent.Schema }

func (TestcaseGroup) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (TestcaseGroup) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("testset_revision_id", uuid.UUID{}).Immutable(),
		field.Int("group_index").
			Min(0).
			Max(problemcontract.MaximumTestcaseGroups-1).
			Immutable(),
		field.Bool("stop_on_failure").
			Default(false).
			Immutable(),
	)
}

func (TestcaseGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("testset_revision", TestsetRevision.Type).
			Ref("groups").
			Field("testset_revision_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("testcase_revisions", TestcaseRevision.Type).Immutable(),
	}
}

func (TestcaseGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("testset_revision_id", "group_index").Unique(),
		index.Fields("testset_revision_id", "group_index", "stop_on_failure").Unique(),
		index.Fields("id", "testset_revision_id", "group_index", "stop_on_failure").Unique(),
		index.Fields("testset_revision_id", "created_at", "id"),
	}
}

func (TestcaseGroup) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"testcase_group_nonzero_identity": "id <> " + problemNilUUIDSQL +
				" AND testset_revision_id <> " + problemNilUUIDSQL,
			"testcase_group_index_bound": fmt.Sprintf(
				"group_index BETWEEN 0 AND %d",
				problemcontract.MaximumTestcaseGroups-1,
			),
		}),
	}
}
