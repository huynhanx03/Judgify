package usecase

import (
	"errors"
	"sort"

	"github.com/google/uuid"
)

const (
	// InitialRating is the canonical virtual value for a user that has never
	// appeared in an applied rating run.
	InitialRating = 1200
	// RatingAlgorithmVersion pins every arithmetic and ordering choice below.
	// Any behavior change requires a new version and a shadow rerating.
	RatingAlgorithmVersion = "expected-rank-v1"
)

var ErrInvalidRatingInput = errors.New("rating algorithm: invalid input")

// RatingInput is a complete, immutable participant snapshot. SolvedCount and
// PenaltySeconds are the official ICPC result used to derive actual rank.
type RatingInput struct {
	UserID                uuid.UUID
	SolvedCount           int
	PenaltySeconds        int
	PreviousRating        int
	PreviousRatedContests int
}

// RatingDelta is the deterministic output persisted in an immutable run.
type RatingDelta struct {
	UserID                uuid.UUID
	Rank                  int
	ActualRank            float64
	ExpectedRank          float64
	PreviousRating        int
	PreviousRatedContests int
	Delta                 int
	NewRating             int
}

// CalculateRatingDeltas implements expected-rank-v1. It owns sorting so a
// database planner or caller cannot influence pairwise arithmetic order.
func CalculateRatingDeltas(inputs []RatingInput) ([]RatingDelta, error) {
	participants := make([]ratingParticipant, len(inputs))
	seen := make(map[uuid.UUID]struct{}, len(inputs))
	for index, input := range inputs {
		if input.UserID == uuid.Nil || input.SolvedCount < 0 ||
			input.PenaltySeconds < 0 || input.PreviousRating < 0 ||
			input.PreviousRatedContests < 0 {
			return nil, ErrInvalidRatingInput
		}
		if _, duplicate := seen[input.UserID]; duplicate {
			return nil, ErrInvalidRatingInput
		}
		seen[input.UserID] = struct{}{}
		participants[index] = ratingParticipant{
			userID:        input.UserID,
			currentRating: input.PreviousRating,
			solvedCount:   input.SolvedCount,
			penalty:       input.PenaltySeconds,
			contestCount:  input.PreviousRatedContests,
		}
	}
	sort.Slice(participants, func(left, right int) bool {
		if participants[left].solvedCount != participants[right].solvedCount {
			return participants[left].solvedCount >
				participants[right].solvedCount
		}
		if participants[left].penalty != participants[right].penalty {
			return participants[left].penalty < participants[right].penalty
		}
		return participants[left].userID.String() <
			participants[right].userID.String()
	})
	assignActualRanks(participants)
	expectedRanks := calculateExpectedRanks(participants)
	output := make([]RatingDelta, len(participants))
	for index, participant := range participants {
		k := kFactorEstablished
		if participant.contestCount < minContestsForK20 {
			k = kFactorNew
		}
		delta := ratingDelta(
			expectedRanks[index],
			participant.actualRank,
			k,
			len(participants),
		)
		next := participant.currentRating + delta
		if next < 0 {
			next = 0
		}
		output[index] = RatingDelta{
			UserID:                participant.userID,
			Rank:                  participant.rank,
			ActualRank:            participant.actualRank,
			ExpectedRank:          expectedRanks[index],
			PreviousRating:        participant.currentRating,
			PreviousRatedContests: participant.contestCount,
			Delta:                 delta,
			NewRating:             next,
		}
	}
	return output, nil
}
