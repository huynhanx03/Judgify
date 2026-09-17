package domain

import (
	"time"

	"github.com/google/uuid"
)

// RatingHistory represents a rating change record after contest ends.
type RatingHistory struct {
	ID                     uuid.UUID
	UserID                 uuid.UUID
	Username               string
	ContestID              uuid.UUID
	OldRating              int
	NewRating              int
	RankPosition           int
	RatingRunID            uuid.UUID
	RatingSequence         int64
	RunVersion             int64
	PriorRatedContestCount int
	ActualRank             float64
	ExpectedRank           float64
	Delta                  int
	AlgorithmVersion       string
}

// RatingParticipantSnapshot is the exact official result and canonical prior
// projection used as input to one immutable rating run.
type RatingParticipantSnapshot struct {
	UserID                uuid.UUID
	Username              string
	SolvedCount           int
	PenaltySeconds        int
	PreviousRating        int
	PreviousRatedContests int
}

// RatingSettlementSnapshot is returned only while the caller-owned
// transaction holds RatingState FOR UPDATE. It must not escape that callback.
type RatingSettlementSnapshot struct {
	ContestID                  uuid.UUID
	RatingSequence             int64
	RatingSequenceSlotID       uuid.UUID
	RunVersion                 int64
	StandingsProjectionVersion int64
	AlgorithmVersion           string
	ProjectionRevision         int64
	ProjectionGenerationID     uuid.UUID
	RunChainID                 uuid.UUID
	ProjectionChecksum         []byte
	RunChainChecksum           []byte
	Participants               []RatingParticipantSnapshot
	IdempotentRun              *RatingRun
}

// RatingRun is the canonical immutable run selected by the active chain.
type RatingRun struct {
	ID                         uuid.UUID
	ContestID                  uuid.UUID
	RatingSequence             int64
	RatingSequenceSlotID       uuid.UUID
	RunVersion                 int64
	StandingsProjectionVersion int64
	AlgorithmVersion           string
	Status                     string
	ParticipantCount           int
	InputChecksum              []byte
	CreatedAt                  time.Time
	AppliedAt                  *time.Time
	Results                    []RatingRunResult
}

// RatingRunResult is one immutable participant output.
type RatingRunResult struct {
	UserID                uuid.UUID
	Username              string
	PreviousRating        int
	PreviousRatedContests int
	Rank                  int
	ActualRank            float64
	ExpectedRank          float64
	Delta                 int
	NewRating             int
}

// ApplyRatingRun is a guarded write against the snapshot returned by Prepare.
type ApplyRatingRun struct {
	Snapshot      RatingSettlementSnapshot
	RunID         uuid.UUID
	InputChecksum []byte
	Results       []RatingRunResult
	AppliedAt     time.Time
}

// RatingReratingPlan is the immutable fence captured when an authorized
// operator requests a historical rebuild. The source pointers stay public
// until every sequence has been replayed into the target generation and the
// final activation transaction verifies this exact tuple again.
type RatingReratingPlan struct {
	OperationID                  uuid.UUID
	TriggerContestID             uuid.UUID
	ActorUserID                  uuid.UUID
	Reason                       string
	FromSequence                 int64
	ThroughSequence              int64
	SourceProjectionRevision     int64
	SourceProjectionGenerationID uuid.UUID
	SourceRunChainID             uuid.UUID
	SourceProjectionChecksum     []byte
	SourceRunChainChecksum       []byte
	TargetProjectionGenerationID uuid.UUID
	TargetRunChainID             uuid.UUID
	AlgorithmVersion             string
	InitialProjectionChecksum    []byte
	InitialRunChainChecksum      []byte
}

// RatingReratingSequenceSnapshot is the complete mutable-input boundary for a
// single recomputed suffix sequence. It is loaded and consumed in the same
// operation batch transaction.
type RatingReratingSequenceSnapshot struct {
	ContestID                  uuid.UUID
	RatingSequence             int64
	RatingSequenceSlotID       uuid.UUID
	RunVersion                 int64
	StandingsProjectionVersion int64
	AlgorithmVersion           string
	Participants               []RatingParticipantSnapshot
}

// ApplyRatingReratingSequence appends one immutable shadow run without
// changing active pointers, compatibility history, or user-facing profiles.
type ApplyRatingReratingSequence struct {
	Plan                       RatingReratingPlan
	Snapshot                   RatingReratingSequenceSnapshot
	RunID                      uuid.UUID
	InputChecksum              []byte
	Results                    []RatingRunResult
	PreviousProjectionChecksum []byte
	PreviousRunChainChecksum   []byte
	AppliedAt                  time.Time
}

// RatingReratingStepResult is small enough to persist in the generic
// operation checkpoint and is sufficient to resume after a process crash.
type RatingReratingStepResult struct {
	RatingRunID        uuid.UUID
	ProjectionChecksum []byte
	RunChainChecksum   []byte
}

// ActivateRatingRerating is the final compare-and-swap command. Activation
// also reconciles the compatibility user_stats mirror in the same transaction.
type ActivateRatingRerating struct {
	Plan               RatingReratingPlan
	ProjectionChecksum []byte
	RunChainChecksum   []byte
	ActivatedAt        time.Time
}
