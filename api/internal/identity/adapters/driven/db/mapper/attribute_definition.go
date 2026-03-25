package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// ToAttributeDefinitionEntity converts Ent AttributeDefinition model to domain entity.
func ToAttributeDefinitionEntity(m *generate.AttributeDefinition) *entity.AttributeDefinition {
	if m == nil {
		return nil
	}
	return &entity.AttributeDefinition{
		ID:          m.ID,
		Key:         m.Key,
		DataType:    m.DataType,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// ToAttributeDefinitionModel converts domain entity to Ent AttributeDefinition model.
func ToAttributeDefinitionModel(e *entity.AttributeDefinition) *generate.AttributeDefinition {
	if e == nil {
		return nil
	}
	return &generate.AttributeDefinition{
		ID:          e.ID,
		Key:         e.Key,
		DataType:    e.DataType,
		Description: e.Description,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}
