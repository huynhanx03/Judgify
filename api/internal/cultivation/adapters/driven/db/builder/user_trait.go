package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateUserTrait builds the create mutation for UserTrait.
func BuildCreateUserTrait(ctx context.Context, e *entity.UserTrait) *generate.UserTraitCreate {
	b := global.EntClient.DB(ctx).UserTrait.Create().
		SetUserID(e.UserID).
		SetTraitID(e.TraitID)

	if e.ID != 0 {
		b.SetID(e.ID)
	}
	return b
}
