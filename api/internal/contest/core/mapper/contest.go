package mapper

import (
	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
)

// ToContestResponse converts entity to API response.
func ToContestResponse(e *entity.Contest, participantCount int, problemIDs []int) *dto.ContestResponse {
	if e == nil {
		return nil
	}
	return &dto.ContestResponse{
		ID:               e.ID,
		Title:            e.Title,
		Description:      e.Description,
		StartTime:        e.StartTime,
		EndTime:          e.EndTime,
		Status:           e.Status,
		AuthorID:         e.AuthorID,
		MaxParticipants:  e.MaxParticipants,
		ParticipantCount: participantCount,
		ProblemIDs:       problemIDs,
		CreatedAt:        e.CreatedAt,
	}
}
