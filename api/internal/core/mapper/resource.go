package mapper

import (
	"github.com/huynhanx03/judgify/internal/core/dto"
	"github.com/huynhanx03/judgify/internal/core/entity"
)

// ToResourceResponse converts Resource entity to ResourceResponse DTO.
func ToResourceResponse(e *entity.Resource) *dto.ResourceResponse {
	if e == nil {
		return nil
	}
	return &dto.ResourceResponse{
		ID:          e.ID,
		Key:         e.Key,
		Description: e.Description,
	}
}

// ToResourceEntityFromCreate converts CreateResourceRequest to Resource entity.
func ToResourceEntityFromCreate(req *dto.CreateResourceRequest) *entity.Resource {
	return &entity.Resource{
		Key:         req.Key,
		Description: req.Description,
	}
}
