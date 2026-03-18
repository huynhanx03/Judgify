package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// BuildCreateElement builds the create mutation for Element.
func BuildCreateElement(ctx context.Context, e *entity.Element) *generate.ElementCreate {
	b := global.EntClient.DB(ctx).Element.Create().
		SetName(e.Name).
		SetCode(e.Code).
		SetDescription(e.Description).
		SetColor(e.Color).
		SetIcon(e.Icon).
		SetOrder(e.Order)

	if e.ID != 0 {
		b.SetID(e.ID)
	}
	return b
}

// BuildUpdateElement builds the update mutation for Element.
func BuildUpdateElement(ctx context.Context, e *entity.Element) *generate.ElementUpdateOne {
	return global.EntClient.DB(ctx).Element.UpdateOneID(e.ID).
		SetName(e.Name).
		SetCode(e.Code).
		SetDescription(e.Description).
		SetColor(e.Color).
		SetIcon(e.Icon).
		SetOrder(e.Order)
}
