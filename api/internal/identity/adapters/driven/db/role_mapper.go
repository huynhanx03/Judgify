package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedrole "github.com/huynhanx03/judgify/internal/ent/generate/role"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// ToRoleEntity converts Ent Role model to domain entity.
func ToRoleEntity(m *generate.Role) *entity.Role {
	if m == nil {
		return nil
	}
	description := ""
	if m.Description != nil {
		description = *m.Description
	}
	return &entity.Role{
		ID:          m.ID,
		Key:         m.Key,
		Name:        m.Name,
		Description: description,
		IsSystem:    m.IsSystem,
		Status:      entity.RoleStatus(m.Status),
		Version:     m.Version,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// ToRoleModel converts domain entity to Ent Role model.
func ToRoleModel(e *entity.Role) *generate.Role {
	if e == nil {
		return nil
	}
	var description *string
	if e.Description != "" {
		description = &e.Description
	}
	return &generate.Role{
		ID:          e.ID,
		Key:         e.Key,
		Name:        e.Name,
		Description: description,
		IsSystem:    e.IsSystem,
		Status:      generatedrole.Status(e.Status),
		Version:     e.Version,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}
