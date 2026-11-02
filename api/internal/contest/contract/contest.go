package contract

import (
	"time"

	"github.com/google/uuid"
)

// ContestDraftProblemRequest is one ordered, mutable problem membership.
// Display order is derived from the array position and cannot be forged.
type ContestDraftProblemRequest struct {
	ProblemID          uuid.UUID
	Alias              string
	Points             *int
	VisibleBeforeStart bool
}

// ContestDraftRequest is the complete replacement document for mutable contest
// authoring. PUT uses the same shape so omitted fields cannot silently retain a
// stale policy.
type ContestDraftRequest struct {
	Reason               string
	Title                string
	Description          string
	StartTime            time.Time
	EndTime              time.Time
	RegistrationMode     string
	RegistrationOpensAt  *time.Time
	RegistrationClosesAt *time.Time
	MaxParticipants      *int
	FreezeStandingsAt    *time.Time
	Rated                *bool
	Problems             []ContestDraftProblemRequest
}

// CreateContestRequest creates version one of a mutable contest draft.
type CreateContestRequest struct {
	ContestDraftRequest
}

// UpdateContestRequest replaces a draft under an optimistic version fence.
type UpdateContestRequest struct {
	ID              uuid.UUID
	ExpectedVersion int64
	ContestDraftRequest
}

// GetContestRequest represents request to get a contest by ID.
type GetContestRequest struct {
	ID uuid.UUID
}

// GetContestProblemRequest identifies one exact frozen contest membership.
// The stable Problem ID is returned by the server and is never trusted from
// the browser URL.
type GetContestProblemRequest struct {
	ContestID        uuid.UUID
	ContestProblemID uuid.UUID
}

// DeleteContestRequest represents request to delete a contest by ID.
type DeleteContestRequest struct {
	ID     uuid.UUID
	Reason string
}

// PublishContestRequest identifies the draft contest to publish.
type PublishContestRequest struct {
	ID uuid.UUID
}

// PublishContestCommandRequest carries only operator-owned audit context.
// Optimistic version and idempotency identities are strict HTTP headers.
type PublishContestCommandRequest struct {
	Reason string
}

// ContestResponse is the authoritative browser-safe contest projection.
type ContestResponse struct {
	ID                         uuid.UUID
	Version                    int64
	Title                      string
	Description                string
	StartTime                  time.Time
	EndTime                    time.Time
	Status                     string
	AuthorID                   uuid.UUID
	RegistrationMode           string
	RegistrationOpensAt        *time.Time
	RegistrationClosesAt       *time.Time
	MaxParticipants            *int
	ParticipantCount           int
	ProblemCount               int
	ScoringPolicy              string
	ScoringPolicyVersion       int64
	StandingsProjectionVersion int64
	FreezeStandingsAt          *time.Time
	Rated                      bool
	RatingAlgorithmVersion     string
	DraftProblems              []ContestDraftProblemResponse
	Problems                   []ContestProblemResponse
	Registration               *ContestRegistrationResponse
	IsRegistered               *bool
	PublishedAt                *time.Time
	StartedAt                  *time.Time
	EndedAt                    *time.Time
	CancelledAt                *time.Time
	RatingCompletedAt          *time.Time
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

// ContestRegistrationResponse is the caller-owned optimistic registration
// projection. Private audit reasons and submission counts are not exposed.
type ContestRegistrationResponse struct {
	ID      uuid.UUID
	Status  string
	Version int64
}

// ContestDraftProblemResponse is returned only from authorized draft reads.
type ContestDraftProblemResponse struct {
	ProblemID          uuid.UUID
	ProblemSlug        string
	ProblemTitle       string
	DisplayOrder       int
	Alias              string
	Points             *int
	VisibleBeforeStart bool
}

// ContestProblemResponse gives the browser the exact frozen membership
// identity needed for official submission without exposing judge internals.
type ContestProblemResponse struct {
	ContestProblemID   uuid.UUID
	ProblemID          uuid.UUID
	ProblemSlug        string
	ProblemTitle       string
	DisplayOrder       int
	Alias              string
	Points             *int
	VisibleBeforeStart bool
}

// ContestProblemDetailResponse is the safe participant workspace projection
// of the exact problem revision frozen at contest publication.
type ContestProblemDetailResponse struct {
	ContestID          uuid.UUID
	ContestProblemID   uuid.UUID
	ProblemID          uuid.UUID
	ProblemSlug        string
	RevisionNumber     int64
	Title              string
	StatementMarkdown  string
	Difficulty         ContestProblemDifficultyResult
	Tags               []ContestProblemTagResponse
	CPUTimeMs          int
	WallTimeMs         int
	MemoryLimitKb      int
	OutputLimitBytes   int
	ProcessLimit       int
	AllowedRuntimeKeys []string
	SubmissionCount    int
	AcceptedCount      int
	AcceptanceRate     float64
	DisplayOrder       int
	Alias              string
	Points             *int
	Samples            []ContestProblemSampleResponse
	CreatedAt          time.Time
}

type ContestProblemDifficultyResult struct {
	ID          uuid.UUID
	Revision    int64
	Name        string
	Level       int
	Description string
}

type ContestProblemTagResponse struct {
	ID       uuid.UUID
	Revision int64
	Name     string
}

type ContestProblemSampleResponse struct {
	Input          string
	ExpectedOutput string
	OrderIndex     int
}

type ContestLifecycleResponse struct {
	ContestID        uuid.UUID
	From             string
	To               string
	Version          int64
	OccurredAt       time.Time
	CommandID        uuid.UUID
	IdempotentReplay bool
	CID              string
}
