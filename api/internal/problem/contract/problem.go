package contract

import (
	"time"

	"github.com/google/uuid"
)

// CreateProblemCommand is the application input for creating a problem.
type CreateProblemCommand struct {
	Slug          string
	Title         string
	Description   string
	DifficultyID  uuid.UUID
	TimeLimitMs   *int
	MemoryLimitKb *int
	TagIDs        []uuid.UUID
}

// ProblemResult is the application projection of a problem.
type ProblemResult struct {
	ID              uuid.UUID
	Slug            string
	Lifecycle       string
	Version         int64
	Title           string
	Description     string
	DifficultyID    uuid.UUID
	Difficulty      *DifficultyResult
	TimeLimitMs     int
	MemoryLimitKb   int
	AuthorID        uuid.UUID
	IsPublished     bool
	SubmissionCount int
	AcceptedCount   int
	AcceptanceRate  float64
	IsSolved        *bool
	Tags            []*TagResult
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
