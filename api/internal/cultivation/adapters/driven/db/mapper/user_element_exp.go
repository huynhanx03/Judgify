package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToUserElementExpEntity converts Ent UserElementExp model to domain entity.
func ToUserElementExpEntity(m *generate.UserElementExp) *entity.UserElementExp {
	if m == nil {
		return nil
	}
	return &entity.UserElementExp{
		ID:        m.ID,
		UserID:    m.UserID,
		ElementID: m.ElementID,
		Exp:       m.Exp,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
