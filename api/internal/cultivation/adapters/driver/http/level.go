package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// LevelHandler defines the level HTTP handler interface.
type LevelHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.LevelResponse], error)
	FindAll(ctx context.Context, req *dto.FindAllLevelsRequest) ([]*dto.LevelResponse, error)
	Get(ctx context.Context, req *dto.GetLevelRequest) (*dto.LevelResponse, error)
	Create(ctx context.Context, req *dto.CreateLevelRequest) (*dto.LevelResponse, error)
	Update(ctx context.Context, req *dto.UpdateLevelRequest) (*dto.LevelResponse, error)
	Delete(ctx context.Context, req *dto.DeleteLevelRequest) (*dto.LevelResponse, error)
}

type levelHandler struct {
	handler.BaseHandler
	levelService ports.LevelService
}

func NewLevelHandler(svc ports.LevelService) LevelHandler {
	return &levelHandler{levelService: svc}
}

func (h *levelHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.LevelResponse], error) {
	return h.levelService.Find(ctx, req)
}

func (h *levelHandler) FindAll(ctx context.Context, _ *dto.FindAllLevelsRequest) ([]*dto.LevelResponse, error) {
	return h.levelService.FindAll(ctx)
}

func (h *levelHandler) Get(ctx context.Context, req *dto.GetLevelRequest) (*dto.LevelResponse, error) {
	return h.levelService.Get(ctx, req.ID)
}

func (h *levelHandler) Create(ctx context.Context, req *dto.CreateLevelRequest) (*dto.LevelResponse, error) {
	return h.levelService.Create(ctx, req)
}

func (h *levelHandler) Update(ctx context.Context, req *dto.UpdateLevelRequest) (*dto.LevelResponse, error) {
	return h.levelService.Update(ctx, req.ID, req)
}

func (h *levelHandler) Delete(ctx context.Context, req *dto.DeleteLevelRequest) (*dto.LevelResponse, error) {
	return nil, h.levelService.Delete(ctx, req.ID)
}
