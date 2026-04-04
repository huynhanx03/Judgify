package service

import (
	"context"
	"fmt"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/cultivation/constant"
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)


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
	logger.FromContext(ctx).Info("element created", zap.Int("element_id", e.ID))
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
	if err := s.elementRepo.Update(ctx, e); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("element updated", zap.Int("element_id", e.ID))
	return mapper.ToElementResponse(e), nil
}

func (s *elementService) Delete(ctx context.Context, id int) error {
	exists, err := s.elementRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjElement), nil)
	}
	if err := s.elementRepo.Delete(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("element deleted", zap.Int("element_id", id))
	return nil
}

// FindAll retrieves all elements without pagination.
func (s *elementService) FindAll(ctx context.Context) ([]*dto.ElementResponse, error) {
	elements, err := s.elementRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]*dto.ElementResponse, len(elements))
	for i, e := range elements {
		responses[i] = mapper.ToElementResponse(e)
	}
	return responses, nil
}
