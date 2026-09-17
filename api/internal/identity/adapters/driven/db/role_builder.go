package db

import (
	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedrole "github.com/huynhanx03/judgify/internal/ent/generate/role"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// BuildCreateRole builds the create mutation for Role entity.
func BuildCreateRole(client *generate.Client, e *entity.Role) *generate.RoleCreate {
	builder := client.Role.Create().
		SetKey(e.Key).
		SetName(e.Name).
		SetIsSystem(e.IsSystem)
	if e.Status.Valid() {
		builder.SetStatus(generatedrole.Status(e.Status))
	}
	if e.Version > 0 {
		builder.SetVersion(e.Version)
	}

	if e.Description != "" {
		builder.SetDescription(e.Description)
	}

	if e.ID != uuid.Nil {
		builder.SetID(e.ID)
	}

	return builder
}
