package mapper

import (
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// ToPermissionResponse converts Permission entity to PermissionResponse DTO.
func ToPermissionResponse(e *entity.Permission) *dto.PermissionResponse {
	if e == nil {
		return nil
	}
	return &dto.PermissionResponse{
		ID:          e.ID,
		RoleID:      e.RoleID,
		ResourceID:  e.ResourceID,
		Description: e.Description,
		Scopes:      e.Scopes,
	}
}

// ToPermissionEntityFromCreate converts CreatePermissionRequest to Permission entity.
func ToPermissionEntityFromCreate(req *dto.CreatePermissionRequest) *entity.Permission {
	return &entity.Permission{
		RoleID:      req.RoleID,
		ResourceID:  req.ResourceID,
		Description: req.Description,
		Scopes:      req.Scopes,
	}
}
