package db

import (
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToRatingHistoryEntity converts Ent RatingHistory model to domain entity.
func ToRatingHistoryEntity(m *generate.RatingHistory) *entity.RatingHistory {
	if m == nil {
		return nil
	}
	username := ""
	if u, err := m.Edges.UserOrErr(); err == nil && u != nil {
		username = u.Username
	}

	result := &entity.RatingHistory{
		ID:                     m.ID,
		UserID:                 m.UserID,
		Username:               username,
		ContestID:              m.ContestID,
		OldRating:              m.OldRating,
		NewRating:              m.NewRating,
		RankPosition:           m.RankPosition,
		PriorRatedContestCount: m.PriorRatedContestCount,
		Delta:                  m.Delta,
	}
	if m.RatingRunID != nil {
		result.RatingRunID = *m.RatingRunID
	}
	if m.RatingSequence != nil {
		result.RatingSequence = *m.RatingSequence
	}
	if m.RunVersion != nil {
		result.RunVersion = *m.RunVersion
	}
	if m.ActualRank != nil {
		result.ActualRank = *m.ActualRank
	}
	if m.ExpectedRank != nil {
		result.ExpectedRank = *m.ExpectedRank
	}
	if m.AlgorithmVersion != nil {
		result.AlgorithmVersion = *m.AlgorithmVersion
	}
	return result
}
