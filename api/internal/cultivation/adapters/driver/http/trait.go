package http

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/ports"
)

// TraitHandler defines the trait HTTP handler interface.
type TraitHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*traitResponse], error)
	FindAll(ctx context.Context) ([]*traitResponse, error)
	Get(ctx context.Context, req *traitIDRequest) (*traitResponse, error)
	Create(ctx context.Context, req *createTraitRequest) (*traitResponse, error)
	Update(ctx context.Context, req *updateTraitRequest) (*traitResponse, error)
	Delete(ctx context.Context, req *deleteTraitRequest) (*traitResponse, error)
}

type traitHandler struct {
	handler.BaseHandler
	traitService ports.TraitService
}

func NewTraitHandler(svc ports.TraitService) TraitHandler {
	return &traitHandler{traitService: svc}
}

func (h *traitHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*traitResponse], error) {
	result, err := h.traitService.Find(ctx, req)
	if err != nil {
		return nil, err
	}
	return toTraitPage(result), nil
}

func (h *traitHandler) FindAll(ctx context.Context) ([]*traitResponse, error) {
	result, err := h.traitService.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return toTraitResponses(result), nil
}

func (h *traitHandler) Get(ctx context.Context, req *traitIDRequest) (*traitResponse, error) {
	id, err := toTraitID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.traitService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return toTraitResponse(result), nil
}

func (h *traitHandler) Create(ctx context.Context, req *createTraitRequest) (*traitResponse, error) {
	command, err := toCreateTraitCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.traitService.Create(ctx, command)
	if err != nil {
		return nil, err
	}
	return toTraitResponse(result), nil
}

func (h *traitHandler) Update(ctx context.Context, req *updateTraitRequest) (*traitResponse, error) {
	id, command, err := toUpdateTraitCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.traitService.Update(ctx, id, command)
	if err != nil {
		return nil, err
	}
	return toTraitResponse(result), nil
}

func (h *traitHandler) Delete(ctx context.Context, req *deleteTraitRequest) (*traitResponse, error) {
	id, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, fmt.Errorf("parse trait id: %w", err)
	}
	return nil, h.traitService.Delete(ctx, id, req.ExpectedVersion, req.Reason)
}
