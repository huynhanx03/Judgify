package http

import (
	"context"

	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	handlerCommon "github.com/huynhanx03/judgify/pkg/common/http/handler"
)

// StandingHandler defines the standing HTTP handler interface.
type StandingHandler interface {
	GetStandings(ctx context.Context, req *dto.GetStandingsRequest) ([]*dto.StandingResponse, error)
}

type standingHandler struct {
	handlerCommon.BaseHandler
	standingService ports.StandingService
}

// NewStandingHandler creates a new StandingHandler instance.
func NewStandingHandler(standingService ports.StandingService) StandingHandler {
	return &standingHandler{standingService: standingService}
}

func (h *standingHandler) GetStandings(ctx context.Context, req *dto.GetStandingsRequest) ([]*dto.StandingResponse, error) {
	return h.standingService.GetStandings(ctx, req.ContestID)
}
