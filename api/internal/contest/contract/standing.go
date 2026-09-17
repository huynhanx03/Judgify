package contract

import (
	"time"

	"github.com/google/uuid"
)

// StandingResponse represents a user's standing in the leaderboard.
type StandingResponse struct {
	Rank             int
	UserID           uuid.UUID
	Username         string
	SolvedCount      int
	Penalty          int
	ProblemResults   map[string]any
	PendingAttempts  int
	PendingByProblem map[string]int
}

// StandingsResponse makes the visibility boundary explicit. A client never
// has to infer whether totals are official or freeze-safe from contest time.
type StandingsResponse struct {
	ContestID         uuid.UUID
	View              string
	ProjectionVersion int64
	LastCursor        string
	FrozenAt          *time.Time
	Items             []*StandingResponse
}

// SelfStandingResponse exposes only the authenticated caller's official row.
// It is intentionally separate from the public freeze-safe table.
type SelfStandingResponse struct {
	ContestID         uuid.UUID
	ProjectionVersion int64
	LastCursor        string
	Item              *StandingResponse
}
