package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

const profileSchemaStateKey = "onboarding"

// ProfileSchemaState is the singleton read/write fence for the exact active
// onboarding schema. Registration takes a shared row lock before validating
// the required set; definition lifecycle commands take an exclusive lock
// before changing any fact that can affect that set. This prevents a newly
// required definition from appearing as a phantom midway through signup.
type ProfileSchemaState struct{ ent.Schema }

func (ProfileSchemaState) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (ProfileSchemaState) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			NotEmpty().
			MaxLen(32).
			Immutable(),
		field.Int64("revision").
			Min(1).
			Default(1),
	}
}

func (ProfileSchemaState) Indexes() []ent.Index {
	return []ent.Index{index.Fields("key").Unique()}
}

func (ProfileSchemaState) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"profile_schema_state_identity": "id <> " + problemNilUUIDSQL,
			"profile_schema_state_key":      "key = '" + profileSchemaStateKey + "'",
		}),
	}
}
