package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// BuildCreateUser builds the create mutation for User entity.
func BuildCreateUser(ctx context.Context, e *entity.User) *generate.UserCreate {
	return global.EntClient.DB(ctx).User.Create().
		SetUsername(e.Username).
		SetRoleID(e.RoleID)
}

// BuildUpdateUser builds the update mutation for User entity.
func BuildUpdateUser(ctx context.Context, e *entity.User) *generate.UserUpdateOne {
	return global.EntClient.DB(ctx).User.UpdateOneID(e.ID).
		SetUsername(e.Username).
		SetRoleID(e.RoleID)
}
