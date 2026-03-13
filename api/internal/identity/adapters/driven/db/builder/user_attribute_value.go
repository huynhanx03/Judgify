package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// BuildCreateUserAttributeValue builds the create mutation for UserAttributeValue entity.
func BuildCreateUserAttributeValue(ctx context.Context, e *entity.UserAttributeValue) *generate.UserAttributeValueCreate {
	builder := global.EntClient.DB(ctx).UserAttributeValue.Create().
		SetUserID(e.UserID).
		SetAttributeID(e.AttributeID).
		SetValue(e.Value)

	if e.ID != 0 {
		builder.SetID(e.ID)
	}

	return builder
}

// BuildUpdateUserAttributeValue builds the update mutation for UserAttributeValue entity.
func BuildUpdateUserAttributeValue(ctx context.Context, e *entity.UserAttributeValue) *generate.UserAttributeValueUpdateOne {
	return global.EntClient.DB(ctx).UserAttributeValue.UpdateOneID(e.ID).
		SetUserID(e.UserID).
		SetAttributeID(e.AttributeID).
		SetValue(e.Value)
}
