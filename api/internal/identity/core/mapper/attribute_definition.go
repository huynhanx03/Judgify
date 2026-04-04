package mapper

import (
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// ToAttributeDefinitionResponse converts AttributeDefinition entity to Response DTO.
func ToAttributeDefinitionResponse(e *entity.AttributeDefinition) *dto.AttributeDefinitionResponse {
	if e == nil {
		return nil
	}
	return &dto.AttributeDefinitionResponse{
		ID:          e.ID,
		Key:         e.Key,
		DataType:    e.DataType,
		Description: e.Description,
	}
}

// ToAttributeDefinitionEntityFromCreate converts CreateAttributeDefinitionRequest to entity.
func ToAttributeDefinitionEntityFromCreate(req *dto.CreateAttributeDefinitionRequest) *entity.AttributeDefinition {
	return &entity.AttributeDefinition{
		Key:         req.Key,
		DataType:    req.DataType,
		Description: req.Description,
	}
}
