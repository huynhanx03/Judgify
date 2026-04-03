package mapper

import (
	"github.com/huynhanx03/judgify/internal/submission/core/dto"
	"github.com/huynhanx03/judgify/internal/submission/core/entity"
)

// ToSubmissionResponse converts Submission entity to response DTO.
func ToSubmissionResponse(e *entity.Submission) *dto.SubmissionResponse {
	if e == nil {
		return nil
	}
	return &dto.SubmissionResponse{
		ID:           e.ID,
		ProblemID:    e.ProblemID,
		UserID:       e.UserID,
		Language:     e.Language,
		SourceCode:   e.SourceCode,
		Status:       e.Status,
		PassedCount:  e.PassedCount,
		TotalCount:   e.TotalCount,
		TimeMs:       e.TimeMs,
		MemoryKb:     e.MemoryKb,
		ErrorMessage: e.ErrorMessage,
		CreatedAt:    e.CreatedAt,
	}
}

// ToSubmissionEntityFromCreate converts CreateSubmissionRequest to entity.
func ToSubmissionEntityFromCreate(req *dto.CreateSubmissionRequest, userID int) *entity.Submission {
	return &entity.Submission{
		ProblemID:  req.ProblemID,
		UserID:     userID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		Status:     "pending",
	}
}
