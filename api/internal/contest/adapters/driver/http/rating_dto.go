package http

import "github.com/huynhanx03/judgify/internal/platform/operation"

type contestIDRequest struct {
	ContestID string `uri:"id" validate:"required,uuid"`
}
type startRatingReratingRequest struct {
	Reason string `json:"reason" validate:"required,min=3,max=1024"`
}
type ratingChangeResponse struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	OldRating int    `json:"old_rating"`
	NewRating int    `json:"new_rating"`
	Rank      int    `json:"rank"`
	Delta     int    `json:"delta"`
}

type ratingReratingStartResponse struct {
	Operation          operation.Projection `json:"operation"`
	TriggerContestID   string               `json:"trigger_contest_id"`
	FromSequence       int64                `json:"from_sequence"`
	ThroughSequence    int64                `json:"through_sequence"`
	SourceRevision     int64                `json:"source_revision"`
	TargetGenerationID string               `json:"target_generation_id"`
	TargetRunChainID   string               `json:"target_run_chain_id"`
	AlgorithmVersion   string               `json:"algorithm_version"`
	IdempotentReplay   bool                 `json:"idempotent_replay"`
}
