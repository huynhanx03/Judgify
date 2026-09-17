package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
)

// ContestAnnouncement is the mutable state/pointer aggregate. Revisions and
// render artifacts remain append-only.
type ContestAnnouncement struct{ ent.Schema }

func (ContestAnnouncement) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (ContestAnnouncement) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.Enum("status").Values("draft", "published", "withdrawn").Default("draft"),
		field.Enum("audience").Values("public", "participants", "jury"),
		field.UUID("current_draft_revision_id", uuid.UUID{}),
		field.Enum("current_draft_owner_type").Values("announcement").Default("announcement"),
		field.Enum("current_draft_content_kind").Values("announcement_body").Default("announcement_body"),
		field.UUID("current_draft_render_source_id", uuid.UUID{}),
		field.Enum("current_draft_render_source_kind").Values(rendercontract.SourceKindContestAnnouncement),
		field.Bytes("current_draft_normalized_source_checksum").NotEmpty(),
		field.UUID("current_draft_render_artifact_id", uuid.UUID{}),
		field.UUID("published_revision_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("published_owner_type").Values("announcement").Optional().Nillable(),
		field.Enum("published_content_kind").Values("announcement_body").Optional().Nillable(),
		field.UUID("published_render_source_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("published_render_source_kind").Values(rendercontract.SourceKindContestAnnouncement).Optional().Nillable(),
		field.Bytes("published_normalized_source_checksum").Optional().Nillable(),
		field.UUID("published_render_artifact_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("version").Min(1).Default(1),
		field.Time("scheduled_for").Optional().Nillable(),
		field.Time("published_at").Optional().Nillable(),
		field.Time("withdrawn_at").Optional().Nillable(),
	}
}

func (ContestAnnouncement) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).Ref("announcements").Field("contest_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContestAnnouncement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "status", "published_at", "id"),
		index.Fields("contest_id", "audience", "status", "published_at", "id"),
		index.Fields("status", "scheduled_for", "id"),
		index.Fields("current_draft_revision_id", "id", "current_draft_owner_type", "current_draft_content_kind", "current_draft_render_source_id", "current_draft_render_source_kind", "current_draft_normalized_source_checksum").Unique(),
		index.Fields("published_revision_id", "id", "published_owner_type", "published_content_kind", "published_render_source_id", "published_render_source_kind", "published_normalized_source_checksum").Unique(),
	}
}

func (ContestAnnouncement) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"contest_announcement_nonzero_identity": "id <> " + contentRenderNilUUIDSQL + " AND contest_id <> " + contentRenderNilUUIDSQL +
			" AND current_draft_revision_id <> " + contentRenderNilUUIDSQL + " AND current_draft_render_source_id <> " + contentRenderNilUUIDSQL +
			" AND current_draft_render_artifact_id <> " + contentRenderNilUUIDSQL,
		"contest_announcement_draft_checksum": "octet_length(current_draft_normalized_source_checksum) = 32 AND current_draft_normalized_source_checksum <> " + contentRenderZeroDigestSQL,
		"contest_announcement_published_pointer": `(
			published_revision_id IS NULL AND published_owner_type IS NULL AND published_content_kind IS NULL
			AND published_render_source_id IS NULL AND published_render_source_kind IS NULL
			AND published_normalized_source_checksum IS NULL AND published_render_artifact_id IS NULL
		) OR (
			published_revision_id IS NOT NULL AND published_revision_id <> ` + contentRenderNilUUIDSQL + `
			AND published_owner_type = 'announcement' AND published_content_kind = 'announcement_body'
			AND published_render_source_id IS NOT NULL AND published_render_source_id <> ` + contentRenderNilUUIDSQL + `
			AND published_render_source_kind = '` + rendercontract.SourceKindContestAnnouncement + `'
			AND octet_length(published_normalized_source_checksum) = 32
			AND published_normalized_source_checksum <> ` + contentRenderZeroDigestSQL + `
			AND published_render_artifact_id IS NOT NULL AND published_render_artifact_id <> ` + contentRenderNilUUIDSQL + `
		)`,
		"contest_announcement_state": `(
			status = 'draft' AND published_at IS NULL AND withdrawn_at IS NULL
		) OR (
			status = 'published' AND published_at IS NOT NULL AND withdrawn_at IS NULL AND published_revision_id IS NOT NULL
		) OR (
			status = 'withdrawn' AND withdrawn_at IS NOT NULL
		)`,
	})}
}
