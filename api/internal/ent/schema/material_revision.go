package schema

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

const (
	materialMaximumTitleBytes       = 255
	materialMaximumDescriptionBytes = 1000
	materialMaximumChangeBytes      = 500
	materialMaximumRevisionNumber   = int64(1_000_000_000)
)

// MaterialRevision is an append-only authored snapshot. Publication changes
// only the stable material pointers and never mutates this row.
type MaterialRevision struct{ ent.Schema }

func (MaterialRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (MaterialRevision) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("material_id", uuid.UUID{}).Immutable(),
		field.Int64("revision_number").Min(1).Max(materialMaximumRevisionNumber).Immutable(),
		field.String("title").NotEmpty().MaxLen(materialMaximumTitleBytes).Immutable(),
		field.String("description").NotEmpty().MaxLen(materialMaximumDescriptionBytes).Immutable(),
		field.Text("content_markdown").NotEmpty().MaxLen(1 << 20).Immutable().Sensitive(),
		field.UUID("category_id", uuid.UUID{}).Immutable(),
		field.UUID("difficulty_id", uuid.UUID{}).Immutable(),
		field.Int("estimated_read_minutes").Min(1).Max(24 * 60).Immutable(),
		field.UUID("author_id", uuid.UUID{}).Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
		field.String("change_summary").NotEmpty().MaxLen(materialMaximumChangeBytes).Immutable(),
		field.Bytes("normalized_source_checksum").NotEmpty().Immutable(),
		field.UUID("render_source_id", uuid.UUID{}).Immutable(),
		field.Enum("render_source_kind").Values("material_body").Immutable(),
		field.UUID("render_artifact_id", uuid.UUID{}).Immutable(),
	}
}

func (MaterialRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("material", Material.Type).
			Ref("revisions").
			Field("material_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("category", MaterialCategory.Type).
			Ref("revisions").
			Field("category_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("difficulty", Difficulty.Type).
			Ref("material_revisions").
			Field("difficulty_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("author", User.Type).
			Ref("material_revisions").
			Field("author_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("tags", MaterialRevisionTag.Type).Immutable(),
	}
}

func (MaterialRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("material_id", "revision_number").Unique(),
		index.Fields("id", "material_id").Unique(),
		index.Fields("id", "render_source_id", "render_source_kind", "normalized_source_checksum").Unique(),
		index.Fields("render_artifact_id", "render_source_id", "render_source_kind", "normalized_source_checksum").Unique(),
		index.Fields("render_source_id").Unique(),
		index.Fields("render_artifact_id").Unique(),
		index.Fields("category_id", "created_at", "id"),
		index.Fields("difficulty_id", "created_at", "id"),
	}
}

func (MaterialRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"material_revision_nonzero_identity": `id <> '00000000-0000-0000-0000-000000000000'::uuid
				AND material_id <> '00000000-0000-0000-0000-000000000000'::uuid
				AND category_id <> '00000000-0000-0000-0000-000000000000'::uuid
				AND difficulty_id <> '00000000-0000-0000-0000-000000000000'::uuid
				AND author_id <> '00000000-0000-0000-0000-000000000000'::uuid
				AND render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid
				AND render_artifact_id <> '00000000-0000-0000-0000-000000000000'::uuid`,
			"material_revision_source_kind":    "render_source_kind = 'material_body'",
			"material_revision_checksum_shape": "octet_length(normalized_source_checksum) = 32 AND normalized_source_checksum <> decode(repeat('00', 32), 'hex')",
			"material_revision_text_bounds": fmt.Sprintf(
				"octet_length(title) BETWEEN 1 AND %d AND octet_length(description) BETWEEN 1 AND %d AND octet_length(content_markdown) BETWEEN 1 AND %d AND octet_length(change_summary) BETWEEN 1 AND %d",
				materialMaximumTitleBytes,
				materialMaximumDescriptionBytes,
				1<<20,
				materialMaximumChangeBytes,
			),
		}),
	}
}
