package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToUserTraitEntity converts Ent UserTrait model to domain entity.
func ToUserTraitEntity(m *generate.UserTrait) *entity.UserTrait {
	if m == nil {
		return nil
	}
	return &entity.UserTrait{
		ID:        m.ID,
		UserID:    m.UserID,
		TraitID:   m.TraitID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
