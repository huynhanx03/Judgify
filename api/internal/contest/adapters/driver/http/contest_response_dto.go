package http

import "time"

type contestRegistrationResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Version int64  `json:"version"`
}
type contestDraftProblemResponse struct {
	ProblemID          string `json:"problem_id"`
	ProblemSlug        string `json:"problem_slug"`
	ProblemTitle       string `json:"problem_title"`
	DisplayOrder       int    `json:"display_order"`
	Alias              string `json:"alias"`
	Points             *int   `json:"points,omitempty"`
	VisibleBeforeStart bool   `json:"visible_before_start"`
}
type contestProblemResponse struct {
	ContestProblemID   string `json:"contest_problem_id"`
	ProblemID          string `json:"problem_id"`
	ProblemSlug        string `json:"problem_slug"`
	ProblemTitle       string `json:"problem_title"`
	DisplayOrder       int    `json:"display_order"`
	Alias              string `json:"alias"`
	Points             *int   `json:"points,omitempty"`
	VisibleBeforeStart bool   `json:"visible_before_start"`
}
type contestResponse struct {
	ID                         string                        `json:"id"`
	Version                    int64                         `json:"version"`
	Title                      string                        `json:"title"`
	Description                string                        `json:"description"`
	StartTime                  time.Time                     `json:"start_time"`
	EndTime                    time.Time                     `json:"end_time"`
	Status                     string                        `json:"status"`
	AuthorID                   string                        `json:"author_id"`
	RegistrationMode           string                        `json:"registration_mode"`
	RegistrationOpensAt        *time.Time                    `json:"registration_opens_at,omitempty"`
	RegistrationClosesAt       *time.Time                    `json:"registration_closes_at,omitempty"`
	MaxParticipants            *int                          `json:"max_participants,omitempty"`
	ParticipantCount           int                           `json:"participant_count"`
	ProblemCount               int                           `json:"problem_count"`
	ScoringPolicy              string                        `json:"scoring_policy"`
	ScoringPolicyVersion       int64                         `json:"scoring_policy_version"`
	StandingsProjectionVersion int64                         `json:"standings_projection_version"`
	FreezeStandingsAt          *time.Time                    `json:"freeze_standings_at,omitempty"`
	Rated                      bool                          `json:"rated"`
	RatingAlgorithmVersion     string                        `json:"rating_algorithm_version"`
	DraftProblems              []contestDraftProblemResponse `json:"draft_problems,omitempty"`
	Problems                   []contestProblemResponse      `json:"problems,omitempty"`
	Registration               *contestRegistrationResponse  `json:"registration,omitempty"`
	IsRegistered               *bool                         `json:"is_registered,omitempty"`
	PublishedAt                *time.Time                    `json:"published_at,omitempty"`
	StartedAt                  *time.Time                    `json:"started_at,omitempty"`
	EndedAt                    *time.Time                    `json:"ended_at,omitempty"`
	CancelledAt                *time.Time                    `json:"cancelled_at,omitempty"`
	RatingCompletedAt          *time.Time                    `json:"rating_completed_at,omitempty"`
	CreatedAt                  time.Time                     `json:"created_at"`
	UpdatedAt                  time.Time                     `json:"updated_at"`
}
type contestProblemDifficultyResponse struct {
	ID          string `json:"id"`
	Revision    int64  `json:"revision"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	Description string `json:"description,omitempty"`
}
type contestProblemTagResponse struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	Name     string `json:"name"`
}
type contestProblemSampleResponse struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	OrderIndex     int    `json:"order_index"`
}
type contestProblemDetailResponse struct {
	ContestID          string                           `json:"contest_id"`
	ContestProblemID   string                           `json:"contest_problem_id"`
	ProblemID          string                           `json:"problem_id"`
	ProblemSlug        string                           `json:"problem_slug"`
	RevisionNumber     int64                            `json:"revision_number"`
	Title              string                           `json:"title"`
	StatementMarkdown  string                           `json:"statement_markdown"`
	Difficulty         contestProblemDifficultyResponse `json:"difficulty"`
	Tags               []contestProblemTagResponse      `json:"tags"`
	CPUTimeMs          int                              `json:"cpu_time_ms"`
	WallTimeMs         int                              `json:"wall_time_ms"`
	MemoryLimitKb      int                              `json:"memory_limit_kb"`
	OutputLimitBytes   int                              `json:"output_limit_bytes"`
	ProcessLimit       int                              `json:"process_limit"`
	AllowedRuntimeKeys []string                         `json:"allowed_runtime_keys"`
	SubmissionCount    int                              `json:"submission_count"`
	AcceptedCount      int                              `json:"accepted_count"`
	AcceptanceRate     float64                          `json:"acceptance_rate"`
	DisplayOrder       int                              `json:"display_order"`
	Alias              string                           `json:"alias"`
	Points             *int                             `json:"points,omitempty"`
	Samples            []contestProblemSampleResponse   `json:"samples"`
	CreatedAt          time.Time                        `json:"created_at"`
}
