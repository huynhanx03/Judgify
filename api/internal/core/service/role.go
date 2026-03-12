package service

import (
	"context"
	"net/http"
	"sort"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/core/dto"
	"github.com/huynhanx03/judgify/internal/core/entity"
	"github.com/huynhanx03/judgify/internal/core/mapper"
	"github.com/huynhanx03/judgify/internal/ports"
)

const roleServiceName = "RoleService"

type roleService struct {
	roleRepo     ports.RoleRepository
	cacheService ports.CacheService
}

// NewRoleService creates a new RoleService instance.
func NewRoleService(roleRepo ports.RoleRepository, cacheService ports.CacheService) ports.RoleService {
	return &roleService{roleRepo: roleRepo, cacheService: cacheService}
}

// Find retrieves roles with pagination.
func (s *roleService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.RoleResponse], error) {
	roles, err := s.roleRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if roles.Records == nil {
		return &d.Paginated[*dto.RoleResponse]{
			Records:    &[]*dto.RoleResponse{},
			Pagination: roles.Pagination,
		}, nil
	}

	entities := *roles.Records
	responses := make([]*dto.RoleResponse, len(entities))
	for i, role := range entities {
		responses[i] = mapper.ToRoleResponse(role)
	}

	return &d.Paginated[*dto.RoleResponse]{
		Records:    &responses,
		Pagination: roles.Pagination,
	}, nil
}

// Get retrieves a role by ID.
func (s *roleService) Get(ctx context.Context, id int) (*dto.RoleResponse, error) {
	role, err := s.roleRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return mapper.ToRoleResponse(role), nil
}

// Create creates a new role.
func (s *roleService) Create(ctx context.Context, req *dto.CreateRoleRequest) (*dto.RoleResponse, error) {
	role := mapper.ToRoleEntityFromCreate(req)

	err := global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		if err := s.roleRepo.Create(ctx, role); err != nil {
			return err
		}

		if err := s.rebuildTree(ctx); err != nil {
			return apperr.NewError(roleServiceName, response.CodeDatabaseError, constant.MsgRebuildTreeFailed, http.StatusInternalServerError, err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Invalidate Permission Config Version
	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		// Log error but don't fail request
	}

	return mapper.ToRoleResponse(role), nil
}

// Update updates an existing role.
func (s *roleService) Update(ctx context.Context, id int, req *dto.UpdateRoleRequest) (*dto.RoleResponse, error) {
	var role *entity.Role
	err := global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		var err error
		role, err = s.roleRepo.Get(ctx, id)
		if err != nil {
			return err
		}

		if req.Name != nil {
			role.Name = *req.Name
		}
		if req.Level != nil {
			role.Level = *req.Level
		}

		parentChanged := false
		if req.ParentID != nil {
			if role.ParentID != *req.ParentID {
				if *req.ParentID == id {
					return apperr.NewError(roleServiceName, response.CodeInvalidID, constant.MsgInvalidParentID, http.StatusBadRequest, nil)
				}
				role.ParentID = *req.ParentID
				parentChanged = true
			}
		}

		role.ID = id
		if err := s.roleRepo.Update(ctx, role); err != nil {
			return err
		}

		if parentChanged {
			if err := s.rebuildTree(ctx); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Invalidate Permission Config Version
	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		// Log error but don't fail request
	}

	return mapper.ToRoleResponse(role), nil
}

// Delete removes a role by ID.
func (s *roleService) Delete(ctx context.Context, id int) error {
	err := global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		exists, err := s.roleRepo.Exists(ctx, id)
		if err != nil {
			return err
		}

		if !exists {
			return apperr.NewError(roleServiceName, response.CodeNotFound, apperr.MsgNotFound, http.StatusNotFound, nil)
		}

		if err := s.roleRepo.Delete(ctx, id); err != nil {
			return err
		}

		if err := s.rebuildTree(ctx); err != nil {
			return apperr.NewError(roleServiceName, response.CodeDatabaseError, constant.MsgRebuildTreeFailed, http.StatusInternalServerError, err)
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Invalidate Permission Config Version
	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		// Log error but don't fail request
	}

	return nil
}

// rebuildTree recalculates lft and rgt values for the entire role tree using DFS.
func (s *roleService) rebuildTree(ctx context.Context) error {
	roles, err := s.roleRepo.FindAll(ctx)
	if err != nil {
		return err
	}

	// Build adjacency list
	childrenMap := make(map[int][]*entity.Role)
	var rootRoles []*entity.Role

	// Separate roots from children
	for _, role := range roles {
		if role.ParentID == -1 {
			rootRoles = append(rootRoles, role)
		} else {
			pid := role.ParentID
			childrenMap[pid] = append(childrenMap[pid], role)
		}
	}

	// Sort to ensure deterministic order
	sort.Slice(rootRoles, func(i, j int) bool {
		return rootRoles[i].ID < rootRoles[j].ID
	})
	for _, children := range childrenMap {
		sort.Slice(children, func(i, j int) bool {
			return children[i].ID < children[j].ID
		})
	}

	var updates []*entity.Role
	counter := 1

	var dfs func(role *entity.Role)
	dfs = func(role *entity.Role) {
		role.Lft = counter
		counter++

		if children, exists := childrenMap[role.ID]; exists {
			for _, child := range children {
				dfs(child)
			}
		}

		role.Rgt = counter
		counter++
		updates = append(updates, role)
	}

	for _, root := range rootRoles {
		dfs(root)
	}

	return s.roleRepo.UpdateBulk(ctx, updates)
}
