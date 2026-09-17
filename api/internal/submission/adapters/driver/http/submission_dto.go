package http

import "time"

type submissionResponse struct {
	ID           string    `json:"id"`
	ProblemID    string    `json:"problem_id"`
	UserID       string    `json:"user_id"`
	ContestID    *string   `json:"contest_id,omitempty"`
	Language     string    `json:"language"`
	SourceCode   string    `json:"source_code"`
	Status       string    `json:"status"`
	PassedCount  int       `json:"passed_count"`
	TotalCount   int       `json:"total_count"`
	TimeMs       *int      `json:"time_ms,omitempty"`
	MemoryKb     *int      `json:"memory_kb,omitempty"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type submissionSummaryResponse struct {
	ID          string    `json:"id"`
	ProblemID   string    `json:"problem_id"`
	UserID      string    `json:"user_id"`
	ContestID   *string   `json:"contest_id,omitempty"`
	Language    string    `json:"language"`
	Status      string    `json:"status"`
	PassedCount int       `json:"passed_count"`
	TotalCount  int       `json:"total_count"`
	TimeMs      *int      `json:"time_ms,omitempty"`
	MemoryKb    *int      `json:"memory_kb,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type mySubmissionsPageResponse struct {
	Items      []*submissionSummaryResponse `json:"items"`
	NextCursor *string                      `json:"next_cursor"`
	HasMore    bool                         `json:"has_more"`
}

type adminSubmissionSummaryResponse struct {
	ID                        string     `json:"id"`
	ProblemID                 string     `json:"problem_id"`
	UserID                    string     `json:"user_id"`
	ContestID                 *string    `json:"contest_id,omitempty"`
	Language                  string     `json:"language"`
	Status                    string     `json:"status"`
	PassedCount               int        `json:"passed_count"`
	TotalCount                int        `json:"total_count"`
	TimeMs                    *int       `json:"time_ms,omitempty"`
	MemoryKb                  *int       `json:"memory_kb,omitempty"`
	LatestGenerationStartedAt *time.Time `json:"latest_generation_started_at,omitempty"`
	JudgedAt                  *time.Time `json:"judged_at,omitempty"`
	VerdictVersion            int64      `json:"verdict_version"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type adminSubmissionDetailResponse struct {
	*adminSubmissionSummaryResponse
	SourceCode              string  `json:"source_code"`
	ErrorMessage            *string `json:"error_message,omitempty"`
	AggregateVersion        int64   `json:"aggregate_version"`
	LatestGeneration        int64   `json:"latest_generation"`
	LatestGenerationPhase   string  `json:"latest_generation_phase"`
	LatestGenerationOutcome *string `json:"latest_generation_outcome,omitempty"`
}

type submissionIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

type createSubmissionRequest struct {
	ProblemID        string  `json:"problem_id" validate:"required,uuid"`
	RuntimeKey       string  `json:"runtime_key" validate:"required"`
	SourceCode       string  `json:"source_code" validate:"required"`
	ContestID        *string `json:"contest_id,omitempty" validate:"omitempty,uuid"`
	ContestProblemID *string `json:"contest_problem_id,omitempty" validate:"omitempty,uuid"`
}

type problemSubmissionsRequest struct {
	ProblemID string `uri:"id" validate:"required,uuid"`
}

type rejudgeSubmissionRequest struct {
	ExpectedVersion int64  `json:"expected_version" validate:"required,min=1"`
	Reason          string `json:"reason" validate:"required"`
}

type cancelSubmissionRequest struct {
	Generation      int64  `json:"generation" validate:"required,min=1"`
	ExpectedVersion int64  `json:"expected_version" validate:"required,min=1"`
	Reason          string `json:"reason" validate:"required"`
}

type judgeOperationResponse struct {
	SubmissionID     string  `json:"submission_id"`
	Generation       int64   `json:"generation"`
	AggregateVersion int64   `json:"aggregate_version"`
	Phase            string  `json:"phase"`
	Outcome          string  `json:"outcome,omitempty"`
	JobID            *string `json:"job_id,omitempty"`
	IdempotentReplay bool    `json:"idempotent_replay"`
}
type submissionAcceptedResponse struct {
	ID                        string    `json:"id"`
	AggregateVersion          int64     `json:"aggregate_version"`
	LatestGeneration          int64     `json:"latest_generation"`
	LatestGenerationPhase     string    `json:"latest_generation_phase"`
	LatestGenerationOutcome   *string   `json:"latest_generation_outcome"`
	ActiveJudgementID         *string   `json:"active_judgement_id"`
	ActiveJudgementGeneration *int64    `json:"active_judgement_generation"`
	ActiveVerdict             *string   `json:"active_verdict"`
	SubmittedAt               time.Time `json:"submitted_at"`
	ContestSubmissionSequence *int64    `json:"contest_submission_sequence,omitempty"`
	CID                       string    `json:"cid"`
	IdempotentReplay          bool      `json:"idempotent_replay"`
}
