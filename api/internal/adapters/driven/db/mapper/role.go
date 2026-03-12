package mapper

import (
	"github.com/huynhanx03/judgify/internal/adapters/driven/db/ent/generate"
	"github.com/huynhanx03/judgify/internal/core/entity"
)

// ToRoleEntity converts Ent Role model to domain entity.
func ToRoleEntity(m *generate.Role) *entity.Role {
	if m == nil {
		return nil
	}
	return &entity.Role{
		ID:        m.ID,
		Name:      m.Name,
		Level:     m.Level,
		ParentID:  m.ParentID,
		Lft:       m.Lft,
		Rgt:       m.Rgt,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// ToRoleModel converts domain entity to Ent Role model.
func ToRoleModel(e *entity.Role) *generate.Role {
	if e == nil {
		return nil
	}
	return &generate.Role{
		ID:        e.ID,
		Name:      e.Name,
		Level:     e.Level,
		ParentID:  e.ParentID,
		Lft:       e.Lft,
		Rgt:       e.Rgt,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}
