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
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

// Problem holds the schema definition for the Problem entity.
type Problem struct {
	ent.Schema
}

// Mixin of the Problem.
func (Problem) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Problem.
func (Problem) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").
			NotEmpty().
			Immutable().
			Unique().
			MaxLen(problemcontract.MaximumSlugBytes),
		field.Enum("lifecycle").
			Values(problemcontract.Lifecycles()...).
			Default(problemcontract.LifecycleDraft),
		field.Int64("version").
			Min(1).
			Default(1),
		field.UUID("tag_mapping_id", uuid.UUID{}).
			Default(e.NewUUID).
			Immutable(),
		field.UUID("element_mapping_id", uuid.UUID{}).
			Default(e.NewUUID).
			Immutable(),
		field.UUID("draft_revision_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.UUID("draft_render_source_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.Enum("draft_render_source_kind").
			Values(problemcontract.RenderSourceKindProblemStatement).
			Optional().
			Nillable(),
		field.Bytes("draft_render_source_checksum").
			Optional().
			Nillable(),
		field.UUID("draft_render_artifact_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.UUID("published_revision_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.UUID("published_render_source_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.Enum("published_render_source_kind").
			Values(problemcontract.RenderSourceKindProblemStatement).
			Optional().
			Nillable(),
		field.Bytes("published_render_source_checksum").
			Optional().
			Nillable(),
		field.UUID("published_render_artifact_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.Time("published_at").
			Optional().
			Nillable(),
		field.Time("archived_at").
			Optional().
			Nillable(),
		// Denormalized current projection used by bounded public/admin list
		// queries. Immutable revision rows remain authoritative and publication
		// advances this projection atomically.
		field.String("title").
			NotEmpty().
			MaxLen(problemcontract.MaximumTitleBytes),
		field.Text("description").
			NotEmpty(),
		field.UUID("difficulty_id", uuid.UUID{}),
		field.Int("time_limit_ms").
			Default(problemcontract.DefaultCPUTimeLimitMilliseconds),
		field.Int("memory_limit_kb").
			Default(problemcontract.DefaultMemoryLimitKilobytes),
		field.UUID("author_id", uuid.UUID{}),
		field.Bool("is_published").
			Default(false),
		field.Int("submission_count").
			Default(0).
			Comment("Denormalized total submission count"),
		field.Int("accepted_count").
			Default(0).
			Comment("Denormalized accepted submission count"),
	}
}

// Edges of the Problem.
func (Problem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("author", User.Type).Ref("problems").Field("author_id").Unique().Required(),
		edge.From("difficulty", Difficulty.Type).Ref("problems").Field("difficulty_id").Unique().Required(),
		edge.To("test_cases", TestCase.Type),
		edge.To("revisions", ProblemRevision.Type),
		edge.To("testset_revisions", TestsetRevision.Type),
		edge.To("contest_draft_memberships", ContestDraftProblem.Type),
		edge.To("contest_problem_snapshots", ContestProblem.Type),
		edge.To("submissions", Submission.Type),
		edge.To("contest_scoring_attempts", ContestScoringAttempt.Type),
		edge.To("contest_problem_results", ContestProblemResult.Type),
		edge.To("tags", Tag.Type),
		edge.To("solvers", UserSolvedProblem.Type),
	}
}

// Indexes of the Problem.
func (Problem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("lifecycle", "created_at", "id"),
		index.Fields("author_id", "lifecycle", "created_at", "id"),
		index.Fields("id", "tag_mapping_id").Unique(),
		index.Fields("id", "element_mapping_id").Unique(),
		index.Fields("draft_revision_id"),
		index.Fields("published_revision_id"),
	}
}

func (Problem) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"problem_slug_bound": fmt.Sprintf(
				"octet_length(slug) BETWEEN 1 AND %d AND slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'",
				problemcontract.MaximumSlugBytes,
			),
			"problem_lifecycle_closed":                    contentRenderClosedDomainCheck("lifecycle", problemcontract.Lifecycles()),
			"problem_positive_version":                    "version >= 1",
			"problem_nonzero_mapping_identities":          "tag_mapping_id <> " + problemNilUUIDSQL + " AND element_mapping_id <> " + problemNilUUIDSQL + " AND tag_mapping_id <> element_mapping_id",
			"problem_nonnegative_counts":                  "submission_count >= 0 AND accepted_count >= 0 AND accepted_count <= submission_count",
			"problem_public_visibility_matches_lifecycle": "is_published = (lifecycle = 'published')",
			"problem_draft_selection_complete":            problemSelectionCompleteCheck("draft"),
			"problem_published_selection_complete":        problemSelectionCompleteCheck("published"),
			"problem_lifecycle_selection": fmt.Sprintf(`(
				lifecycle = '%s'
				AND published_revision_id IS NULL
				AND published_at IS NULL
				AND archived_at IS NULL
			) OR (
				lifecycle = '%s'
				AND published_revision_id IS NOT NULL
				AND published_at IS NOT NULL
				AND archived_at IS NULL
			) OR (
				lifecycle = '%s'
				AND published_revision_id IS NOT NULL
				AND published_at IS NOT NULL
				AND archived_at IS NOT NULL
				AND archived_at >= published_at
			)`,
				problemcontract.LifecycleDraft,
				problemcontract.LifecyclePublished,
				problemcontract.LifecycleArchived,
			),
		}),
	}
}

func problemSelectionCompleteCheck(prefix string) string {
	revision := prefix + "_revision_id"
	source := prefix + "_render_source_id"
	kind := prefix + "_render_source_kind"
	checksum := prefix + "_render_source_checksum"
	artifact := prefix + "_render_artifact_id"

	return fmt.Sprintf(`(
		(%s IS NULL AND %s IS NULL AND %s IS NULL AND %s IS NULL AND %s IS NULL) OR
		(%s IS NOT NULL AND %s <> %s AND
		 %s IS NOT NULL AND %s <> %s AND
		 %s = '%s' AND
		 %s IS NOT NULL AND octet_length(%s) = 32 AND %s <> %s AND
		 %s IS NOT NULL AND %s <> %s)
	)`,
		revision, source, kind, checksum, artifact,
		revision, revision, problemNilUUIDSQL,
		source, source, problemNilUUIDSQL,
		kind, problemcontract.RenderSourceKindProblemStatement,
		checksum, checksum, checksum, problemZeroDigestSQL,
		artifact, artifact, problemNilUUIDSQL,
	)
}
