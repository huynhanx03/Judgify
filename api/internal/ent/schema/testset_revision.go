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

// TestsetRevision stores one content-addressed immutable testset manifest.
type TestsetRevision struct{ ent.Schema }

func (TestsetRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (TestsetRevision) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.Int64("revision_number").
			Min(1).
			Max(problemcontract.MaximumTestsetRevisionNumber).
			Immutable(),
		field.Bytes("checksum").
			NotEmpty().
			Immutable(),
		field.Int("test_count").
			Min(1).
			Max(problemcontract.MaximumTestCases).
			Immutable(),
		field.Int64("total_uncompressed_bytes").
			Min(0).
			Max(problemcontract.MaximumTestsetUncompressedBytes).
			Immutable(),
		field.UUID("created_by", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
	)
}

func (TestsetRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("problem", Problem.Type).
			Ref("testset_revisions").
			Field("problem_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("groups", TestcaseGroup.Type).Immutable(),
		edge.To("testcase_revisions", TestcaseRevision.Type).Immutable(),
		edge.To("problem_revisions", ProblemRevision.Type).Immutable(),
		edge.To("contest_problem_snapshots", ContestProblem.Type).Immutable(),
	}
}

func (TestsetRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("problem_id", "revision_number").Unique(),
		index.Fields("id", "problem_id").Unique(),
		index.Fields("problem_id", "created_at", "id"),
		index.Fields("checksum"),
	}
}

func (TestsetRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"testset_revision_nonzero_identity": "id <> " + problemNilUUIDSQL +
				" AND problem_id <> " + problemNilUUIDSQL +
				" AND (created_by IS NULL OR created_by <> " + problemNilUUIDSQL + ")",
			"testset_revision_checksum_shape": "octet_length(checksum) = 32 AND checksum <> " + problemZeroDigestSQL,
			"testset_revision_number_bound": fmt.Sprintf(
				"revision_number BETWEEN 1 AND %d",
				problemcontract.MaximumTestsetRevisionNumber,
			),
			"testset_revision_count_bound": fmt.Sprintf(
				"test_count BETWEEN 1 AND %d",
				problemcontract.MaximumTestCases,
			),
			"testset_revision_total_size_bound": fmt.Sprintf(
				"total_uncompressed_bytes BETWEEN 0 AND %d",
				problemcontract.MaximumTestsetUncompressedBytes,
			),
		}),
	}
}
