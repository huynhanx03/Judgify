package mapper

import (
	"github.com/huynhanx03/judgify/internal/adapters/driven/db/ent/generate"
	"github.com/huynhanx03/judgify/internal/core/entity"
)

// ToUserAttributeValueEntity converts Ent UserAttributeValue model to domain entity.
func ToUserAttributeValueEntity(m *generate.UserAttributeValue) *entity.UserAttributeValue {
	if m == nil {
		return nil
	}
	return &entity.UserAttributeValue{
		ID:          m.ID,
		UserID:      m.UserID,
		AttributeID: m.AttributeID,
		Value:       m.Value,
		Definition:  ToAttributeDefinitionEntity(m.Edges.Definition),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// ToUserAttributeValueModel converts domain entity to Ent UserAttributeValue model.
func ToUserAttributeValueModel(e *entity.UserAttributeValue) *generate.UserAttributeValue {
	if e == nil {
		return nil
	}
	return &generate.UserAttributeValue{
		ID:          e.ID,
		UserID:      e.UserID,
		AttributeID: e.AttributeID,
		Value:       e.Value,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}
