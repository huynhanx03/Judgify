package contract

import "github.com/google/uuid"

// CreateLevelCommand contains the data needed to create a cultivation level.
type CreateLevelCommand struct {
	Name        string
	MinExp      int64
	Description string
}

// UpdateLevelCommand contains mutable cultivation level data.
type UpdateLevelCommand struct {
	Name        *string
	MinExp      *int64
	Description *string
}

// LevelResponse represents a cultivation level result.
type LevelResponse struct {
	ID          uuid.UUID
	Name        string
	MinExp      int64
	Description string
}
