package http

import (
	"fmt"
	"github.com/google/uuid"
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
)

func toContestID(request *contestIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ContestID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse contest id: %w", err)
	}
	return id, nil
}
func toRatingChanges(results []*contract.RatingChangeResponse) []*ratingChangeResponse {
	values := make([]*ratingChangeResponse, len(results))
	for i, result := range results {
		values[i] = &ratingChangeResponse{UserID: result.UserID.String(), Username: result.Username, OldRating: result.OldRating, NewRating: result.NewRating, Rank: result.Rank, Delta: result.Delta}
	}
	return values
}

func toRatingReratingStartResponse(response *contract.RatingReratingStartResponse) *ratingReratingStartResponse {
	if response == nil {
		return nil
	}
	return &ratingReratingStartResponse{
		Operation:          response.Operation,
		TriggerContestID:   response.TriggerContestID.String(),
		FromSequence:       response.FromSequence,
		ThroughSequence:    response.ThroughSequence,
		SourceRevision:     response.SourceRevision,
		TargetGenerationID: response.TargetGenerationID.String(),
		TargetRunChainID:   response.TargetRunChainID.String(),
		AlgorithmVersion:   response.AlgorithmVersion,
		IdempotentReplay:   response.IdempotentReplay,
	}
}
