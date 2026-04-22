package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// RankingHandler defines the ranking HTTP handler interface.
type RankingHandler interface {
	GetTopByRating(ctx context.Context, req *dto.GetTopRankingRequest) ([]*dto.RankingEntry, error)
	GetTopByExp(ctx context.Context, req *dto.GetTopRankingRequest) ([]*dto.RankingEntry, error)
}

type rankingHandler struct {
	handler.BaseHandler
	rankingService ports.RankingService
}

// NewRankingHandler creates a new RankingHandler instance.
func NewRankingHandler(rankingService ports.RankingService) RankingHandler {
	return &rankingHandler{rankingService: rankingService}
}

func (h *rankingHandler) GetTopByRating(ctx context.Context, req *dto.GetTopRankingRequest) ([]*dto.RankingEntry, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	return h.rankingService.GetTopByRating(ctx, limit)
}

func (h *rankingHandler) GetTopByExp(ctx context.Context, req *dto.GetTopRankingRequest) ([]*dto.RankingEntry, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	return h.rankingService.GetTopByExp(ctx, limit)
}
