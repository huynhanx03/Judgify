package http

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/ports"
)

// RankHandler defines the rank HTTP handler interface.
type RankHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*rankResponse], error)
	FindAll(ctx context.Context) ([]*rankResponse, error)
	Get(ctx context.Context, req *rankIDRequest) (*rankResponse, error)
	Create(ctx context.Context, req *createRankRequest) (*rankResponse, error)
	Update(ctx context.Context, req *updateRankRequest) (*rankResponse, error)
	Delete(ctx context.Context, req *rankIDRequest) (*rankResponse, error)
}

type rankHandler struct {
	handler.BaseHandler
	rankService ports.RankService
}

func NewRankHandler(svc ports.RankService) RankHandler {
	return &rankHandler{rankService: svc}
}

func (h *rankHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*rankResponse], error) {
	result, err := h.rankService.Find(ctx, req)
	if err != nil {
		return nil, err
	}
	return toRankPage(result), nil
}

func (h *rankHandler) FindAll(ctx context.Context) ([]*rankResponse, error) {
	result, err := h.rankService.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return toRankResponses(result), nil
}

func (h *rankHandler) Get(ctx context.Context, req *rankIDRequest) (*rankResponse, error) {
	id, err := toRankID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.rankService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return toRankResponse(result), nil
}

func (h *rankHandler) Create(ctx context.Context, req *createRankRequest) (*rankResponse, error) {
	result, err := h.rankService.Create(ctx, toCreateRankCommand(req))
	if err != nil {
		return nil, err
	}
	return toRankResponse(result), nil
}

func (h *rankHandler) Update(ctx context.Context, req *updateRankRequest) (*rankResponse, error) {
	id, command, err := toUpdateRankCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.rankService.Update(ctx, id, command)
	if err != nil {
		return nil, err
	}
	return toRankResponse(result), nil
}

func (h *rankHandler) Delete(ctx context.Context, req *rankIDRequest) (*rankResponse, error) {
	id, err := toRankID(req)
	if err != nil {
		return nil, err
	}
	return nil, h.rankService.Delete(ctx, id)
}
