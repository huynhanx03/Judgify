package ports

import (
	"context"

	"github.com/huynhanx03/judgify/internal/submission/core/dto"
	"github.com/huynhanx03/judgify/internal/submission/core/entity"
)

// SubmissionRepository defines the submission data access interface.
type SubmissionRepository interface {
	Get(ctx context.Context, id int) (*entity.Submission, error)
	FindByProblemID(ctx context.Context, problemID int) ([]*entity.Submission, error)
	FindByUserID(ctx context.Context, userID int) ([]*entity.Submission, error)
	FindByUserAndProblem(ctx context.Context, userID, problemID int) ([]*entity.Submission, error)
	Create(ctx context.Context, e *entity.Submission) error
	Update(ctx context.Context, e *entity.Submission) error
}

// SubmissionService defines the submission business logic interface.
type SubmissionService interface {
	Get(ctx context.Context, id int) (*dto.SubmissionResponse, error)
	FindByProblemID(ctx context.Context, problemID int) ([]*dto.SubmissionResponse, error)
	FindByUserAndProblem(ctx context.Context, userID, problemID int) ([]*dto.SubmissionResponse, error)
	Create(ctx context.Context, userID int, req *dto.CreateSubmissionRequest) (*dto.SubmissionResponse, error)
}
