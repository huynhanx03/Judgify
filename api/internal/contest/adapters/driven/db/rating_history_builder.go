package db

import (
	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateRatingHistory builds the create mutation for RatingHistory.
func BuildCreateRatingHistory(client *generate.Client, e *entity.RatingHistory) *generate.RatingHistoryCreate {
	create := client.RatingHistory.Create().
		SetUserID(e.UserID).
		SetContestID(e.ContestID).
		SetOldRating(e.OldRating).
		SetNewRating(e.NewRating).
		SetRankPosition(e.RankPosition).
		SetPriorRatedContestCount(e.PriorRatedContestCount).
		SetDelta(e.Delta)
	if e.RatingRunID != uuid.Nil {
		create.SetRatingRunID(e.RatingRunID)
	}
	if e.RatingSequence > 0 {
		create.SetRatingSequence(e.RatingSequence)
	}
	if e.RunVersion > 0 {
		create.SetRunVersion(e.RunVersion)
	}
	if e.ActualRank > 0 {
		create.SetActualRank(e.ActualRank)
	}
	if e.ExpectedRank > 0 {
		create.SetExpectedRank(e.ExpectedRank)
	}
	if e.AlgorithmVersion != "" {
		create.SetAlgorithmVersion(e.AlgorithmVersion)
	}
	return create
}
