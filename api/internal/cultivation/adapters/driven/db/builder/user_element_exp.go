package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateUserElementExp builds the create mutation for UserElementExp.
func BuildCreateUserElementExp(ctx context.Context, e *entity.UserElementExp) *generate.UserElementExpCreate {
	b := global.EntClient.DB(ctx).UserElementExp.Create().
		SetUserID(e.UserID).
		SetElementID(e.ElementID).
		SetExp(e.Exp)

	if e.ID != 0 {
		b.SetID(e.ID)
	}
	return b
}

// BuildUpdateUserElementExp builds the update mutation for UserElementExp.
func BuildUpdateUserElementExp(ctx context.Context, e *entity.UserElementExp) *generate.UserElementExpUpdateOne {
	return global.EntClient.DB(ctx).UserElementExp.UpdateOneID(e.ID).
		SetExp(e.Exp)
}
