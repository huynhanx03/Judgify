package dto

import "time"

// CreateProblemRequest represents request to create a problem.
type CreateProblemRequest struct {
	Title         string   `json:"title" validate:"required,min=1,max=300"`
	Description   string   `json:"description" validate:"required"`
	DifficultyID  int      `json:"difficulty_id" validate:"required,min=1"`
	TimeLimitMs   *int     `json:"time_limit_ms" validate:"omitempty,min=100,max=30000"`
	MemoryLimitKb *int     `json:"memory_limit_kb" validate:"omitempty,min=1024,max=1048576"`
	TagIDs        []int    `json:"tag_ids" validate:"omitempty"`
}

// UpdateProblemRequest represents request to update a problem.
type UpdateProblemRequest struct {
	ID            int      `json:"-" uri:"id"`
	Title         *string  `json:"title" validate:"omitempty,min=1,max=300"`
	Description   *string  `json:"description" validate:"omitempty"`
	DifficultyID  *int     `json:"difficulty_id" validate:"omitempty,min=1"`
	TimeLimitMs   *int     `json:"time_limit_ms" validate:"omitempty,min=100,max=30000"`
	MemoryLimitKb *int     `json:"memory_limit_kb" validate:"omitempty,min=1024,max=1048576"`
	IsPublished   *bool    `json:"is_published" validate:"omitempty"`
	TagIDs        *[]int   `json:"tag_ids" validate:"omitempty"`
}

// GetProblemRequest represents request to get a problem by ID.
type GetProblemRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteProblemRequest represents request to delete a problem.
type DeleteProblemRequest struct {
	ID int `uri:"id" validate:"required"`
}

// ProblemResponse represents problem data in API response.
type ProblemResponse struct {
	ID            int            `json:"id"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	DifficultyID  int            `json:"difficulty_id"`
	Difficulty    *DifficultyResponse `json:"difficulty,omitempty"`
	TimeLimitMs   int            `json:"time_limit_ms"`
	MemoryLimitKb int            `json:"memory_limit_kb"`
	AuthorID      int            `json:"author_id"`
	IsPublished   bool           `json:"is_published"`
	Tags          []*TagResponse `json:"tags,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
