package http

import (
	"context"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"
)

// GachaHandler defines the gacha HTTP handler interface.
type GachaHandler interface {
	Roll(ctx context.Context, _ *dto.GachaRollRequest) (*dto.GachaRollResponse, error)
}

type gachaHandler struct {
	handler.BaseHandler
	gachaService ports.GachaService
}

// NewGachaHandler creates a new GachaHandler.
func NewGachaHandler(svc ports.GachaService) GachaHandler {
	return &gachaHandler{gachaService: svc}
}

func (h *gachaHandler) Roll(ctx context.Context, req *dto.GachaRollRequest) (*dto.GachaRollResponse, error) {
	return h.gachaService.Roll(ctx)
}
