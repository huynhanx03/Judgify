package db

import (
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToRegistrationEntity converts Ent ContestRegistration model to domain entity.
func ToRegistrationEntity(m *generate.ContestRegistration) *entity.ContestRegistration {
	if m == nil {
		return nil
	}
	return &entity.ContestRegistration{
		ID:        m.ID,
		ContestID: m.ContestID,
		UserID:    m.UserID,
	}
}
