package usecase

import (
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// ToAttributeDefinitionResponse converts AttributeDefinition entity to Response DTO.
func ToAttributeDefinitionResponse(e *entity.AttributeDefinition) *dto.AttributeDefinitionResponse {
	if e == nil {
		return nil
	}
	return &dto.AttributeDefinitionResponse{
		ID:                   e.ID,
		Key:                  e.Key,
		Kind:                 e.Kind,
		RequiredOnOnboarding: e.RequiredOnOnboarding,
		Status:               e.Status,
		Version:              e.Version,
		DraftRevisionID:      e.DraftRevisionID,
		ActiveRevisionID:     e.ActiveRevisionID,
		DraftRevision:        ToAttributeDefinitionRevisionResponse(e.DraftRevision),
		ActiveRevision:       ToAttributeDefinitionRevisionResponse(e.ActiveRevision),
	}
}

func ToAttributeDefinitionRevisionResponse(e *entity.AttributeDefinitionRevision) *dto.AttributeDefinitionRevisionResponse {
	if e == nil {
		return nil
	}
	return &dto.AttributeDefinitionRevisionResponse{
		ID:           e.ID,
		DefinitionID: e.DefinitionID,
		Revision:     e.Revision,
		DataType:     string(e.DataType),
		Label:        e.Label,
		Description:  e.Description,
		Visibility:   string(e.Visibility),
		UserEditable: e.UserEditable,
		DisplayOrder: e.DisplayOrder,
		Validation:   append([]byte(nil), e.Validation...),
		CreatedAt:    e.CreatedAt,
	}
}

// ToAttributeDefinitionEntityFromCreate converts CreateAttributeDefinitionRequest to entity.
func ToAttributeDefinitionEntityFromCreate(req *dto.CreateAttributeDefinitionRequest) *entity.AttributeDefinition {
	return &entity.AttributeDefinition{
		Key:                  req.Key,
		Kind:                 "custom",
		RequiredOnOnboarding: req.RequiredOnOnboarding,
		Status:               "active",
		Version:              1,
	}
}
