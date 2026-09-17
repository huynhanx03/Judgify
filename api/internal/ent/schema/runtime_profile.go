package schema

import (
	"encoding/json"
	"fmt"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

const (
	maximumRuntimeDisplayNameBytes = 128
	maximumRuntimeVersionBytes     = 128
	maximumCommandIDBytes          = 128
	maximumDigestTextBytes         = 71
	maximumArtifactManifestBytes   = 32 << 10
)

// RuntimeProfile stores a code/config-owned runtime version. Admin data names
// trusted command IDs only; it never stores arbitrary command or shell text.
type RuntimeProfile struct{ ent.Schema }

func (RuntimeProfile) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (RuntimeProfile) Fields() []ent.Field {
	return []ent.Field{
		field.String("runtime_key").
			NotEmpty().
			MaxLen(problemcontract.MaximumRuntimeKeyBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Immutable(),
		field.String("language").
			NotEmpty().
			MaxLen(problemcontract.MaximumRuntimeKeyBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Immutable(),
		field.String("display_name").
			NotEmpty().
			MaxLen(maximumRuntimeDisplayNameBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("profile_version").
			NotEmpty().
			MaxLen(maximumRuntimeVersionBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("compiler_version").
			NotEmpty().
			MaxLen(maximumRuntimeVersionBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("runtime_bootstrap_version").
			NotEmpty().
			MaxLen(maximumRuntimeVersionBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("sandbox_image_digest").
			NotEmpty().
			MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).
			Immutable(),
		field.String("compile_command_id").
			NotEmpty().
			MaxLen(maximumCommandIDBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("run_command_id").
			NotEmpty().
			MaxLen(maximumCommandIDBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.Int("default_compile_timeout_ms").
			Min(1).
			Max(60_000).
			SchemaType(map[string]string{dialect.Postgres: "integer"}).
			Immutable(),
		field.String("manifest_version").
			NotEmpty().
			MaxLen(maximumRuntimeVersionBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).
			Immutable(),
		field.String("manifest_checksum").
			NotEmpty().
			MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).
			Immutable(),
		field.JSON("artifact_manifest", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).
			Immutable(),
		field.Bool("active").Default(false),
	}
}

func (RuntimeProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("runtime_key", "profile_version").Unique(),
		index.Fields("runtime_key", "active", "profile_version"),
		index.Fields("sandbox_image_digest"),
	}
}

func (RuntimeProfile) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"runtime_profiles_default_compile_timeout_ms": "default_compile_timeout_ms BETWEEN 1 AND 60000",
			"runtime_profile_key_closed_shape": fmt.Sprintf(
				"octet_length(runtime_key) BETWEEN 1 AND %d AND runtime_key ~ '^[a-z0-9][a-z0-9._-]*$'",
				problemcontract.MaximumRuntimeKeyBytes,
			),
			"runtime_profile_version_closed_shape": `profile_version ~ '^[a-z0-9][a-z0-9._-]*$'
				AND manifest_version ~ '^[a-z0-9][a-z0-9._-]*$'`,
			"runtime_profile_command_ids_closed_shape": `compile_command_id ~ '^[a-z0-9][a-z0-9._-]*$'
				AND run_command_id ~ '^[a-z0-9][a-z0-9._-]*$'`,
			"runtime_profile_pinned_digests": `sandbox_image_digest ~ '^sha256:[0-9a-f]{64}$'
				AND manifest_checksum ~ '^sha256:[0-9a-f]{64}$'`,
			"runtime_profile_artifact_manifest_bound": fmt.Sprintf(
				"jsonb_typeof(artifact_manifest) = 'array' AND jsonb_array_length(artifact_manifest) > 0 AND octet_length(artifact_manifest::text) <= %d",
				maximumArtifactManifestBytes,
			),
		}),
	}
}
