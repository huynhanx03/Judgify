package contract

import "github.com/google/uuid"

// CreateRankCommand contains the data needed to create a rank.
type CreateRankCommand struct {
	Name        string
	MinRating   int
	Description string
}

// UpdateRankCommand contains mutable rank data.
type UpdateRankCommand struct {
	Name        *string
	MinRating   *int
	Description *string
}

// RankResponse represents a rank result.
type RankResponse struct {
	ID          uuid.UUID
	Name        string
	MinRating   int
	Description string
}
