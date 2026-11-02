package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

// CheckerProfile stores one trusted built-in checker implementation identity.
type CheckerProfile struct{ ent.Schema }

func (CheckerProfile) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (CheckerProfile) Fields() []ent.Field {
	return []ent.Field{
		field.String("checker_key").
			NotEmpty().
			MaxLen(problemcontract.MaximumCheckerKeyBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Immutable(),
		field.String("checker_version").
			NotEmpty().
			MaxLen(problemcontract.MaximumCheckerVersionBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(64)"}).
			Immutable(),
		field.String("implementation_checksum").
			NotEmpty().
			MaxLen(maximumDigestTextBytes).
			SchemaType(map[string]string{dialect.Postgres: "character varying(71)"}).
			Immutable(),
		field.Enum("config_kind").Values("none", "float_tolerance").Immutable(),
		field.Bool("active").Default(false),
	}
}

func (CheckerProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("checker_key", "checker_version").Unique(),
		index.Fields("checker_key", "active", "checker_version"),
	}
}

func (CheckerProfile) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"checker_profile_identity_closed_shape": `checker_key ~ '^[a-z0-9][a-z0-9._-]*$'
				AND checker_version ~ '^[a-z0-9][a-z0-9._-]*$'`,
			"checker_profile_implementation_digest": "implementation_checksum ~ '^sha256:[0-9a-f]{64}$'",
			"checker_profile_config_kind_closed":    "config_kind IN ('none', 'float_tolerance')",
		}),
	}
}
