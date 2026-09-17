package schema

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// ContestProblem freezes the exact Problem-owned judging identities selected
// for one contest. Mutable Problem pointers are never consulted for an
// official submission after this row is published.
type ContestProblem struct{ ent.Schema }

func (ContestProblem) Mixin() []ent.Mixin { return []ent.Mixin{commonent.AppendOnlyMixin{}} }

func (ContestProblem) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.UUID("problem_revision_id", uuid.UUID{}).Immutable(),
		field.UUID("testset_revision_id", uuid.UUID{}).Immutable(),
		field.Int("display_order").Positive().Immutable(),
		field.String("alias").NotEmpty().MaxLen(16).Immutable(),
		field.Int("points").Positive().Optional().Nillable().Immutable(),
		field.JSON("penalty_policy", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Immutable(),
		field.Bool("visible_before_start").Default(false).Immutable(),
		field.String("checker_key").NotEmpty().MaxLen(64).Immutable(),
		field.String("checker_version").NotEmpty().MaxLen(64).Immutable(),
		field.JSON("checker_config", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Immutable(),
		field.JSON("allowed_runtime_keys", []string{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Immutable(),
		field.Bytes("snapshot_checksum").NotEmpty().Immutable(),
	)
}

func (ContestProblem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "alias").Unique(),
		index.Fields("contest_id", "display_order").Unique(),
		index.Fields("contest_id", "problem_id").Unique(),
		index.Fields("id", "contest_id", "problem_id").Unique(),
		index.Fields("id", "contest_id").Unique(),
		index.Fields("problem_revision_id", "problem_id"),
		index.Fields("testset_revision_id", "problem_id"),
	}
}

func (ContestProblem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).Ref("contest_problems").Field("contest_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("problem", Problem.Type).Ref("contest_problem_snapshots").Field("problem_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("problem_revision", ProblemRevision.Type).Ref("contest_problem_snapshots").Field("problem_revision_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("testset_revision", TestsetRevision.Type).Ref("contest_problem_snapshots").Field("testset_revision_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("clarifications", ContestClarification.Type),
		edge.To("scoring_attempts", ContestScoringAttempt.Type),
		edge.To("scoring_results", ContestProblemResult.Type),
	}
}

func (ContestProblem) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"contest_problem_alias":             "alias ~ '^[A-Z][A-Z0-9]{0,15}$'",
			"contest_problem_snapshot_checksum": "octet_length(snapshot_checksum) = 32 AND snapshot_checksum <> decode(repeat('00', 32), 'hex')",
			"contest_problem_json_bounds": fmt.Sprintf(
				"jsonb_typeof(penalty_policy) = 'object' AND octet_length(penalty_policy::text) <= %d AND jsonb_typeof(checker_config) = 'object' AND octet_length(checker_config::text) <= %d AND jsonb_typeof(allowed_runtime_keys) = 'array' AND jsonb_array_length(allowed_runtime_keys) BETWEEN 1 AND %d",
				4096, 4096, 32,
			),
		}),
	}
}
