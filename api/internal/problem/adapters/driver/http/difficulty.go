package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

// DifficultyHandler defines the difficulty HTTP handler interface.
type DifficultyHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.DifficultyResponse], error)
	FindAll(ctx context.Context, req *dto.FindAllDifficultiesRequest) ([]*dto.DifficultyResponse, error)
	Get(ctx context.Context, req *dto.GetDifficultyRequest) (*dto.DifficultyResponse, error)
	Create(ctx context.Context, req *dto.CreateDifficultyRequest) (*dto.DifficultyResponse, error)
	Update(ctx context.Context, req *dto.UpdateDifficultyRequest) (*dto.DifficultyResponse, error)
	Delete(ctx context.Context, req *dto.DeleteDifficultyRequest) (*dto.DifficultyResponse, error)
}

type difficultyHandler struct {
	handler.BaseHandler
	difficultyService ports.DifficultyService
}

// NewDifficultyHandler creates a new DifficultyHandler instance.
func NewDifficultyHandler(difficultyService ports.DifficultyService) DifficultyHandler {
	return &difficultyHandler{difficultyService: difficultyService}
}

func (h *difficultyHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.DifficultyResponse], error) {
	return h.difficultyService.Find(ctx, req)
}

func (h *difficultyHandler) FindAll(ctx context.Context, _ *dto.FindAllDifficultiesRequest) ([]*dto.DifficultyResponse, error) {
	return h.difficultyService.FindAll(ctx)
}

func (h *difficultyHandler) Get(ctx context.Context, req *dto.GetDifficultyRequest) (*dto.DifficultyResponse, error) {
	return h.difficultyService.Get(ctx, req.ID)
}

func (h *difficultyHandler) Create(ctx context.Context, req *dto.CreateDifficultyRequest) (*dto.DifficultyResponse, error) {
	return h.difficultyService.Create(ctx, req)
}

func (h *difficultyHandler) Update(ctx context.Context, req *dto.UpdateDifficultyRequest) (*dto.DifficultyResponse, error) {
	return h.difficultyService.Update(ctx, req.ID, req)
}

func (h *difficultyHandler) Delete(ctx context.Context, req *dto.DeleteDifficultyRequest) (*dto.DifficultyResponse, error) {
	return nil, h.difficultyService.Delete(ctx, req.ID)
}
