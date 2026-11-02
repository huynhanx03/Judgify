package http

import "time"

type standingContestIDRequest struct {
	ContestID string `uri:"id" validate:"required,uuid"`
}

type standingResponse struct {
	Rank             int            `json:"rank"`
	UserID           string         `json:"user_id"`
	Username         string         `json:"username"`
	SolvedCount      int            `json:"solved_count"`
	Penalty          int            `json:"penalty"`
	ProblemResults   map[string]any `json:"problem_results,omitempty"`
	PendingAttempts  int            `json:"pending_attempts"`
	PendingByProblem map[string]int `json:"pending_by_problem,omitempty"`
}

type standingsResponse struct {
	ContestID         string              `json:"contest_id"`
	View              string              `json:"view"`
	ProjectionVersion int64               `json:"projection_version"`
	LastCursor        string              `json:"last_cursor"`
	FrozenAt          *time.Time          `json:"frozen_at,omitempty"`
	Items             []*standingResponse `json:"items"`
}

type selfStandingResponse struct {
	ContestID         string            `json:"contest_id"`
	ProjectionVersion int64             `json:"projection_version"`
	LastCursor        string            `json:"last_cursor"`
	Item              *standingResponse `json:"item,omitempty"`
}
