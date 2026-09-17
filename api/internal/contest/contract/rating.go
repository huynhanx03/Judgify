package contract

import (
	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/platform/operation"
)

// RatingChangeResponse represents a user's rating change after a contest.
type RatingChangeResponse struct {
	UserID    uuid.UUID
	Username  string
	OldRating int
	NewRating int
	Rank      int
	Delta     int
}

type RatingReratingStartResponse struct {
	Operation          operation.Projection
	TriggerContestID   uuid.UUID
	FromSequence       int64
	ThroughSequence    int64
	SourceRevision     int64
	TargetGenerationID uuid.UUID
	TargetRunChainID   uuid.UUID
	AlgorithmVersion   string
	IdempotentReplay   bool
}
