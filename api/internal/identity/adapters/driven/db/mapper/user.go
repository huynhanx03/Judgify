package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// ToUserEntity converts Ent User model to domain entity.
func ToUserEntity(m *generate.User) *entity.User {
	if m == nil {
		return nil
	}
	u := &entity.User{
		ID:        m.ID,
		Username:  m.Username,
		RoleID:    m.RoleID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
	if m.Edges.Role != nil {
		u.RoleName = m.Edges.Role.Name
	}
	return u
}

// ToUserModel converts domain entity to Ent User model.
func ToUserModel(e *entity.User) *generate.User {
	if e == nil {
		return nil
	}
	return &generate.User{
		ID:        e.ID,
		Username:  e.Username,
		RoleID:    e.RoleID,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}
