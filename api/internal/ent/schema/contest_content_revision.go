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

	contestentity "github.com/huynhanx03/judgify/internal/contest/domain"
	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
)

// ContestContentRevision stores immutable raw authored Markdown bound to the
// exact shared render source and artifact selected when the revision commits.
type ContestContentRevision struct{ ent.Schema }

func (ContestContentRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (ContestContentRevision) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.Enum("owner_type").Values("announcement", "clarification").Immutable(),
		field.UUID("owner_id", uuid.UUID{}).Immutable(),
		field.Enum("content_kind").Values("announcement_body", "question", "public_question_summary", "answer").Immutable(),
		field.Int64("revision_number").Min(1).Immutable(),
		field.String("title").MaxLen(contestentity.MaximumAnnouncementTitleBytes).Optional().Nillable().Immutable(),
		field.Text("markdown").NotEmpty().Immutable().Sensitive(),
		field.UUID("author_id", uuid.UUID{}).Immutable(),
		field.UUID("render_source_id", uuid.UUID{}).Immutable(),
		field.Enum("render_source_kind").Values(
			rendercontract.SourceKindContestAnnouncement,
			rendercontract.SourceKindContestClarification,
		).Immutable(),
		field.Bytes("normalized_source_checksum").NotEmpty().Immutable(),
		field.UUID("render_artifact_id", uuid.UUID{}).Immutable(),
		field.Time("created_at").Default(func() time.Time { return time.Now().UTC() }).Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (ContestContentRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("author", User.Type).
			Ref("contest_content_revisions").
			Field("author_id").
			Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContestContentRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_type", "owner_id", "content_kind", "revision_number").Unique(),
		index.Fields("id", "render_source_id", "render_source_kind", "normalized_source_checksum").Unique(),
		index.Fields("id", "owner_type", "owner_id", "content_kind", "render_source_id", "render_source_kind", "normalized_source_checksum").Unique(),
		index.Fields("render_artifact_id", "render_source_id", "render_source_kind", "normalized_source_checksum").Unique(),
		index.Fields("owner_type", "owner_id", "created_at", "id"),
	}
}

func (ContestContentRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"contest_content_revision_nonzero_identity": "id <> " + contentRenderNilUUIDSQL +
			" AND owner_id <> " + contentRenderNilUUIDSQL +
			" AND author_id <> " + contentRenderNilUUIDSQL +
			" AND render_source_id <> " + contentRenderNilUUIDSQL +
			" AND render_artifact_id <> " + contentRenderNilUUIDSQL,
		"contest_content_revision_checksum": "octet_length(normalized_source_checksum) = 32 AND normalized_source_checksum <> " + contentRenderZeroDigestSQL,
		"contest_content_revision_shape": `(
			owner_type = 'announcement' AND content_kind = 'announcement_body'
			AND render_source_kind = '` + rendercontract.SourceKindContestAnnouncement + `'
			AND title IS NOT NULL
			AND octet_length(title) BETWEEN 1 AND ` + fmt.Sprint(contestentity.MaximumAnnouncementTitleBytes) + `
			AND octet_length(markdown) BETWEEN 1 AND ` + fmt.Sprint(contestentity.MaximumAnnouncementMarkdownBytes) + `
		) OR (
			owner_type = 'clarification' AND content_kind IN ('question', 'public_question_summary', 'answer')
			AND render_source_kind = '` + rendercontract.SourceKindContestClarification + `'
			AND title IS NULL
			AND octet_length(markdown) BETWEEN 1 AND CASE content_kind
				WHEN 'question' THEN ` + fmt.Sprint(contestentity.MaximumClarificationQuestionBytes) + `
				WHEN 'public_question_summary' THEN ` + fmt.Sprint(contestentity.MaximumClarificationSummaryBytes) + `
				ELSE ` + fmt.Sprint(contestentity.MaximumClarificationAnswerBytes) + ` END
		)`,
	})}
}
