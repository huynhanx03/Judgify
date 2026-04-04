package mapper

import (
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// ToRoleResponse converts Role entity to RoleResponse DTO.
func ToRoleResponse(e *entity.Role) *dto.RoleResponse {
	if e == nil {
		return nil
	}
	return &dto.RoleResponse{
		ID:    e.ID,
		Name:  e.Name,
		Level: e.Level,
	}
}

// ToRoleEntityFromCreate converts CreateRoleRequest to Role entity.
func ToRoleEntityFromCreate(req *dto.CreateRoleRequest) *entity.Role {
	parentID := -1
	if req.ParentID != nil {
		parentID = *req.ParentID
	}
	return &entity.Role{
		Name:     req.Name,
		Level:    req.Level,
		ParentID: parentID,
	}
}
