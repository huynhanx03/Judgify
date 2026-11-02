package schema

import (
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

// ActivatedJudgeCapability is the explicit platform authorization for later
// API JudgeSpec emission. Worker evidence never creates this row directly.
type ActivatedJudgeCapability struct{ ent.Schema }

func (ActivatedJudgeCapability) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (ActivatedJudgeCapability) Fields() []ent.Field {
	return []ent.Field{
		field.String("release_id").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("approved_release_manifest_checksum").NotEmpty().MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.UUID("actor_id", uuid.UUID{}).Immutable(),
		field.UUID("audit_id", uuid.UUID{}).Immutable(),
		field.String("reason").NotEmpty().MaxLen(500).
			SchemaType(map[string]string{dialect.Postgres: "character varying(500)"}).Immutable(),
		field.String("judge_spec_protocol").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("runtime_key").NotEmpty().MaxLen(64).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).Immutable(),
		field.String("runtime_profile_version").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("compiler_version").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("runtime_bootstrap_version").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("compile_command_id").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("run_command_id").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("image_digest").NotEmpty().MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("manifest_version").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("manifest_checksum").NotEmpty().MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("supervisor_protocol").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.String("checker_key").NotEmpty().MaxLen(64).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).Immutable(),
		field.String("checker_version").NotEmpty().MaxLen(64).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).Immutable(),
		field.String("capability_profile_hash").NotEmpty().MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("checker_implementation_checksum").NotEmpty().MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("smoke_checksum").NotEmpty().MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.String("golden_checksum").NotEmpty().MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).Immutable(),
		field.UUID("proof_worker_instance_id", uuid.UUID{}).Immutable(),
		field.UUID("proof_boot_id", uuid.UUID{}).Immutable(),
		field.UUID("proof_evidence_id", uuid.UUID{}).Immutable(),
		field.Enum("state").Values("active", "retired").Default("active").
			SchemaType(map[string]string{dialect.Postgres: "character varying(16)"}),
		field.Time("activated_at").Immutable(),
		field.Time("retired_at").Optional().Nillable(),
		field.UUID("retired_by", uuid.UUID{}).Optional().Nillable(),
		field.UUID("retirement_audit_id", uuid.UUID{}).Optional().Nillable(),
		field.String("retirement_reason").MaxLen(500).Optional().Nillable().
			SchemaType(map[string]string{dialect.Postgres: "character varying(500)"}),
	}
}

func (ActivatedJudgeCapability) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("proof_evidence", JudgeWorkerCapability.Type).
			Ref("activations").Field("proof_evidence_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ActivatedJudgeCapability) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("release_id", "capability_profile_hash").Unique(),
		index.Fields(
			"release_id",
			"runtime_key",
			"checker_key",
			"checker_version",
		).
			Unique().
			Annotations(entsql.IndexWhere("state = 'active'")),
		index.Fields("state", "release_id", "capability_profile_hash", "activated_at"),
		index.Fields("proof_worker_instance_id", "proof_boot_id"),
	}
}

func (ActivatedJudgeCapability) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"activated_judge_capability_nonzero_identity": "id <> " + judgeWorkerNilUUIDSQL +
				" AND actor_id <> " + judgeWorkerNilUUIDSQL + " AND audit_id <> " + judgeWorkerNilUUIDSQL +
				" AND proof_worker_instance_id <> " + judgeWorkerNilUUIDSQL + " AND proof_boot_id <> " + judgeWorkerNilUUIDSQL +
				" AND proof_evidence_id <> " + judgeWorkerNilUUIDSQL,
			"activated_judge_capability_identifiers": `release_id ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND judge_spec_protocol ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND runtime_key ~ '^[a-z0-9][a-z0-9._-]{0,63}$'
				AND runtime_profile_version ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND compiler_version ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND runtime_bootstrap_version ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND compile_command_id ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND run_command_id ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND manifest_version ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND supervisor_protocol ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
				AND checker_key ~ '^[a-z0-9][a-z0-9._-]{0,63}$'
				AND checker_version ~ '^[a-z0-9][a-z0-9._-]{0,63}$'`,
			"activated_judge_capability_pinned_digests": `image_digest ~ '^sha256:[0-9a-f]{64}$'
				AND manifest_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND approved_release_manifest_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND capability_profile_hash ~ '^sha256:[0-9a-f]{64}$'
				AND checker_implementation_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND smoke_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND golden_checksum ~ '^sha256:[0-9a-f]{64}$'`,
			"activated_judge_capability_state_times": `(state = 'active' AND retired_at IS NULL AND retired_by IS NULL AND retirement_audit_id IS NULL AND retirement_reason IS NULL)
				OR (state = 'retired' AND retired_at IS NOT NULL AND retired_by IS NOT NULL AND retirement_audit_id IS NOT NULL AND retirement_reason IS NOT NULL
					AND retired_at >= activated_at)`,
		})}
}
