package service

import (
	"context"
	"net/http"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/constant"
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

const elementServiceName = "ElementService"

type elementService struct {
	elementRepo ports.ElementRepository
}

// NewElementService creates a new ElementService instance.
func NewElementService(elementRepo ports.ElementRepository) ports.ElementService {
	return &elementService{elementRepo: elementRepo}
}

func (s *elementService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.ElementResponse], error) {
	result, err := s.elementRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*dto.ElementResponse]{Records: &[]*dto.ElementResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*dto.ElementResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToElementResponse(e)
	}

	return &d.Paginated[*dto.ElementResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *elementService) Get(ctx context.Context, id int) (*dto.ElementResponse, error) {
	e, err := s.elementRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToElementResponse(e), nil
}

func (s *elementService) Create(ctx context.Context, req *dto.CreateElementRequest) (*dto.ElementResponse, error) {
	e := mapper.ToElementEntityFromCreate(req)
	if err := s.elementRepo.Create(ctx, e); err != nil {
		return nil, err
	}
	return mapper.ToElementResponse(e), nil
}

func (s *elementService) Update(ctx context.Context, id int, req *dto.UpdateElementRequest) (*dto.ElementResponse, error) {
	e, err := s.elementRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		e.Name = *req.Name
	}
	if req.Code != nil {
		e.Code = *req.Code
	}
	if req.Description != nil {
		e.Description = *req.Description
	}
	if req.Color != nil {
		e.Color = *req.Color
	}
	if req.Icon != nil {
		e.Icon = *req.Icon
	}
	if req.Order != nil {
		e.Order = *req.Order
	}

	if err := s.elementRepo.Update(ctx, e); err != nil {
		return nil, err
	}
	return mapper.ToElementResponse(e), nil
}

func (s *elementService) Delete(ctx context.Context, id int) error {
	exists, err := s.elementRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.NewError(elementServiceName, response.CodeNotFound, constant.MsgElementNotFound, http.StatusNotFound, nil)
	}
	return s.elementRepo.Delete(ctx, id)
}
