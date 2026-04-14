package dto

// StandingResponse represents a user's standing in the leaderboard.
type StandingResponse struct {
	Rank           int                     `json:"rank"`
	UserID         int                     `json:"user_id"`
	Username       string                  `json:"username"`
	SolvedCount    int                     `json:"solved_count"`
	Penalty        int                     `json:"penalty"`
	ProblemResults map[string]any          `json:"problem_results,omitempty"`
}

// GetStandingsRequest represents request to get contest standings.
type GetStandingsRequest struct {
	ContestID int `uri:"id" validate:"required"`
}
