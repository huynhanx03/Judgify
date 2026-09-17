package schema

import (
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Material holds the schema definition for the Material entity.
type Material struct {
	ent.Schema
}

// Mixin of the Material.
func (Material) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Material.
func (Material) Fields() []ent.Field {
	return []ent.Field{
		// Transitional legacy projection columns remain nullable for one rollout
		// so the existing CRUD read adapter can be removed after backfill. New
		// authoring and every public read use material_revisions exclusively.
		field.String("title").Optional().MaxLen(255),
		field.String("description").Optional().MaxLen(1000),
		field.Text("content").Optional().Sensitive(),
		field.UUID("difficulty_id", uuid.UUID{}).Optional(),
		field.UUID("category_id", uuid.UUID{}).Optional(),
		field.Int("group_id").Optional().Nillable(),
		field.Int("estimated_read_time").Default(0).NonNegative(),
		// The draft revision pointer is filled only after its immutable revision,
		// source, and artifact have been created in the same transaction. The
		// canonical slug pointer is present at identity creation.
		field.UUID("current_slug_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.Enum("current_slug_state").
			Values("current").
			Default("current"),
		field.UUID("author_id", uuid.UUID{}),
		field.Enum("status").
			Values("draft", "in_review", "published", "archived").
			Default("draft"),
		field.Enum("visibility").
			Values("public", "authenticated").
			Default("public"),
		field.UUID("current_draft_revision_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.UUID("published_revision_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.UUID("published_render_source_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.UUID("published_render_artifact_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.Enum("published_render_source_kind").
			Values("material_body").
			Optional().
			Nillable(),
		field.Bytes("published_source_checksum").
			Optional().
			Nillable(),
		field.Time("published_at").
			Optional().
			Nillable(),
		field.Time("archived_at").
			Optional().
			Nillable(),
		field.Int64("view_count").
			Default(0).
			NonNegative(),
		field.Int64("version").
			Default(1).
			Min(1),
	}
}

// Indexes of the Material.
func (Material) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("current_slug_id", "id", "current_slug_state").Unique(),
		index.Fields("published_revision_id", "id").Unique(),
		index.Fields("published_at", "id"),
		index.Fields("view_count", "id"),
	}
}

// Edges of the Material.
func (Material) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("author", User.Type).Ref("materials").Field("author_id").Unique().Required(),
		edge.From("category", MaterialCategory.Type).Ref("legacy_materials").Field("category_id").Unique(),
		edge.From("difficulty", Difficulty.Type).Ref("materials").Field("difficulty_id").Unique(),
		edge.To("tags", Tag.Type),
		edge.To("revisions", MaterialRevision.Type),
		edge.To("slugs", MaterialSlug.Type),
		edge.To("daily_views", MaterialViewDaily.Type),
	}
}

func (Material) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"material_nonzero_identity": "id <> '00000000-0000-0000-0000-000000000000'::uuid AND author_id <> '00000000-0000-0000-0000-000000000000'::uuid AND current_slug_id <> '00000000-0000-0000-0000-000000000000'::uuid",
			"material_publication_pointer_shape": `(
				published_revision_id IS NULL AND published_render_source_id IS NULL
				AND published_render_artifact_id IS NULL AND published_render_source_kind IS NULL
				AND published_source_checksum IS NULL AND published_at IS NULL
			) OR (
				published_revision_id IS NOT NULL AND published_render_source_id IS NOT NULL
				AND published_render_artifact_id IS NOT NULL
				AND published_render_source_kind = 'material_body'
				AND published_source_checksum IS NOT NULL
				AND octet_length(published_source_checksum) = 32
				AND published_at IS NOT NULL
			)`,
			"material_lifecycle_shape": `(
				status = 'archived' AND archived_at IS NOT NULL
			) OR (
				status <> 'archived' AND archived_at IS NULL
			)`,
			"material_positive_version": fmt.Sprintf("version BETWEEN 1 AND %d", int64(^uint64(0)>>1)),
		}),
	}
}
