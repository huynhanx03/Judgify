package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// ToUserAttributeValueEntity converts Ent UserAttributeValue model to domain entity.
func ToUserAttributeValueEntity(m *generate.UserAttributeValue) *entity.UserAttributeValue {
	if m == nil {
		return nil
	}
	return &entity.UserAttributeValue{
		ID:                   m.ID,
		UserID:               m.UserID,
		AttributeID:          m.AttributeID,
		Value:                m.Value,
		DefinitionRevisionID: m.DefinitionRevisionID,
		SourceRevisionID:     m.SourceRevisionID,
		CanonicalValue:       append([]byte(nil), m.CanonicalValue...),
		SourceValueVersion:   m.SourceValueVersion,
		Version:              m.Version,
		Definition:           ToAttributeDefinitionEntity(m.Edges.Definition),
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
	}
}
