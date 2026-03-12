package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/adapters/driven/db/ent/generate"
	"github.com/huynhanx03/judgify/internal/core/entity"
)

// BuildCreatePermission builds the create mutation for Permission entity.
func BuildCreatePermission(ctx context.Context, e *entity.Permission) *generate.PermissionCreate {
	builder := global.EntClient.DB(ctx).Permission.Create().
		SetRoleID(e.RoleID).
		SetResourceID(e.ResourceID).
		SetScopes(e.Scopes)
	if e.Description != nil {
		builder.SetDescription(*e.Description)
	}
	return builder
}

// BuildUpdatePermission builds the update mutation for Permission entity.
func BuildUpdatePermission(ctx context.Context, e *entity.Permission) *generate.PermissionUpdateOne {
	builder := global.EntClient.DB(ctx).Permission.UpdateOneID(e.ID).
		SetRoleID(e.RoleID).
		SetResourceID(e.ResourceID).
		SetScopes(e.Scopes)
	if e.Description != nil {
		builder.SetDescription(*e.Description)
	}
	return builder
}
