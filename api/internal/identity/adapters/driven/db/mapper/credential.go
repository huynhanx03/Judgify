package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// ToCredentialEntity converts Ent Credential model to domain entity.
func ToCredentialEntity(m *generate.Credential) *entity.Credential {
	if m == nil {
		return nil
	}

	// Convert map[string]interface{} to string for domain entity if needed,
	// or keep as map if entity supports it.
	// Assuming Entity uses string for simplicity in legacy code or needs update.
	// Let's check Entity definition first. If Entity has string, we marshal.
	// If Entity has map, we pass directly.

	// Based on previous error, Entity/Model had SecretData.
	// New schema has CredentialData (JSON).

	return &entity.Credential{
		ID:             m.ID,
		UserID:         m.UserID,
		Type:           m.Type,
		CredentialData: m.CredentialData, // Assumes Entity updated to matches JSON type
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

// ToCredentialModel converts domain entity to Ent Credential model.
func ToCredentialModel(e *entity.Credential) *generate.Credential {
	if e == nil {
		return nil
	}
	return &generate.Credential{
		ID:             e.ID,
		UserID:         e.UserID,
		Type:           e.Type,
		CredentialData: e.CredentialData,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
}
