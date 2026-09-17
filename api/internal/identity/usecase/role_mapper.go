package usecase

import (
	"github.com/huynhanx03/judgify/internal/constant"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// ToRoleResponse converts Role entity to RoleResponse DTO.
func ToRoleResponse(e *entity.Role) *dto.RoleResponse {
	if e == nil {
		return nil
	}
	return &dto.RoleResponse{
		ID:          e.ID,
		Key:         e.Key,
		Name:        e.Name,
		Description: e.Description,
		IsSystem:    e.IsSystem,
		IsProtected: e.Key == constant.ProtectedSuperadminRoleKey,
		Status:      string(e.Status),
		Version:     e.Version,
	}
}

// ToRoleEntityFromCreate converts CreateRoleRequest to Role entity.
func ToRoleEntityFromCreate(req *dto.CreateRoleRequest) *entity.Role {
	return &entity.Role{
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		IsSystem:    false,
		Status:      entity.RoleStatusActive,
		Version:     1,
	}
}
