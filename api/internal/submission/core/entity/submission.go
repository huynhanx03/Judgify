package entity

import "time"

// Submission represents a code submission for a problem.
type Submission struct {
	ID           int        `json:"id"`
	ProblemID    int        `json:"problem_id"`
	UserID       int        `json:"user_id"`
	ContestID    *int       `json:"contest_id,omitempty"`
	Language     string     `json:"language"`
	SourceCode   string     `json:"source_code"`
	Status       string     `json:"status"`
	PassedCount  int        `json:"passed_count"`
	TotalCount   int        `json:"total_count"`
	TimeMs       *int       `json:"time_ms"`
	MemoryKb     *int       `json:"memory_kb"`
	ErrorMessage *string    `json:"error_message"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
