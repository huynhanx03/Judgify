package service

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/mapper"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)


type resourceService struct {
	resourceRepo ports.ResourceRepository
	cacheService ports.CacheService
}

// NewResourceService creates a new ResourceService instance.
func NewResourceService(resourceRepo ports.ResourceRepository, cacheService ports.CacheService) ports.ResourceService {
	return &resourceService{resourceRepo: resourceRepo, cacheService: cacheService}
}

// Find retrieves resources with pagination.
func (s *resourceService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.ResourceResponse], error) {
	resources, err := s.resourceRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if resources.Records == nil {
		return &d.Paginated[*dto.ResourceResponse]{
			Records:    &[]*dto.ResourceResponse{},
			Pagination: resources.Pagination,
		}, nil
	}

	entities := *resources.Records
	responses := make([]*dto.ResourceResponse, len(entities))
	for i, resource := range entities {
		responses[i] = mapper.ToResourceResponse(resource)
	}

	return &d.Paginated[*dto.ResourceResponse]{
		Records:    &responses,
		Pagination: resources.Pagination,
	}, nil
}

// Get retrieves a resource by ID.
func (s *resourceService) Get(ctx context.Context, id int) (*dto.ResourceResponse, error) {
	resource, err := s.resourceRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return mapper.ToResourceResponse(resource), nil
}

// Create creates a new resource.
func (s *resourceService) Create(ctx context.Context, req *dto.CreateResourceRequest) (*dto.ResourceResponse, error) {
	resource := mapper.ToResourceEntityFromCreate(req)

	if err := s.resourceRepo.Create(ctx, resource); err != nil {
		return nil, err
	}

	// Invalidate Permission Config Version
	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		// Log error but don't fail request
	}

	logger.FromContext(ctx).Info("resource created", zap.Int("resource_id", resource.ID))
	return mapper.ToResourceResponse(resource), nil
}

// Update updates an existing resource.
func (s *resourceService) Update(ctx context.Context, id int, req *dto.UpdateResourceRequest) (*dto.ResourceResponse, error) {
	resource, err := s.resourceRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Key != nil {
		resource.Key = *req.Key
	}
	if req.Description != nil {
		resource.Description = req.Description
	}

	resource.ID = id
	if err := s.resourceRepo.Update(ctx, resource); err != nil {
		return nil, err
	}

	// Invalidate Permission Config Version
	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		// Log error but don't fail request
	}

	logger.FromContext(ctx).Info("resource updated", zap.Int("resource_id", resource.ID))
	return mapper.ToResourceResponse(resource), nil
}

// Delete removes a resource by ID.
func (s *resourceService) Delete(ctx context.Context, id int) error {
	exists, err := s.resourceRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.New(response.CodeNotFound, apperr.MsgNotFound, nil)
	}

	if err := s.resourceRepo.Delete(ctx, id); err != nil {
		return err
	}

	// Invalidate Permission Config Version
	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		// Log error but don't fail request
	}

	logger.FromContext(ctx).Info("resource deleted", zap.Int("resource_id", id))
	return nil
}
