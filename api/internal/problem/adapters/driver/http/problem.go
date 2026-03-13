package http

import (
	"context"
	"net/http"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

// ProblemHandler defines the problem HTTP handler interface.
type ProblemHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.ProblemResponse], error)
	Get(ctx context.Context, req *dto.GetProblemRequest) (*dto.ProblemResponse, error)
	Create(ctx context.Context, req *dto.CreateProblemRequest) (*dto.ProblemResponse, error)
	Update(ctx context.Context, req *dto.UpdateProblemRequest) (*dto.ProblemResponse, error)
	Delete(ctx context.Context, req *dto.DeleteProblemRequest) (*dto.ProblemResponse, error)
}

type problemHandler struct {
	handler.BaseHandler
	problemService ports.ProblemService
}

// NewProblemHandler creates a new ProblemHandler instance.
func NewProblemHandler(problemService ports.ProblemService) ProblemHandler {
	return &problemHandler{problemService: problemService}
}

func (h *problemHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.ProblemResponse], error) {
	return h.problemService.Find(ctx, req)
}

func (h *problemHandler) Get(ctx context.Context, req *dto.GetProblemRequest) (*dto.ProblemResponse, error) {
	return h.problemService.Get(ctx, req.ID)
}

func (h *problemHandler) Create(ctx context.Context, req *dto.CreateProblemRequest) (*dto.ProblemResponse, error) {
	userIDVal := ctx.Value(constraints.ContextKeyUserID)
	if userIDVal == nil {
		return nil, apperr.New(response.CodeUnauthorized, "user id not found in context", http.StatusUnauthorized, nil)
	}
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, apperr.New(response.CodeInternalError, "invalid user id type", http.StatusInternalServerError, nil)
	}

	return h.problemService.Create(ctx, userID, req)
}

func (h *problemHandler) Update(ctx context.Context, req *dto.UpdateProblemRequest) (*dto.ProblemResponse, error) {
	return h.problemService.Update(ctx, req.ID, req)
}

func (h *problemHandler) Delete(ctx context.Context, req *dto.DeleteProblemRequest) (*dto.ProblemResponse, error) {
	return nil, h.problemService.Delete(ctx, req.ID)
}
