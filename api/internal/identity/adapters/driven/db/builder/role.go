package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// BuildCreateRole builds the create mutation for Role entity.
func BuildCreateRole(ctx context.Context, e *entity.Role) *generate.RoleCreate {
	builder := global.EntClient.DB(ctx).Role.Create().
		SetName(e.Name).
		SetLevel(e.Level).
		SetParentID(e.ParentID).
		SetLft(e.Lft).
		SetRgt(e.Rgt)

	if e.ID != 0 {
		builder.SetID(e.ID)
	}

	return builder
}

// BuildUpdateRole builds the update mutation for Role entity.
func BuildUpdateRole(ctx context.Context, e *entity.Role) *generate.RoleUpdateOne {
	return global.EntClient.DB(ctx).Role.UpdateOneID(e.ID).
		SetName(e.Name).
		SetLevel(e.Level).
		SetParentID(e.ParentID).
		SetLft(e.Lft).
		SetRgt(e.Rgt)
}
