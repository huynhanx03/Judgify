package http

import "time"

// CreateProblemRequest is the HTTP payload for creating a problem.
type CreateProblemRequest struct {
	Slug          string   `json:"slug" validate:"required,max=128"`
	Title         string   `json:"title" validate:"required,min=1,max=300"`
	Description   string   `json:"description" validate:"required"`
	DifficultyID  string   `json:"difficulty_id" validate:"required,uuid"`
	TimeLimitMs   *int     `json:"time_limit_ms" validate:"omitempty,min=100,max=30000"`
	MemoryLimitKb *int     `json:"memory_limit_kb" validate:"omitempty,min=1024,max=1048576"`
	TagIDs        []string `json:"tag_ids"`
}

type problemIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

// ProblemResponse is the HTTP representation of a problem.
type ProblemResponse struct {
	ID              string              `json:"id"`
	Slug            string              `json:"slug"`
	Lifecycle       string              `json:"lifecycle"`
	Version         int64               `json:"version"`
	Title           string              `json:"title"`
	Description     string              `json:"description"`
	DifficultyID    string              `json:"difficulty_id"`
	Difficulty      *DifficultyResponse `json:"difficulty,omitempty"`
	TimeLimitMs     int                 `json:"time_limit_ms"`
	MemoryLimitKb   int                 `json:"memory_limit_kb"`
	AuthorID        string              `json:"author_id"`
	IsPublished     bool                `json:"is_published"`
	SubmissionCount int                 `json:"submission_count"`
	AcceptedCount   int                 `json:"accepted_count"`
	AcceptanceRate  float64             `json:"acceptance_rate"`
	IsSolved        *bool               `json:"is_solved,omitempty"`
	Tags            []*TagResponse      `json:"tags,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}
