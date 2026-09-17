package http

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/ports"
)

// DifficultyHandler defines the difficulty HTTP handler interface.
type DifficultyHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*DifficultyResponse], error)
	FindAll(ctx context.Context) ([]*DifficultyResponse, error)
	Get(ctx context.Context, req *difficultyIDRequest) (*DifficultyResponse, error)
	Create(ctx context.Context, req *CreateDifficultyRequest) (*DifficultyResponse, error)
	Update(ctx context.Context, req *UpdateDifficultyRequest) (*DifficultyResponse, error)
	Delete(ctx context.Context, req *difficultyIDRequest) (*DifficultyResponse, error)
}

type difficultyHandler struct {
	handler.BaseHandler
	difficultyService ports.DifficultyService
}

// NewDifficultyHandler creates a new DifficultyHandler instance.
func NewDifficultyHandler(difficultyService ports.DifficultyService) DifficultyHandler {
	return &difficultyHandler{difficultyService: difficultyService}
}

func (h *difficultyHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*DifficultyResponse], error) {
	results, err := h.difficultyService.Find(ctx, req)
	if err != nil {
		return nil, err
	}
	responses := make([]*DifficultyResponse, 0)
	if results.Records != nil {
		for _, result := range *results.Records {
			responses = append(responses, toDifficultyResponse(result))
		}
	}
	return &d.Paginated[*DifficultyResponse]{Records: &responses, Pagination: results.Pagination}, nil
}

func (h *difficultyHandler) FindAll(ctx context.Context) ([]*DifficultyResponse, error) {
	results, err := h.difficultyService.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]*DifficultyResponse, len(results))
	for i, result := range results {
		responses[i] = toDifficultyResponse(result)
	}
	return responses, nil
}

func (h *difficultyHandler) Get(ctx context.Context, req *difficultyIDRequest) (*DifficultyResponse, error) {
	id, err := toDifficultyID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.difficultyService.Get(ctx, id)
	return toDifficultyResponse(result), err
}

func (h *difficultyHandler) Create(ctx context.Context, req *CreateDifficultyRequest) (*DifficultyResponse, error) {
	command := toCreateDifficultyCommand(req)
	result, err := h.difficultyService.Create(ctx, &command)
	return toDifficultyResponse(result), err
}

func (h *difficultyHandler) Update(ctx context.Context, req *UpdateDifficultyRequest) (*DifficultyResponse, error) {
	id, command, err := toUpdateDifficultyCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.difficultyService.Update(ctx, id, &command)
	return toDifficultyResponse(result), err
}

func (h *difficultyHandler) Delete(ctx context.Context, req *difficultyIDRequest) (*DifficultyResponse, error) {
	id, err := toDifficultyID(req)
	if err != nil {
		return nil, err
	}
	return nil, h.difficultyService.Delete(ctx, id)
}
