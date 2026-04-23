package http

import (
	"context"

	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	handlerCommon "github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"

	d "github.com/huynhanx03/judgify/pkg/dto"
)

// ContestHandler defines the contest HTTP handler interface.
type ContestHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.ContestResponse], error)
	Get(ctx context.Context, req *dto.GetContestRequest) (*dto.ContestResponse, error)
	Create(ctx context.Context, req *dto.CreateContestRequest) (*dto.ContestResponse, error)
	Update(ctx context.Context, req *dto.UpdateContestRequest) (*dto.ContestResponse, error)
	Delete(ctx context.Context, req *dto.DeleteContestRequest) (*dto.ContestResponse, error)
}

type contestHandler struct {
	handlerCommon.BaseHandler
	contestService ports.ContestService
}

// NewContestHandler creates a new ContestHandler instance.
func NewContestHandler(contestService ports.ContestService) ContestHandler {
	return &contestHandler{contestService: contestService}
}

func (h *contestHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.ContestResponse], error) {
	return h.contestService.Find(ctx, req)
}

func (h *contestHandler) Get(ctx context.Context, req *dto.GetContestRequest) (*dto.ContestResponse, error) {
	userID := extractUserID(ctx)
	return h.contestService.Get(ctx, req.ID, userID)
}

func (h *contestHandler) Create(ctx context.Context, req *dto.CreateContestRequest) (*dto.ContestResponse, error) {
	userIDVal := ctx.Value(constraints.ContextKeyUserID)
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, apperr.New(response.CodeUnauthorized, "unauthorized", nil)
	}
	return h.contestService.Create(ctx, userID, req)
}

func (h *contestHandler) Update(ctx context.Context, req *dto.UpdateContestRequest) (*dto.ContestResponse, error) {
	return h.contestService.Update(ctx, req.ID, req)
}

func (h *contestHandler) Delete(ctx context.Context, req *dto.DeleteContestRequest) (*dto.ContestResponse, error) {
	if err := h.contestService.Delete(ctx, req.ID); err != nil {
		return nil, err
	}
	return nil, nil
}

// extractUserID safely extracts user ID from context, returns 0 if not found.
func extractUserID(ctx context.Context) int {
	if v := ctx.Value(constraints.ContextKeyUserID); v != nil {
		if id, ok := v.(int); ok {
			return id
		}
	}
	return 0
}

// ensure handler.Wrap is available (used by handler group)
var _ = handler.Wrap[any, any]
