package contract

import "github.com/google/uuid"

// CreateDifficultyCommand is the transport-neutral input for creating a difficulty.
type CreateDifficultyCommand struct {
	Name        string
	Level       int
	ExpReward   int64
	Description string
}

// UpdateDifficultyCommand is the transport-neutral input for updating a difficulty.
type UpdateDifficultyCommand struct {
	Name        *string
	Level       *int
	ExpReward   *int64
	Description *string
}

// DifficultyResult is the application projection returned by a difficulty use case.
type DifficultyResult struct {
	ID          uuid.UUID
	Name        string
	Level       int
	ExpReward   int64
	Description string
	Version     int64
}
