package entity

import "time"

// Problem represents a programming problem.
type Problem struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	DifficultyID  int       `json:"difficulty_id"`
	TimeLimitMs   int       `json:"time_limit_ms"`
	MemoryLimitKb int       `json:"memory_limit_kb"`
	AuthorID      int       `json:"author_id"`
	IsPublished   bool      `json:"is_published"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
