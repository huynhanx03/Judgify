package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// ToAttributeDefinitionEntity converts Ent AttributeDefinition model to domain entity.
func ToAttributeDefinitionEntity(m *generate.AttributeDefinition) *entity.AttributeDefinition {
	if m == nil {
		return nil
	}
	return &entity.AttributeDefinition{
		ID:                   m.ID,
		Key:                  m.Key,
		Kind:                 string(m.Kind),
		RequiredOnOnboarding: m.RequiredOnOnboarding,
		Status:               string(m.Status),
		Version:              m.Version,
		DraftRevisionID:      m.DraftRevisionID,
		ActiveRevisionID:     m.ActiveRevisionID,
		DraftRevision:        ToAttributeDefinitionRevisionEntity(m.Edges.DraftRevision),
		ActiveRevision:       ToAttributeDefinitionRevisionEntity(m.Edges.ActiveRevision),
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
	}
}

func ToAttributeDefinitionRevisionEntity(m *generate.AttributeDefinitionRevision) *entity.AttributeDefinitionRevision {
	if m == nil {
		return nil
	}
	return &entity.AttributeDefinitionRevision{
		ID:           m.ID,
		DefinitionID: m.DefinitionID,
		Revision:     m.Revision,
		DataType:     entity.AttributeValueType(m.DataType),
		Label:        m.Label,
		Description:  m.Description,
		Visibility:   entity.AttributeVisibility(m.Visibility),
		UserEditable: m.UserEditable,
		DisplayOrder: m.DisplayOrder,
		Validation:   append([]byte(nil), m.Validation...),
		CreatedBy:    m.CreatedBy,
		CreatedAt:    m.CreatedAt,
	}
}
