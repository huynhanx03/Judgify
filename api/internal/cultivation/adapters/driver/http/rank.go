package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// RankHandler defines the rank HTTP handler interface.
type RankHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.RankResponse], error)
	Get(ctx context.Context, req *dto.GetRankRequest) (*dto.RankResponse, error)
	Create(ctx context.Context, req *dto.CreateRankRequest) (*dto.RankResponse, error)
	Update(ctx context.Context, req *dto.UpdateRankRequest) (*dto.RankResponse, error)
	Delete(ctx context.Context, req *dto.DeleteRankRequest) (*dto.RankResponse, error)
}

type rankHandler struct {
	handler.BaseHandler
	rankService ports.RankService
}

func NewRankHandler(svc ports.RankService) RankHandler {
	return &rankHandler{rankService: svc}
}

func (h *rankHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.RankResponse], error) {
	return h.rankService.Find(ctx, req)
}

func (h *rankHandler) Get(ctx context.Context, req *dto.GetRankRequest) (*dto.RankResponse, error) {
	return h.rankService.Get(ctx, req.ID)
}

func (h *rankHandler) Create(ctx context.Context, req *dto.CreateRankRequest) (*dto.RankResponse, error) {
	return h.rankService.Create(ctx, req)
}

func (h *rankHandler) Update(ctx context.Context, req *dto.UpdateRankRequest) (*dto.RankResponse, error) {
	return h.rankService.Update(ctx, req.ID, req)
}

func (h *rankHandler) Delete(ctx context.Context, req *dto.DeleteRankRequest) (*dto.RankResponse, error) {
	return nil, h.rankService.Delete(ctx, req.ID)
}
