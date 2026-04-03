package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// BuildCreateTag builds the create mutation for Tag entity.
func BuildCreateTag(ctx context.Context, e *entity.Tag) *generate.TagCreate {
	c := global.EntClient.DB(ctx).Tag.Create().SetName(e.Name)
	if ids := e.ElementIDs(); len(ids) > 0 {
		c = c.AddElementIDs(ids...)
	}
	return c
}

// BuildUpdateTag builds the update mutation for Tag entity.
func BuildUpdateTag(ctx context.Context, e *entity.Tag) *generate.TagUpdateOne {
	u := global.EntClient.DB(ctx).Tag.UpdateOneID(e.ID).SetName(e.Name)
	u = u.ClearElements().AddElementIDs(e.ElementIDs()...)
	return u
}
