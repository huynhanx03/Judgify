package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Role holds the schema definition for the Role entity.
type Role struct {
	ent.Schema
}

// Mixin of the Role.
func (Role) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Role.
func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			Unique().
			NotEmpty().
			Comment("Stable authorization key used by Casbin, for example admin, teacher, student"),
		field.String("name").
			NotEmpty(),
		field.String("description").
			Optional().
			Nillable(),
		field.Bool("is_system").
			Default(false).
			Comment("System roles are seeded and should not be deleted casually"),
		field.Enum("status").
			Values("active", "archived").
			Default("active"),
		field.Int64("version").
			Min(1).
			Default(1),
	}
}

// Edges of the Role.
func (Role) Edges() []ent.Edge {
	return nil
}

func (Role) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"role_status_closed": "status IN ('active', 'archived')",
		}),
	}
}
