package domain

import "github.com/google/uuid"

// ContestStanding represents a user's standing in a contest.
type ContestStanding struct {
	ID                        uuid.UUID
	ContestID                 uuid.UUID
	UserID                    uuid.UUID
	Username                  string
	SolvedCount               int
	Penalty                   int
	ProblemResults            map[string]any
	LastAcceptedElapsedMS     *int64
	Version                   int64
	FrozenSolvedCount         int
	FrozenPenalty             int
	FrozenProblemResults      map[string]any
	FrozenPendingByProblem    map[string]int
	FrozenPendingAttemptCount int
	FrozenLastAcceptedMS      *int64
	FrozenVersion             int64
	VisiblePendingAttempts    int
	VisiblePendingByProblem   map[string]int
}
