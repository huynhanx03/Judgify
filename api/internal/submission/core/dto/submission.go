package dto

// CreateSubmissionRequest represents request to submit code for judging.
type CreateSubmissionRequest struct {
	ProblemID  int    `json:"problem_id" validate:"required,min=1"`
	Language   string `json:"language" validate:"required,oneof=cpp python java go"`
	SourceCode string `json:"source_code" validate:"required,max=262144"`
}

// GetSubmissionRequest represents request to get a submission by ID.
type GetSubmissionRequest struct {
	ID int `uri:"id" validate:"required"`
}

// ListByProblemRequest represents request to list submissions by problem.
type ListByProblemRequest struct {
	ProblemID int `uri:"id" validate:"required"`
}

// SubmissionResponse represents submission data in API response.
type SubmissionResponse struct {
	ID           int     `json:"id"`
	ProblemID    int     `json:"problem_id"`
	UserID       int     `json:"user_id"`
	Language     string  `json:"language"`
	Status       string  `json:"status"`
	PassedCount  int     `json:"passed_count"`
	TotalCount   int     `json:"total_count"`
	TimeMs       *int    `json:"time_ms,omitempty"`
	MemoryKb     *int    `json:"memory_kb,omitempty"`
	ErrorMessage *string `json:"error_message,omitempty"`
}
