package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// AttributeDefinition holds the schema definition for the AttributeDefinition entity.
type AttributeDefinition struct {
	ent.Schema
}

// Mixin of the AttributeDefinition.
func (AttributeDefinition) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the AttributeDefinition.
func (AttributeDefinition) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			Unique().
			NotEmpty().
			MaxLen(50).
			Immutable(),
		field.Enum("kind").
			Values("system", "custom").
			Default("custom").
			Immutable(),
		field.Bool("required_on_onboarding").
			Default(false),
		field.Enum("status").
			Values("active", "archived").
			Default("active"),
		field.UUID("draft_revision_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.UUID("active_revision_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.Int64("version").
			Min(1).
			Default(1),
	}
}

// Edges of the AttributeDefinition.
func (AttributeDefinition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("values", UserAttributeValue.Type),
		edge.To("revisions", AttributeDefinitionRevision.Type),
		edge.To("draft_revision", AttributeDefinitionRevision.Type).
			Field("draft_revision_id").
			Unique(),
		edge.To("active_revision", AttributeDefinitionRevision.Type).
			Field("active_revision_id").
			Unique(),
	}
}

func (AttributeDefinition) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"attribute_definition_kind_closed":   "kind IN ('system', 'custom')",
			"attribute_definition_status_closed": "status IN ('active', 'archived')",
		}),
	}
}
