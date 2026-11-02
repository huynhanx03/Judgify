package http

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/ports"
)

// LevelHandler defines the level HTTP handler interface.
type LevelHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*levelResponse], error)
	FindAll(ctx context.Context) ([]*levelResponse, error)
	Get(ctx context.Context, req *levelIDRequest) (*levelResponse, error)
	Create(ctx context.Context, req *createLevelRequest) (*levelResponse, error)
	Update(ctx context.Context, req *updateLevelRequest) (*levelResponse, error)
	Delete(ctx context.Context, req *levelIDRequest) (*levelResponse, error)
}

type levelHandler struct {
	handler.BaseHandler
	levelService ports.LevelService
}

func NewLevelHandler(svc ports.LevelService) LevelHandler {
	return &levelHandler{levelService: svc}
}

func (h *levelHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*levelResponse], error) {
	result, err := h.levelService.Find(ctx, req)
	if err != nil {
		return nil, err
	}
	return toLevelPage(result), nil
}

func (h *levelHandler) FindAll(ctx context.Context) ([]*levelResponse, error) {
	result, err := h.levelService.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return toLevelResponses(result), nil
}

func (h *levelHandler) Get(ctx context.Context, req *levelIDRequest) (*levelResponse, error) {
	id, err := toLevelID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.levelService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return toLevelResponse(result), nil
}

func (h *levelHandler) Create(ctx context.Context, req *createLevelRequest) (*levelResponse, error) {
	result, err := h.levelService.Create(ctx, toCreateLevelCommand(req))
	if err != nil {
		return nil, err
	}
	return toLevelResponse(result), nil
}

func (h *levelHandler) Update(ctx context.Context, req *updateLevelRequest) (*levelResponse, error) {
	id, command, err := toUpdateLevelCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.levelService.Update(ctx, id, command)
	if err != nil {
		return nil, err
	}
	return toLevelResponse(result), nil
}

func (h *levelHandler) Delete(ctx context.Context, req *levelIDRequest) (*levelResponse, error) {
	id, err := toLevelID(req)
	if err != nil {
		return nil, err
	}
	return nil, h.levelService.Delete(ctx, id)
}
