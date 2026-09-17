package schema

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

var judgeTestVerdicts = []string{
	"accepted", "wrong_answer", "runtime_error", "time_limit_exceeded",
	"memory_limit_exceeded", "process_limit_exceeded", "output_limit_exceeded",
}

// JudgeTestResult stores bounded metadata for one testcase on the canonical
// successful execution attempt. It never stores hidden input or expected data.
type JudgeTestResult struct{ ent.Schema }

func (JudgeTestResult) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (JudgeTestResult) Fields() []ent.Field {
	return append(submissionImmutableRecordFields(),
		field.UUID("attempt_id", uuid.UUID{}).Immutable(),
		field.UUID("judge_spec_id", uuid.UUID{}).Immutable(),
		field.UUID("testset_revision_id", uuid.UUID{}).Immutable(),
		field.UUID("group_id", uuid.UUID{}).Immutable(),
		field.Int("group_index").Min(0).Max(problemcontract.MaximumTestcaseGroups-1).Immutable(),
		field.Bool("group_stop_on_failure").Immutable(),
		field.UUID("testcase_revision_id", uuid.UUID{}).Immutable(),
		// case_order is immutable order inside one testcase group and is part
		// of the exact testcase-revision identity. execution_order is the
		// attempt-global sequence used for deterministic result playback.
		field.Int("case_order").Min(0).Max(problemcontract.MaximumTestCases-1).Immutable(),
		field.Int("execution_order").Min(0).Max(problemcontract.MaximumTestCases-1).Immutable(),
		field.Enum("test_kind").Values(problemcontract.TestKinds()...).Immutable(),
		field.Enum("verdict").Values(judgeTestVerdicts...).Immutable(),
		field.Int("time_ms").Min(0).Max(problemcontract.MaximumWallTimeLimitMilliseconds).Immutable(),
		field.Int("memory_kb").Min(0).Max(problemcontract.MaximumMemoryLimitKilobytes).Immutable(),
		field.Int("exit_code").Optional().Nillable().Immutable(),
		field.Int("terminating_signal").Min(1).Optional().Nillable().Immutable(),
		field.Int("stdout_bytes").Min(0).Max(problemcontract.MaximumOutputLimitBytes).Immutable(),
		field.Int("stderr_bytes").Min(0).Max(problemcontract.MaximumOutputLimitBytes).Immutable(),
		field.String("safe_diagnostic").MaxLen(maximumJudgeDiagnosticBytes).
			Optional().Nillable().Sensitive().Immutable(),
		field.JSON("checker_metadata", json.RawMessage{}).
			SchemaType(submissionJSONSchemaType()).Immutable(),
		field.Time("execution_started_at").Immutable(),
		field.Time("execution_completed_at").Immutable(),
	)
}

func (JudgeTestResult) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("attempt_id", "testcase_revision_id").Unique(),
		index.Fields("attempt_id", "execution_order").Unique(),
		index.Fields("attempt_id", "group_index", "case_order").Unique(),
		index.Fields("judge_spec_id", "testset_revision_id", "group_index", "case_order"),
		index.Fields("testcase_revision_id"),
	}
}

func (JudgeTestResult) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"judge_test_result_verdict_closed": `verdict IN ('accepted', 'wrong_answer', 'runtime_error', 'time_limit_exceeded', 'memory_limit_exceeded', 'process_limit_exceeded', 'output_limit_exceeded')`,
			"judge_test_result_kind_closed":    "test_kind IN ('sample', 'hidden')",
			"judge_test_result_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
				AND attempt_id <> ` + problemNilUUIDSQL + `
				AND judge_spec_id <> ` + problemNilUUIDSQL + `
				AND testset_revision_id <> ` + problemNilUUIDSQL + `
				AND group_id <> ` + problemNilUUIDSQL + `
				AND testcase_revision_id <> ` + problemNilUUIDSQL,
			"judge_test_result_index_bounds": fmt.Sprintf(
				"group_index BETWEEN 0 AND %d AND case_order BETWEEN 0 AND %d AND execution_order BETWEEN 0 AND %d",
				problemcontract.MaximumTestcaseGroups-1,
				problemcontract.MaximumTestCases-1,
				problemcontract.MaximumTestCases-1,
			),
			"judge_test_result_resource_bounds": fmt.Sprintf(
				`time_ms BETWEEN 0 AND %d
				AND memory_kb BETWEEN 0 AND %d
				AND stdout_bytes BETWEEN 0 AND %d
				AND stderr_bytes BETWEEN 0 AND %d`,
				problemcontract.MaximumWallTimeLimitMilliseconds,
				problemcontract.MaximumMemoryLimitKilobytes,
				problemcontract.MaximumOutputLimitBytes,
				problemcontract.MaximumOutputLimitBytes,
			),
			"judge_test_result_exit_complete": `exit_code IS NULL OR terminating_signal IS NULL`,
			"judge_test_result_safe_diagnostic": fmt.Sprintf(
				`safe_diagnostic IS NULL OR (
					test_kind = 'sample'
					AND octet_length(safe_diagnostic) BETWEEN 1 AND %d
				)`,
				maximumJudgeDiagnosticBytes,
			),
			"judge_test_result_checker_metadata_bound": fmt.Sprintf(
				"jsonb_typeof(checker_metadata) = 'object' AND octet_length(checker_metadata::text) <= %d",
				problemcontract.MaximumCheckerConfigBytes,
			),
			"judge_test_result_execution_order": "execution_completed_at >= execution_started_at",
		}),
	}
}
