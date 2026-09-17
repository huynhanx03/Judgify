package schema

import (
	"encoding/json"

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

// AttributeDefinitionRevision is an immutable, append-only profile schema
// revision. Publishing changes a definition pointer; it never rewrites a
// revision that existing values were validated against.
type AttributeDefinitionRevision struct{ ent.Schema }

func (AttributeDefinitionRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{
		commonent.UUIDMixin{},
		commonent.AppendOnlyMixin{},
	}
}

func (AttributeDefinitionRevision) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("definition_id", uuid.UUID{}).Immutable(),
		field.Int64("revision").Min(1).Immutable(),
		field.Enum("data_type").Values("string", "number", "boolean", "date").Immutable(),
		field.String("label").NotEmpty().MaxLen(128).Immutable(),
		field.String("description").MaxLen(1024).Optional().Immutable(),
		field.Enum("visibility").Values("public", "private", "admin").Immutable(),
		field.Bool("user_editable").Default(false).Immutable(),
		field.Int("display_order").Min(0).Max(10000).Default(0).Immutable(),
		field.JSON("validation", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}).
			Immutable(),
		field.UUID("created_by", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("created_at").Immutable(),
	}
}

func (AttributeDefinitionRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("definition", AttributeDefinition.Type).
			Ref("revisions").
			Field("definition_id").
			Unique().
			Immutable().
			Required(),
		edge.To("values", UserAttributeValue.Type),
		edge.To("derived_values", UserAttributeValue.Type),
	}
}

func (AttributeDefinitionRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("definition_id", "revision").Unique(),
		// PostgreSQL composite provenance foreign keys from profile values use
		// this identity+owner key to prove a referenced revision belongs to the
		// same definition. The primary key alone cannot express that invariant.
		index.Fields("id", "definition_id").Unique(),
		index.Fields("definition_id", "created_at"),
	}
}

func (AttributeDefinitionRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"attribute_revision_data_type_closed":  "data_type IN ('string', 'number', 'boolean', 'date')",
			"attribute_revision_visibility_closed": "visibility IN ('public', 'private', 'admin')",
		}),
	}
}
