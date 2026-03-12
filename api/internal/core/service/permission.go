package service

import (
	"context"
	"net/http"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/core/dto"
	"github.com/huynhanx03/judgify/internal/core/mapper"
	"github.com/huynhanx03/judgify/internal/ports"
)

const permissionServiceName = "PermissionService"

type permissionService struct {
	permissionRepo ports.PermissionRepository
	cacheService   ports.CacheService
}

// NewPermissionService creates a new PermissionService instance.
func NewPermissionService(permissionRepo ports.PermissionRepository, cacheService ports.CacheService) ports.PermissionService {
	return &permissionService{permissionRepo: permissionRepo, cacheService: cacheService}
}

// Find retrieves permissions with pagination.
func (s *permissionService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.PermissionResponse], error) {
	permissions, err := s.permissionRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if permissions.Records == nil {
		return &d.Paginated[*dto.PermissionResponse]{
			Records:    &[]*dto.PermissionResponse{},
			Pagination: permissions.Pagination,
		}, nil
	}

	entities := *permissions.Records
	responses := make([]*dto.PermissionResponse, len(entities))
	for i, permission := range entities {
		responses[i] = mapper.ToPermissionResponse(permission)
	}

	return &d.Paginated[*dto.PermissionResponse]{
		Records:    &responses,
		Pagination: permissions.Pagination,
	}, nil
}

// Get retrieves a permission by ID
func (s *permissionService) Get(ctx context.Context, id int) (*dto.PermissionResponse, error) {
	permission, err := s.permissionRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return mapper.ToPermissionResponse(permission), nil
}

// Create creates a new permission.
func (s *permissionService) Create(ctx context.Context, req *dto.CreatePermissionRequest) (*dto.PermissionResponse, error) {
	permission := mapper.ToPermissionEntityFromCreate(req)

	if err := s.permissionRepo.Create(ctx, permission); err != nil {
		return nil, err
	}

	// Invalidate Permission Config Version
	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		// Log error but don't fail request
	}

	return mapper.ToPermissionResponse(permission), nil
}

// Update updates an existing permission.
func (s *permissionService) Update(ctx context.Context, id int, req *dto.UpdatePermissionRequest) (*dto.PermissionResponse, error) {
	permission, err := s.permissionRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Description != nil {
		permission.Description = req.Description
	}
	if req.Scopes != nil {
		permission.Scopes = *req.Scopes
	}

	permission.ID = id
	if err := s.permissionRepo.Update(ctx, permission); err != nil {
		return nil, err
	}

	// Invalidate Permission Config Version
	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		// Log error but don't fail request
	}

	return mapper.ToPermissionResponse(permission), nil
}

// Delete removes a permission by ID.
func (s *permissionService) Delete(ctx context.Context, id int) error {
	exists, err := s.permissionRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.NewError(permissionServiceName, response.CodeNotFound, apperr.MsgNotFound, http.StatusNotFound, nil)
	}

	if err := s.permissionRepo.Delete(ctx, id); err != nil {
		return err
	}

	// Invalidate Permission Config Version
	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		// Log error but don't fail request
	}

	return nil
}
