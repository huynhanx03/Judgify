package contract

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	EventSubmissionJudgedV1          = "submission.judged.v1"
	EventProblemSolveStatusChangedV1 = "problem.solve_status_changed.v1"
	AggregateUserProblemSolve        = "user_problem_solve"
)

// ActiveJudgementV1 is a browser-safe pointer to canonical contestant state.
// It contains no source, diagnostics, testcase data, or execution internals.
type ActiveJudgementV1 struct {
	JudgementID uuid.UUID `json:"judgement_id"`
	Generation  int64     `json:"generation"`
	Verdict     string    `json:"verdict"`
}

// ContestSubmissionSnapshotV1 contains only immutable ordering facts captured
// at intake. All fields are present together for a contest submission.
type ContestSubmissionSnapshotV1 struct {
	ContestID            uuid.UUID `json:"contest_id"`
	ContestProblemID     uuid.UUID `json:"contest_problem_id"`
	ScoringPolicyVersion int64     `json:"scoring_policy_version"`
	ElapsedMilliseconds  int64     `json:"elapsed_ms"`
	SubmissionSequence   int64     `json:"submission_sequence"`
}

// SubmissionJudgedV1 describes one committed terminal generation and the
// before/after active contestant pointer. It deliberately excludes worker,
// lease, source, hidden-test, and raw diagnostic data.
type SubmissionJudgedV1 struct {
	EventID           uuid.UUID                    `json:"event_id"`
	SubmissionID      uuid.UUID                    `json:"submission_id"`
	UserID            uuid.UUID                    `json:"user_id"`
	ProblemID         uuid.UUID                    `json:"problem_id"`
	Generation        int64                        `json:"generation"`
	JudgementID       uuid.UUID                    `json:"judgement_id"`
	ProblemRevisionID uuid.UUID                    `json:"problem_revision_id"`
	TestsetRevisionID uuid.UUID                    `json:"testset_revision_id"`
	AggregateVersion  int64                        `json:"aggregate_version"`
	LatestPhase       string                       `json:"latest_phase"`
	LatestOutcome     string                       `json:"latest_outcome"`
	Verdict           string                       `json:"verdict"`
	PassedCount       int                          `json:"passed_count"`
	TotalCount        int                          `json:"total_count"`
	MaximumTimeMS     *int                         `json:"maximum_time_ms"`
	MaximumMemoryKB   *int                         `json:"maximum_memory_kb"`
	BecameActive      bool                         `json:"became_active"`
	PreviousActive    *ActiveJudgementV1           `json:"previous_active"`
	ResultingActive   *ActiveJudgementV1           `json:"resulting_active"`
	Contest           *ContestSubmissionSnapshotV1 `json:"contest"`
	SubmittedAt       time.Time                    `json:"submitted_at"`
	OccurredAt        time.Time                    `json:"occurred_at"`
	CorrelationID     string                       `json:"correlation_id"`
}

// SolveStateV1 is one side of a contiguous canonical solve transition.
// RewardFacts is the immutable Problem-owned snapshot, never a live lookup.
type SolveStateV1 struct {
	Version             int64           `json:"version"`
	Status              string          `json:"status"`
	SubmissionID        *uuid.UUID      `json:"submission_id"`
	JudgementID         *uuid.UUID      `json:"judgement_id"`
	Generation          *int64          `json:"generation"`
	ProblemRevisionID   *uuid.UUID      `json:"problem_revision_id"`
	RewardGrantSourceID *uuid.UUID      `json:"reward_grant_source_id"`
	RewardProvenance    *string         `json:"reward_provenance"`
	RewardFacts         json.RawMessage `json:"reward_facts"`
	RewardFactsChecksum *string         `json:"reward_facts_checksum"`
	SolvedAt            *time.Time      `json:"solved_at"`
}

// ProblemSolveStatusChangedV1 is the exact immutable input for cultivation
// and other contiguous consumers. Its outbox aggregate version equals To.Version.
type ProblemSolveStatusChangedV1 struct {
	EventID       uuid.UUID    `json:"event_id"`
	TransitionID  uuid.UUID    `json:"transition_id"`
	SolveID       uuid.UUID    `json:"solve_id"`
	UserID        uuid.UUID    `json:"user_id"`
	ProblemID     uuid.UUID    `json:"problem_id"`
	From          SolveStateV1 `json:"from"`
	To            SolveStateV1 `json:"to"`
	Provenance    string       `json:"provenance"`
	OccurredAt    time.Time    `json:"occurred_at"`
	CorrelationID string       `json:"correlation_id"`
}
