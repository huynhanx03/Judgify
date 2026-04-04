package entity

import "time"

// ProblemDifficulty holds difficulty data loaded via problem edges.
type ProblemDifficulty struct {
	ID          int
	Name        string
	Level       int
	ExpReward   int64
	Description string
}

// Problem represents a programming problem.
type Problem struct {
	ID            int                `json:"id"`
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	DifficultyID  int                `json:"difficulty_id"`
	Difficulty    *ProblemDifficulty `json:"-"`
	TimeLimitMs   int                `json:"time_limit_ms"`
	MemoryLimitKb int                `json:"memory_limit_kb"`
	AuthorID      int                `json:"author_id"`
	IsPublished     bool               `json:"is_published"`
	SubmissionCount int                `json:"submission_count"`
	AcceptedCount   int                `json:"accepted_count"`
	IsSolved        *bool              `json:"is_solved,omitempty"`
	Tags            []Tag              `json:"-"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}
