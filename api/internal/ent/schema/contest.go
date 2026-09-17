package schema

import (
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

// Contest holds the schema definition for the Contest entity.
type Contest struct {
	ent.Schema
}

func (Contest) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

func (Contest) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").NotEmpty().MaxLen(300),
		field.Text("description").Optional(),
		field.Time("start_time"),
		field.Time("end_time"),
		field.Enum("status").
			Values("draft", "upcoming", "running", "ended", "cancelled").
			Default("draft"),
		field.Int64("version").Min(1).Default(1),
		field.UUID("author_id", uuid.UUID{}),
		field.Enum("registration_mode").Values("registered", "open").Default("registered"),
		field.Time("registration_opens_at").Optional().Nillable(),
		field.Time("registration_closes_at").Optional().Nillable(),
		field.Int("max_participants").Positive().Optional().Nillable().Comment("NULL = unlimited"),
		field.Int64("next_submission_sequence").Min(1).Default(1),
		field.Enum("scoring_policy").Values("icpc").Default("icpc"),
		field.Int64("scoring_policy_version").Min(1).Default(1),
		field.Int64("standings_projection_version").Min(1).Default(1),
		field.Time("freeze_standings_at").Optional().Nillable(),
		field.Bool("rated").Default(true),
		field.String("rating_algorithm_version").
			NotEmpty().
			MaxLen(64).
			Default("expected-rank-v1"),
		field.Time("published_at").Optional().Nillable(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("ended_at").Optional().Nillable(),
		field.Time("cancelled_at").Optional().Nillable(),
		field.Time("rating_completed_at").Optional().Nillable(),
	}
}

func (Contest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("start_time"),
		index.Fields("status", "start_time", "id"),
		index.Fields("status", "rating_completed_at", "end_time", "id"),
		index.Fields("status", "rated", "rating_completed_at", "end_time", "id"),
	}
}

func (Contest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("author", User.Type).Ref("contests").Field("author_id").Unique().Required(),
		// Draft selections remain mutable but ordered. Publication freezes
		// their exact authoring and executable identities in ContestProblem.
		edge.To("draft_problems", ContestDraftProblem.Type),
		edge.To("contest_problems", ContestProblem.Type),
		edge.To("transitions", ContestTransition.Type),
		edge.To("registrations", ContestRegistration.Type),
		edge.To("standings", ContestStanding.Type),
		edge.To("submissions", Submission.Type),
		edge.To("rating_histories", RatingHistory.Type),
		edge.To("rating_sequence_slots", RatingSequenceSlot.Type),
		edge.To("rating_runs", ContestRatingRun.Type),
		edge.To("announcements", ContestAnnouncement.Type),
		edge.To("clarifications", ContestClarification.Type),
		edge.To("content_commands", ContestContentCommand.Type),
		edge.To("scoring_attempts", ContestScoringAttempt.Type),
		edge.To("problem_results", ContestProblemResult.Type),
	}
}

func (Contest) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"contest_valid_window":                 "start_time < end_time",
			"contest_positive_version":             "version >= 1",
			"contest_positive_submission_sequence": "next_submission_sequence >= 1",
			"contest_positive_scoring_version":     "scoring_policy_version >= 1",
			"contest_positive_standings_version":   "standings_projection_version >= 1",
			"contest_rating_algorithm":             "rating_algorithm_version = 'expected-rank-v1'",
			"contest_rating_completion":            "rating_completed_at IS NULL OR (rated AND status = 'ended' AND ended_at IS NOT NULL AND rating_completed_at >= ended_at)",
			"contest_positive_capacity":            "max_participants IS NULL OR max_participants > 0",
			"contest_registration_policy": `status IN ('draft', 'cancelled') OR ((
				registration_mode = 'registered'
				AND registration_opens_at IS NOT NULL
				AND registration_closes_at IS NOT NULL
				AND registration_opens_at <= registration_closes_at
				AND registration_closes_at <= start_time
			) OR (
				registration_mode = 'open'
				AND registration_opens_at IS NULL
				AND registration_closes_at IS NULL
			))`,
			"contest_freeze_window": "freeze_standings_at IS NULL OR (freeze_standings_at >= start_time AND freeze_standings_at < end_time)",
			"contest_lifecycle_timestamps": `(
				status = 'draft' AND published_at IS NULL AND started_at IS NULL AND ended_at IS NULL AND cancelled_at IS NULL
			) OR (
				status = 'upcoming' AND published_at IS NOT NULL AND started_at IS NULL AND ended_at IS NULL AND cancelled_at IS NULL
			) OR (
				status = 'running' AND published_at IS NOT NULL AND started_at IS NOT NULL AND ended_at IS NULL AND cancelled_at IS NULL
			) OR (
				status = 'ended' AND published_at IS NOT NULL AND started_at IS NOT NULL AND ended_at IS NOT NULL AND cancelled_at IS NULL
			) OR (
				status = 'cancelled' AND cancelled_at IS NOT NULL AND ended_at IS NULL
			)`,
		}),
	}
}
