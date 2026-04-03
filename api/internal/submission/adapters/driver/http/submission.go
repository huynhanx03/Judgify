package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"

	"github.com/huynhanx03/judgify/internal/submission/core/dto"
	"github.com/huynhanx03/judgify/internal/submission/ports"
)

// SubmissionHandler defines the submission HTTP handler interface.
type SubmissionHandler interface {
	Submit(ctx context.Context, req *dto.CreateSubmissionRequest) (*dto.SubmissionResponse, error)
	Get(ctx context.Context, req *dto.GetSubmissionRequest) (*dto.SubmissionResponse, error)
	FindByProblem(ctx context.Context, req *dto.ProblemSubmissionsRequest) ([]*dto.SubmissionResponse, error)
	FindMySubmissions(ctx context.Context, req *dto.ProblemSubmissionsRequest) ([]*dto.SubmissionResponse, error)
}

type submissionHandler struct {
	handler.BaseHandler
	submissionService ports.SubmissionService
}

// NewSubmissionHandler creates a new SubmissionHandler instance.
func NewSubmissionHandler(submissionService ports.SubmissionService) SubmissionHandler {
	return &submissionHandler{submissionService: submissionService}
}

func (h *submissionHandler) Submit(ctx context.Context, req *dto.CreateSubmissionRequest) (*dto.SubmissionResponse, error) {
	userIDVal := ctx.Value(constraints.ContextKeyUserID)
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, apperr.New(response.CodeUnauthorized, "unauthorized", nil)
	}
	return h.submissionService.Create(ctx, userID, req)
}

func (h *submissionHandler) Get(ctx context.Context, req *dto.GetSubmissionRequest) (*dto.SubmissionResponse, error) {
	return h.submissionService.Get(ctx, req.ID)
}

func (h *submissionHandler) FindByProblem(ctx context.Context, req *dto.ProblemSubmissionsRequest) ([]*dto.SubmissionResponse, error) {
	return h.submissionService.FindByProblemID(ctx, req.ProblemID)
}

func (h *submissionHandler) FindMySubmissions(ctx context.Context, req *dto.ProblemSubmissionsRequest) ([]*dto.SubmissionResponse, error) {
	userIDVal := ctx.Value(constraints.ContextKeyUserID)
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, apperr.New(response.CodeUnauthorized, "unauthorized", nil)
	}
	return h.submissionService.FindByUserAndProblem(ctx, userID, req.ProblemID)
}
