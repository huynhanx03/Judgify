package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// ToFederatedIdentityEntity converts Ent FederatedIdentity model to domain entity.
func ToFederatedIdentityEntity(m *generate.FederatedIdentity) *entity.FederatedIdentity {
	if m == nil {
		return nil
	}
	return &entity.FederatedIdentity{
		ID:         m.ID,
		UserID:     m.UserID,
		Provider:   m.Provider,
		ExternalID: m.ExternalID,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

// ToFederatedIdentityModel converts domain entity to Ent FederatedIdentity model.
func ToFederatedIdentityModel(e *entity.FederatedIdentity) *generate.FederatedIdentity {
	if e == nil {
		return nil
	}
	return &generate.FederatedIdentity{
		ID:         e.ID,
		UserID:     e.UserID,
		Provider:   e.Provider,
		ExternalID: e.ExternalID,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
	}
}
