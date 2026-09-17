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

// TestcaseRevision is the authoritative immutable test snapshot. TestCase
// provides stable identity only; no worker or public mapper reads legacy test
// payload columns after cutover.
type TestcaseRevision struct{ ent.Schema }

func (TestcaseRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (TestcaseRevision) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("test_case_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.UUID("testset_revision_id", uuid.UUID{}).Immutable(),
		field.UUID("group_id", uuid.UUID{}).Immutable(),
		field.Enum("kind").
			Values(problemcontract.TestKinds()...).
			Immutable(),
		field.Int("group_index").
			Min(0).
			Max(problemcontract.MaximumTestcaseGroups-1).
			Immutable(),
		field.Int("order_index").
			Min(0).
			Max(problemcontract.MaximumTestCases-1).
			Immutable(),
		field.UUID("input_asset_id", uuid.UUID{}).Immutable(),
		field.UUID("expected_asset_id", uuid.UUID{}).Immutable(),
		field.Int("weight").
			Default(1).
			Immutable(),
		field.Bool("stop_on_failure").
			Default(false).
			Immutable(),
	)
}

func (TestcaseRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("test_case", TestCase.Type).
			Ref("revisions").
			Field("test_case_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("testset_revision", TestsetRevision.Type).
			Ref("testcase_revisions").
			Field("testset_revision_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("group", TestcaseGroup.Type).
			Ref("testcase_revisions").
			Field("group_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("input_asset", TestAsset.Type).
			Ref("input_testcase_revisions").
			Field("input_asset_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("expected_asset", TestAsset.Type).
			Ref("expected_testcase_revisions").
			Field("expected_asset_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (TestcaseRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("testset_revision_id", "test_case_id").Unique(),
		index.Fields("test_case_id", "problem_id"),
		index.Fields("testset_revision_id", "problem_id"),
		index.Fields("testset_revision_id", "group_index", "order_index").Unique(),
		index.Fields("group_id", "order_index").Unique(),
		index.Fields("group_id", "testset_revision_id", "group_index", "stop_on_failure"),
		index.Fields("testset_revision_id", "kind", "group_index", "order_index"),
		index.Fields("input_asset_id"),
		index.Fields("expected_asset_id"),
	}
}

func (TestcaseRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"testcase_revision_nonzero_identity": "id <> " + problemNilUUIDSQL +
				" AND test_case_id <> " + problemNilUUIDSQL +
				" AND problem_id <> " + problemNilUUIDSQL +
				" AND testset_revision_id <> " + problemNilUUIDSQL +
				" AND group_id <> " + problemNilUUIDSQL +
				" AND input_asset_id <> " + problemNilUUIDSQL +
				" AND expected_asset_id <> " + problemNilUUIDSQL,
			"testcase_revision_kind_closed": contentRenderClosedDomainCheck("kind", problemcontract.TestKinds()),
			"testcase_revision_group_index_bound": fmt.Sprintf(
				"group_index BETWEEN 0 AND %d",
				problemcontract.MaximumTestcaseGroups-1,
			),
			"testcase_revision_order_index_bound": fmt.Sprintf(
				"order_index BETWEEN 0 AND %d",
				problemcontract.MaximumTestCases-1,
			),
			"testcase_revision_launch_weight": "weight = 1",
		}),
	}
}
