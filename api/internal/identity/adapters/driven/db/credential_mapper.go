package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// ToCredentialEntity converts Ent Credential model to domain entity.
func ToCredentialEntity(m *generate.Credential) *entity.Credential {
	if m == nil {
		return nil
	}

	return &entity.Credential{
		ID:             m.ID,
		UserID:         m.UserID,
		Type:           m.Type,
		CredentialData: m.CredentialData,
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
