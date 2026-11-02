package contract

import "github.com/google/uuid"

// CreateRarityCommand contains the data needed to create a rarity.
type CreateRarityCommand struct {
	Name        string
	Code        string
	Weight      int
	Description string
}

// UpdateRarityCommand contains mutable rarity data.
type UpdateRarityCommand struct {
	Name        *string
	Code        *string
	Weight      *int
	Description *string
}

// RarityResponse represents a rarity result.
type RarityResponse struct {
	ID          uuid.UUID
	Name        string
	Code        string
	Weight      int
	Description string
}
