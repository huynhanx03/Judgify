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

// ContestClarification holds private question ownership plus exact approved
// summary/answer pointers. Participant queries never select raw question data.
type ContestClarification struct{ ent.Schema }

func (ContestClarification) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (ContestClarification) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.UUID("requester_id", uuid.UUID{}).Immutable(),
		field.UUID("contest_problem_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Enum("status").Values("open", "answered", "closed", "withdrawn").Default("open"),
		field.Enum("answer_audience").Values("requester", "participants").Default("requester"),
		field.UUID("question_revision_id", uuid.UUID{}).Immutable(),
		field.Enum("question_owner_type").Values("clarification").Default("clarification").Immutable(),
		field.Enum("question_content_kind").Values("question").Default("question").Immutable(),
		field.UUID("question_render_source_id", uuid.UUID{}).Immutable(),
		field.Enum("question_render_source_kind").Values(rendercontract.SourceKindContestClarification).Immutable(),
		field.Bytes("question_normalized_source_checksum").NotEmpty().Immutable(),
		field.UUID("question_render_artifact_id", uuid.UUID{}).Immutable(),
		field.UUID("current_public_question_revision_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("current_public_question_owner_type").Values("clarification").Optional().Nillable(),
		field.Enum("current_public_question_content_kind").Values("public_question_summary").Optional().Nillable(),
		field.UUID("current_public_question_render_source_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("current_public_question_render_source_kind").Values(rendercontract.SourceKindContestClarification).Optional().Nillable(),
		field.Bytes("current_public_question_normalized_source_checksum").Optional().Nillable(),
		field.UUID("current_public_question_render_artifact_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("current_answer_revision_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("current_answer_owner_type").Values("clarification").Optional().Nillable(),
		field.Enum("current_answer_content_kind").Values("answer").Optional().Nillable(),
		field.UUID("current_answer_render_source_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("current_answer_render_source_kind").Values(rendercontract.SourceKindContestClarification).Optional().Nillable(),
		field.Bytes("current_answer_normalized_source_checksum").Optional().Nillable(),
		field.UUID("current_answer_render_artifact_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("version").Min(1).Default(1),
		field.Time("answered_at").Optional().Nillable(),
		field.Time("closed_at").Optional().Nillable(),
		field.Time("withdrawn_at").Optional().Nillable(),
	}
}

func (ContestClarification) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).Ref("clarifications").Field("contest_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("requester", User.Type).Ref("contest_clarifications").Field("requester_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("contest_problem", ContestProblem.Type).Ref("clarifications").Field("contest_problem_id").Unique().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContestClarification) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "status", "created_at", "id"),
		index.Fields("contest_id", "requester_id", "created_at", "id"),
		index.Fields("contest_id", "answer_audience", "status", "answered_at", "id"),
		index.Fields("question_revision_id", "id", "question_owner_type", "question_content_kind", "question_render_source_id", "question_render_source_kind", "question_normalized_source_checksum").Unique(),
		index.Fields("current_public_question_revision_id", "id", "current_public_question_owner_type", "current_public_question_content_kind", "current_public_question_render_source_id", "current_public_question_render_source_kind", "current_public_question_normalized_source_checksum").Unique(),
		index.Fields("current_answer_revision_id", "id", "current_answer_owner_type", "current_answer_content_kind", "current_answer_render_source_id", "current_answer_render_source_kind", "current_answer_normalized_source_checksum").Unique(),
	}
}

func (ContestClarification) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"contest_clarification_nonzero_identity": "id <> " + contentRenderNilUUIDSQL + " AND contest_id <> " + contentRenderNilUUIDSQL +
			" AND requester_id <> " + contentRenderNilUUIDSQL + " AND (contest_problem_id IS NULL OR contest_problem_id <> " + contentRenderNilUUIDSQL + ")" +
			" AND question_revision_id <> " + contentRenderNilUUIDSQL + " AND question_render_source_id <> " + contentRenderNilUUIDSQL +
			" AND question_render_artifact_id <> " + contentRenderNilUUIDSQL,
		"contest_clarification_question_checksum": "octet_length(question_normalized_source_checksum) = 32 AND question_normalized_source_checksum <> " + contentRenderZeroDigestSQL,
		"contest_clarification_public_pointer":    nullableClarificationPointerCheck("current_public_question", "public_question_summary"),
		"contest_clarification_answer_pointer":    nullableClarificationPointerCheck("current_answer", "answer"),
		"contest_clarification_audience": `answer_audience <> 'participants' OR (
			current_public_question_revision_id IS NOT NULL AND current_answer_revision_id IS NOT NULL
		)`,
		"contest_clarification_state": `(
			status = 'open' AND answered_at IS NULL AND closed_at IS NULL AND withdrawn_at IS NULL AND current_answer_revision_id IS NULL
		) OR (
			status = 'answered' AND answered_at IS NOT NULL AND closed_at IS NULL AND withdrawn_at IS NULL AND current_answer_revision_id IS NOT NULL
		) OR (
			status = 'closed' AND closed_at IS NOT NULL AND withdrawn_at IS NULL
		) OR (
			status = 'withdrawn' AND withdrawn_at IS NOT NULL AND answered_at IS NULL AND closed_at IS NULL AND current_answer_revision_id IS NULL
		)`,
	})}
}

func nullableClarificationPointerCheck(prefix, contentKind string) string {
	return `(
		` + prefix + `_revision_id IS NULL AND ` + prefix + `_owner_type IS NULL AND ` + prefix + `_content_kind IS NULL
		AND ` + prefix + `_render_source_id IS NULL AND ` + prefix + `_render_source_kind IS NULL
		AND ` + prefix + `_normalized_source_checksum IS NULL AND ` + prefix + `_render_artifact_id IS NULL
	) OR (
		` + prefix + `_revision_id IS NOT NULL AND ` + prefix + `_revision_id <> ` + contentRenderNilUUIDSQL + `
		AND ` + prefix + `_owner_type = 'clarification' AND ` + prefix + `_content_kind = '` + contentKind + `'
		AND ` + prefix + `_render_source_id IS NOT NULL AND ` + prefix + `_render_source_id <> ` + contentRenderNilUUIDSQL + `
		AND ` + prefix + `_render_source_kind = '` + rendercontract.SourceKindContestClarification + `'
		AND octet_length(` + prefix + `_normalized_source_checksum) = 32
		AND ` + prefix + `_normalized_source_checksum <> ` + contentRenderZeroDigestSQL + `
		AND ` + prefix + `_render_artifact_id IS NOT NULL AND ` + prefix + `_render_artifact_id <> ` + contentRenderNilUUIDSQL + `
	)`
}
