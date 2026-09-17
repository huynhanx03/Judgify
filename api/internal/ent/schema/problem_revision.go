package schema

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

const (
	problemNilUUIDSQL    = "'00000000-0000-0000-0000-000000000000'::uuid"
	problemZeroDigestSQL = "decode(repeat('00', 32), 'hex')"
)

// ProblemRevision stores one immutable authored and executable problem
// snapshot. Saving an edit creates another row; publication only switches the
// stable Problem aggregate's selection pointers.
type ProblemRevision struct{ ent.Schema }

func (ProblemRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (ProblemRevision) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
		field.Int64("revision_number").
			Min(1).
			Max(problemcontract.MaximumProblemRevisionNumber).
			Immutable(),
		field.String("title").
			NotEmpty().
			MaxLen(problemcontract.MaximumTitleBytes).
			Immutable(),
		field.Text("statement_markdown").
			NotEmpty().
			MaxLen(problemcontract.MaximumStatementMarkdownBytes).
			Immutable().
			Sensitive(),
		field.UUID("render_source_id", uuid.UUID{}).Immutable(),
		field.Enum("render_source_kind").
			Values(problemcontract.RenderSourceKindProblemStatement).
			Immutable(),
		field.Bytes("render_source_checksum").
			NotEmpty().
			Immutable(),
		field.UUID("difficulty_id", uuid.UUID{}).Immutable(),
		field.Int64("difficulty_revision").
			Min(1).
			Max(problemcontract.MaximumDifficultyRevision).
			Immutable(),
		field.Int64("base_exp").
			Min(0).
			Max(problemcontract.MaximumBaseExperience).
			Immutable(),
		field.UUID("tag_mapping_id", uuid.UUID{}).Immutable(),
		field.Int64("tag_mapping_revision").
			Min(1).
			Max(problemcontract.MaximumRewardMappingRevision).
			Immutable(),
		field.JSON("tag_snapshot", json.RawMessage{}).
			SchemaType(problemJSONSchemaType()).
			Immutable(),
		field.UUID("element_mapping_id", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
		field.Int64("element_mapping_revision").
			Min(1).
			Max(problemcontract.MaximumRewardMappingRevision).
			Optional().
			Nillable().
			Immutable(),
		field.JSON("element_snapshot", json.RawMessage{}).
			SchemaType(problemJSONSchemaType()).
			Optional().
			Immutable(),
		field.Bytes("reward_snapshot_checksum").
			NotEmpty().
			Immutable(),
		field.Int("cpu_time_limit_ms").
			Min(problemcontract.MinimumCPUTimeLimitMilliseconds).
			Max(problemcontract.MaximumCPUTimeLimitMilliseconds).
			Immutable(),
		field.Int("wall_time_limit_ms").
			Min(problemcontract.MinimumCPUTimeLimitMilliseconds).
			Max(problemcontract.MaximumWallTimeLimitMilliseconds).
			Immutable(),
		field.Int("memory_limit_kb").
			Min(problemcontract.MinimumMemoryLimitKilobytes).
			Max(problemcontract.MaximumMemoryLimitKilobytes).
			Immutable(),
		field.Int("output_limit_bytes").
			Min(problemcontract.MinimumOutputLimitBytes).
			Max(problemcontract.MaximumOutputLimitBytes).
			Immutable(),
		field.Int("process_limit").
			Min(problemcontract.MinimumProcessLimit).
			Max(problemcontract.MaximumProcessLimit).
			Immutable(),
		field.String("checker_key").
			NotEmpty().
			MaxLen(problemcontract.MaximumCheckerKeyBytes).
			Immutable(),
		field.String("checker_version").
			NotEmpty().
			MaxLen(problemcontract.MaximumCheckerVersionBytes).
			Immutable(),
		field.JSON("checker_config", json.RawMessage{}).
			SchemaType(problemJSONSchemaType()).
			Immutable(),
		field.UUID("testset_revision_id", uuid.UUID{}).Immutable(),
		field.JSON("allowed_runtime_keys", []string{}).
			SchemaType(problemJSONSchemaType()).
			Immutable(),
		field.UUID("created_by", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
		field.Bytes("content_checksum").
			NotEmpty().
			Immutable(),
	)
}

func (ProblemRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("problem", Problem.Type).
			Ref("revisions").
			Field("problem_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("testset_revision", TestsetRevision.Type).
			Ref("problem_revisions").
			Field("testset_revision_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("contest_problem_snapshots", ContestProblem.Type).Immutable(),
	}
}

func (ProblemRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("problem_id", "revision_number").Unique(),
		index.Fields("id", "problem_id").Unique(),
		index.Fields("id", "render_source_id", "render_source_kind", "render_source_checksum").Unique(),
		index.Fields("render_source_id", "render_source_kind").Unique(),
		index.Fields("problem_id", "created_at", "id"),
		index.Fields("testset_revision_id"),
		index.Fields("content_checksum"),
	}
}

func (ProblemRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"problem_revision_nonzero_identity": `id <> ` + problemNilUUIDSQL +
				` AND problem_id <> ` + problemNilUUIDSQL +
				` AND render_source_id <> ` + problemNilUUIDSQL +
				` AND difficulty_id <> ` + problemNilUUIDSQL +
				` AND tag_mapping_id <> ` + problemNilUUIDSQL +
				` AND testset_revision_id <> ` + problemNilUUIDSQL +
				` AND (created_by IS NULL OR created_by <> ` + problemNilUUIDSQL + `)`,
			"problem_revision_title_bound": fmt.Sprintf(
				"octet_length(title) BETWEEN 1 AND %d",
				problemcontract.MaximumTitleBytes,
			),
			"problem_revision_statement_bound": fmt.Sprintf(
				"octet_length(statement_markdown) BETWEEN 1 AND %d",
				problemcontract.MaximumStatementMarkdownBytes,
			),
			"problem_revision_number_bound": fmt.Sprintf(
				"revision_number BETWEEN 1 AND %d",
				problemcontract.MaximumProblemRevisionNumber,
			),
			"problem_revision_difficulty_bound": fmt.Sprintf(
				"difficulty_revision BETWEEN 1 AND %d",
				problemcontract.MaximumDifficultyRevision,
			),
			"problem_revision_reward_mapping_bound": fmt.Sprintf(
				`tag_mapping_revision BETWEEN 1 AND %d
				AND (element_mapping_revision IS NULL OR element_mapping_revision BETWEEN 1 AND %d)
				AND base_exp BETWEEN 0 AND %d`,
				problemcontract.MaximumRewardMappingRevision,
				problemcontract.MaximumRewardMappingRevision,
				problemcontract.MaximumBaseExperience,
			),
			"problem_revision_source_kind": "render_source_kind = '" +
				problemcontract.RenderSourceKindProblemStatement + "'",
			"problem_revision_checksum_shape": `octet_length(render_source_checksum) = 32
				AND octet_length(reward_snapshot_checksum) = 32
				AND octet_length(content_checksum) = 32`,
			"problem_revision_nonzero_checksums": `render_source_checksum <> ` + problemZeroDigestSQL +
				` AND reward_snapshot_checksum <> ` + problemZeroDigestSQL +
				` AND content_checksum <> ` + problemZeroDigestSQL,
			"problem_revision_element_snapshot_complete": `(
				element_mapping_id IS NULL AND element_mapping_revision IS NULL AND element_snapshot IS NULL
			) OR (
				element_mapping_id IS NOT NULL AND element_mapping_id <> ` + problemNilUUIDSQL + `
				AND element_mapping_revision IS NOT NULL AND element_snapshot IS NOT NULL
			)`,
			"problem_revision_reward_snapshot_bound": fmt.Sprintf(
				`jsonb_typeof(tag_snapshot) = 'object'
				AND octet_length(tag_snapshot::text) <= %d
				AND (element_snapshot IS NULL OR (
					jsonb_typeof(element_snapshot) = 'object'
					AND octet_length(element_snapshot::text) <= %d
				))`,
				problemcontract.MaximumRewardSnapshotJSONBytes,
				problemcontract.MaximumRewardSnapshotJSONBytes,
			),
			"problem_revision_wall_covers_cpu": "wall_time_limit_ms >= cpu_time_limit_ms",
			"problem_revision_limit_bounds": fmt.Sprintf(
				`cpu_time_limit_ms BETWEEN %d AND %d
				AND wall_time_limit_ms BETWEEN %d AND %d
				AND memory_limit_kb BETWEEN %d AND %d
				AND output_limit_bytes BETWEEN %d AND %d
				AND process_limit BETWEEN %d AND %d`,
				problemcontract.MinimumCPUTimeLimitMilliseconds,
				problemcontract.MaximumCPUTimeLimitMilliseconds,
				problemcontract.MinimumCPUTimeLimitMilliseconds,
				problemcontract.MaximumWallTimeLimitMilliseconds,
				problemcontract.MinimumMemoryLimitKilobytes,
				problemcontract.MaximumMemoryLimitKilobytes,
				problemcontract.MinimumOutputLimitBytes,
				problemcontract.MaximumOutputLimitBytes,
				problemcontract.MinimumProcessLimit,
				problemcontract.MaximumProcessLimit,
			),
			"problem_revision_checker_identity_bound": fmt.Sprintf(
				`octet_length(checker_key) BETWEEN 1 AND %d
				AND octet_length(checker_version) BETWEEN 1 AND %d`,
				problemcontract.MaximumCheckerKeyBytes,
				problemcontract.MaximumCheckerVersionBytes,
			),
			"problem_revision_checker_config_bound": fmt.Sprintf(
				`jsonb_typeof(checker_config) = 'object'
				AND octet_length(checker_config::text) <= %d`,
				problemcontract.MaximumCheckerConfigBytes,
			),
			"problem_revision_runtime_keys_bound": fmt.Sprintf(
				`jsonb_typeof(allowed_runtime_keys) = 'array'
				AND jsonb_array_length(allowed_runtime_keys) BETWEEN 1 AND %d
				AND octet_length(allowed_runtime_keys::text) <= %d
				AND NOT jsonb_path_exists(allowed_runtime_keys, '$[*] ? (@.type() != "string")')`,
				problemcontract.MaximumAllowedRuntimes,
				problemcontract.MaximumAllowedRuntimes*(problemcontract.MaximumRuntimeKeyBytes+3),
			),
		}),
	}
}

func problemAppendOnlyRecordFields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(commonent.NewUUID).
			Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func problemJSONSchemaType() map[string]string {
	return map[string]string{
		dialect.Postgres: "jsonb",
		dialect.SQLite:   "json",
	}
}
