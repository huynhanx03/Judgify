// Package contract owns stable integration-event names and wire payloads
// emitted by the contest domain. These contracts contain routing metadata and
// invalidation hints only; authored Markdown and rendered private content must
// never cross this boundary.
package contract

import (
	"time"

	"github.com/google/uuid"
)

const (
	AggregateContest          = "contest"
	AggregateAnnouncement     = "announcement"
	AggregateClarification    = "clarification"
	AggregateRegistration     = "contest_registration"
	AggregateRatingRun        = "contest_rating_run"
	AggregateRatingProjection = "rating_projection"

	EventDraftCreatedV1              = "contest.draft_created.v1"
	EventDraftUpdatedV1              = "contest.draft_updated.v1"
	EventDraftDeletedV1              = "contest.draft_deleted.v1"
	EventLifecycleChangedV1          = "contest.lifecycle.changed.v1"
	EventRegistrationChangedV1       = "contest.registration.changed.v1"
	EventAnnouncementPublishedV1     = "contest.announcement.published.v1"
	EventAnnouncementWithdrawnV1     = "contest.announcement.withdrawn.v1"
	EventClarificationSubmittedV1    = "contest.clarification.submitted.v1"
	EventClarificationAnsweredV1     = "contest.clarification.answered.v1"
	EventClarificationClosedV1       = "contest.clarification.closed.v1"
	EventClarificationWithdrawnV1    = "contest.clarification.withdrawn.v1"
	EventRatingCalculatedV1          = "contest.rating.calculated.v1"
	EventRatingSkippedV1             = "contest.rating.skipped.v1"
	EventRatingProjectionActivatedV1 = "contest.rating.projection_activated.v1"
	RatingReratingOperationKind      = "contest.rating.rerating.v1"
	MaximumRatingReratingSequences   = 10000
)

// DraftChangedV1 is an administration-only invalidation/evidence payload. A
// delete deliberately carries only identity, terminal state, and version.
type DraftChangedV1 struct {
	ID                     uuid.UUID `json:"id"`
	Version                int64     `json:"version"`
	State                  string    `json:"state"`
	AuthorID               uuid.UUID `json:"author_id,omitempty"`
	StartTime              time.Time `json:"start_time,omitempty"`
	EndTime                time.Time `json:"end_time,omitempty"`
	RegistrationMode       string    `json:"registration_mode,omitempty"`
	ProblemCount           *int      `json:"problem_count,omitempty"`
	Rated                  bool      `json:"rated,omitempty"`
	RatingAlgorithmVersion string    `json:"rating_algorithm_version,omitempty"`
}

// RatingCalculatedV1 is a bounded invalidation and notification source. It
// carries no whole leaderboard; authorized clients fetch the canonical active
// run and current profile projection through REST.
type RatingCalculatedV1 struct {
	EventID                    uuid.UUID `json:"event_id"`
	RatingRunID                uuid.UUID `json:"rating_run_id"`
	ContestID                  uuid.UUID `json:"contest_id"`
	RatingSequence             int64     `json:"rating_sequence"`
	RunVersion                 int64     `json:"run_version"`
	StandingsProjectionVersion int64     `json:"standings_projection_version"`
	AlgorithmVersion           string    `json:"algorithm_version"`
	ParticipantCount           int       `json:"participant_count"`
	OccurredAt                 time.Time `json:"occurred_at"`
}

// RatingProjectionActivatedV1 invalidates rating/profile views after a fully
// verified historical rebuild. It intentionally carries pointers and
// cardinality only; participant results remain available through canonical
// REST reads.
type RatingProjectionActivatedV1 struct {
	EventID                uuid.UUID `json:"event_id"`
	OperationID            uuid.UUID `json:"operation_id"`
	TriggerContestID       uuid.UUID `json:"trigger_contest_id"`
	FromSequence           int64     `json:"from_sequence"`
	ThroughSequence        int64     `json:"through_sequence"`
	ProjectionRevision     int64     `json:"projection_revision"`
	ProjectionGenerationID uuid.UUID `json:"projection_generation_id"`
	RunChainID             uuid.UUID `json:"run_chain_id"`
	AlgorithmVersion       string    `json:"algorithm_version"`
	OccurredAt             time.Time `json:"occurred_at"`
}

type LifecycleChangedV1 struct {
	EventID    uuid.UUID `json:"event_id"`
	ContestID  uuid.UUID `json:"contest_id"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	Version    int64     `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
}

// RegistrationChangedV1 contains private routing metadata but no participant
// profile or submission data. It is currently terminated by the local event
// observer until a concrete client projection needs realtime invalidation.
type RegistrationChangedV1 struct {
	ContestID      uuid.UUID `json:"contest_id"`
	RegistrationID uuid.UUID `json:"registration_id"`
	UserID         uuid.UUID `json:"user_id"`
	Status         string    `json:"status"`
	Source         string    `json:"source"`
	Version        int64     `json:"version"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type AnnouncementChangedV1 struct {
	EventID        uuid.UUID `json:"event_id"`
	ContestID      uuid.UUID `json:"contest_id"`
	AnnouncementID uuid.UUID `json:"announcement_id"`
	Version        int64     `json:"version"`
	Audience       string    `json:"audience"`
	Revision       int64     `json:"revision,omitempty"`
	Title          string    `json:"title,omitempty"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type ClarificationChangedV1 struct {
	EventID          uuid.UUID  `json:"event_id"`
	ContestID        uuid.UUID  `json:"contest_id"`
	ClarificationID  uuid.UUID  `json:"clarification_id"`
	RequesterID      uuid.UUID  `json:"requester_id"`
	ContestProblemID *uuid.UUID `json:"contest_problem_id,omitempty"`
	Version          int64      `json:"version"`
	Audience         string     `json:"audience"`
	Shared           bool       `json:"shared,omitempty"`
	OccurredAt       time.Time  `json:"occurred_at"`
}

// ClarificationSubmittedV1 deliberately omits requester identity and all
// authored question bytes. It is suitable only as a staff invalidation.
type ClarificationSubmittedV1 struct {
	EventID          uuid.UUID  `json:"event_id"`
	ContestID        uuid.UUID  `json:"contest_id"`
	ClarificationID  uuid.UUID  `json:"clarification_id"`
	ContestProblemID *uuid.UUID `json:"contest_problem_id,omitempty"`
	Version          int64      `json:"version"`
	OccurredAt       time.Time  `json:"occurred_at"`
}
