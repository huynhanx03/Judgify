package schema

import (
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
)

// JudgeWorkerCapability persists one complete, passing tuple row. There are no
// independent runtime/checker/version arrays from which combinations can be
// synthesized.
type JudgeWorkerCapability struct{ ent.Schema }

func (JudgeWorkerCapability) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (JudgeWorkerCapability) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("worker_instance_id", uuid.UUID{}).Immutable(),
		field.UUID("boot_id", uuid.UUID{}).Immutable(),
		field.String("judge_spec_protocol").
			NotEmpty().
			MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("runtime_key").
			NotEmpty().
			MaxLen(64).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Immutable(),
		field.String("runtime_profile_version").
			NotEmpty().
			MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("compiler_version").
			NotEmpty().
			MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("runtime_bootstrap_version").
			NotEmpty().
			MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("compile_command_id").
			NotEmpty().
			MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("run_command_id").
			NotEmpty().
			MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("image_digest").
			NotEmpty().
			MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).
			Immutable(),
		field.String("manifest_version").
			NotEmpty().
			MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("manifest_checksum").
			NotEmpty().
			MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).
			Immutable(),
		field.String("supervisor_protocol").
			NotEmpty().
			MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("checker_key").
			NotEmpty().
			MaxLen(64).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Immutable(),
		field.String("checker_version").
			NotEmpty().
			MaxLen(64).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Immutable(),
		field.String("capability_profile_hash").
			NotEmpty().
			MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).
			Immutable(),
		field.String("smoke_checksum").
			NotEmpty().
			MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).
			Immutable(),
		field.String("checker_implementation_checksum").
			NotEmpty().
			MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).
			Immutable(),
		field.String("golden_checksum").
			NotEmpty().
			MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).
			Immutable(),
		field.Time("probed_at").Immutable(),
		field.Bool("passing").Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (JudgeWorkerCapability) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("worker_instance", JudgeWorkerInstance.Type).
			Ref("capabilities").
			Field("worker_instance_id").
			Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("activations", ActivatedJudgeCapability.Type),
	}
}

func (JudgeWorkerCapability) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("id", "worker_instance_id", "boot_id").Unique(),
		index.Fields(
			"id", "worker_instance_id", "boot_id",
			"judge_spec_protocol", "runtime_key", "runtime_profile_version",
			"compiler_version", "runtime_bootstrap_version", "compile_command_id", "run_command_id",
			"image_digest", "manifest_version", "manifest_checksum", "supervisor_protocol",
			"checker_key", "checker_version", "capability_profile_hash",
			"checker_implementation_checksum", "smoke_checksum", "golden_checksum",
		).Unique().StorageKey("judge_worker_capabilities_activation_proof_identity"),
		index.Fields("worker_instance_id", "boot_id", "capability_profile_hash").Unique(),
		index.Fields("capability_profile_hash", "probed_at"),
		index.Fields("worker_instance_id", "probed_at"),
	}
}

func (JudgeWorkerCapability) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"judge_worker_capability_nonzero_identity": "id <> " + judgeWorkerNilUUIDSQL +
				" AND worker_instance_id <> " + judgeWorkerNilUUIDSQL + " AND boot_id <> " + judgeWorkerNilUUIDSQL,
			"judge_worker_capability_identifiers": `judge_spec_protocol ~ '^[a-z0-9][a-z0-9._-]{0,127}$'
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
			"judge_worker_capability_pinned_digests": `image_digest ~ '^sha256:[0-9a-f]{64}$'
				AND manifest_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND capability_profile_hash ~ '^sha256:[0-9a-f]{64}$'
				AND smoke_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND checker_implementation_checksum ~ '^sha256:[0-9a-f]{64}$'
				AND golden_checksum ~ '^sha256:[0-9a-f]{64}$'`,
			"judge_worker_capability_passing_only": "passing",
		}),
	}
}
