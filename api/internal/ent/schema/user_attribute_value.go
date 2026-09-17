package schema

import (
	"encoding/json"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// UserAttributeValue holds the schema definition for the UserAttributeValue entity.
type UserAttributeValue struct {
	ent.Schema
}

// Mixin of the UserAttributeValue.
func (UserAttributeValue) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the UserAttributeValue.
func (UserAttributeValue) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("attribute_id", uuid.UUID{}),
		field.UUID("definition_revision_id", uuid.UUID{}),
		field.UUID("source_revision_id", uuid.UUID{}),
		field.JSON("canonical_value", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}),
		field.Int64("source_value_version").
			Min(1).
			Default(1),
		field.Int64("version").
			Min(1).
			Default(1),
		// Deterministic scalar projection used only to prove cross-revision
		// conversion equivalence. It is derived from canonical_value and is
		// never an independent write input.
		field.Text("value"),
	}
}

// Edges of the UserAttributeValue.
func (UserAttributeValue) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("attributes").
			Field("user_id").
			Unique().
			Required(),
		edge.From("definition", AttributeDefinition.Type).
			Ref("values").
			Field("attribute_id").
			Unique().
			Required(),
		edge.From("definition_revision", AttributeDefinitionRevision.Type).
			Ref("values").
			Field("definition_revision_id").
			Unique().
			Required(),
		edge.From("source_revision", AttributeDefinitionRevision.Type).
			Ref("derived_values").
			Field("source_revision_id").
			Unique().
			Required(),
	}
}

// Indexes of the UserAttributeValue.
func (UserAttributeValue) Indexes() []ent.Index {
	return []ent.Index{
		// One current source and one candidate shadow can coexist. The
		// revision is therefore part of the identity, not merely an index
		// suffix for lookup.
		index.Fields("user_id", "attribute_id", "definition_revision_id").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
