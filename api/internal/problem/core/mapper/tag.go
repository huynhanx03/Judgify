package mapper

import (
	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ToTagResponse converts Tag entity to TagResponse DTO.
func ToTagResponse(e *entity.Tag) *dto.TagResponse {
	if e == nil {
		return nil
	}
	elements := make([]*dto.TagElementInfo, len(e.Elements))
	for i, el := range e.Elements {
		elements[i] = &dto.TagElementInfo{ID: el.ID, Name: el.Name, Code: el.Code}
	}
	return &dto.TagResponse{
		ID:       e.ID,
		Name:     e.Name,
		Elements: elements,
	}
}

// ToTagEntityFromCreate converts CreateTagRequest to Tag entity.
func ToTagEntityFromCreate(req *dto.CreateTagRequest) *entity.Tag {
	return &entity.Tag{
		Name: req.Name,
	}
}
