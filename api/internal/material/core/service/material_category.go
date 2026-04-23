package service

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/material/core/dto"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
	"github.com/huynhanx03/judgify/internal/material/core/mapper"
	"github.com/huynhanx03/judgify/internal/material/ports"
)

type materialCategoryService struct {
	repo ports.MaterialCategoryRepository
}

// NewMaterialCategoryService creates a new MaterialCategoryService.
func NewMaterialCategoryService(repo ports.MaterialCategoryRepository) ports.MaterialCategoryService {
	return &materialCategoryService{repo: repo}
}

func (s *materialCategoryService) FindAll(ctx context.Context) ([]*dto.MaterialCategoryResponse, error) {
	categories, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*dto.MaterialCategoryResponse, len(categories))
	for i, c := range categories {
		result[i] = mapper.ToMaterialCategoryResponse(c)
	}
	return result, nil
}

func (s *materialCategoryService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.MaterialCategoryResponse], error) {
	paginated, err := s.repo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	records := *paginated.Records
	dtos := make([]*dto.MaterialCategoryResponse, len(records))
	for i, e := range records {
		dtos[i] = mapper.ToMaterialCategoryResponse(e)
	}

	return &d.Paginated[*dto.MaterialCategoryResponse]{
		Records:    &dtos,
		Pagination: paginated.Pagination,
	}, nil
}

func (s *materialCategoryService) Get(ctx context.Context, id int) (*dto.MaterialCategoryResponse, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToMaterialCategoryResponse(e), nil
}

func (s *materialCategoryService) Create(ctx context.Context, req *dto.CreateMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error) {
	e := &entity.MaterialCategory{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.repo.Create(ctx, e); err != nil {
		return nil, err
	}

	return mapper.ToMaterialCategoryResponse(e), nil
}

func (s *materialCategoryService) Update(ctx context.Context, id int, req *dto.UpdateMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		e.Name = *req.Name
	}
	if req.Description != nil {
		e.Description = *req.Description
	}

	if err := s.repo.Update(ctx, e); err != nil {
		return nil, err
	}

	return mapper.ToMaterialCategoryResponse(e), nil
}

func (s *materialCategoryService) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
