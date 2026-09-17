package domain

import (
	"time"

	"github.com/google/uuid"
)

// ProblemDifficulty holds difficulty data loaded via problem edges.
type ProblemDifficulty struct {
	ID          uuid.UUID
	Name        string
	Level       int
	ExpReward   int64
	Description string
	Version     int64
}

// Problem represents a programming problem.
type Problem struct {
	ID              uuid.UUID          `json:"id"`
	Slug            string             `json:"slug"`
	Lifecycle       Lifecycle          `json:"lifecycle"`
	Version         int64              `json:"version"`
	Title           string             `json:"title"`
	Description     string             `json:"description"`
	DifficultyID    uuid.UUID          `json:"difficulty_id"`
	Difficulty      *ProblemDifficulty `json:"-"`
	TimeLimitMs     int                `json:"time_limit_ms"`
	MemoryLimitKb   int                `json:"memory_limit_kb"`
	AuthorID        uuid.UUID          `json:"author_id"`
	IsPublished     bool               `json:"is_published"`
	SubmissionCount int                `json:"submission_count"`
	AcceptedCount   int                `json:"accepted_count"`
	IsSolved        *bool              `json:"is_solved,omitempty"`
	Tags            []Tag              `json:"-"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}
