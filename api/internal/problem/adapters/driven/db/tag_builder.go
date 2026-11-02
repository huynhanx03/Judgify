package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// BuildCreateTag builds the create mutation for Tag entity.
func BuildCreateTag(client *generate.Client, e *entity.Tag) *generate.TagCreate {
	c := client.Tag.Create().SetName(e.Name).SetVersion(1)
	if ids := e.ElementIDs(); len(ids) > 0 {
		c = c.AddElementIDs(ids...)
	}
	return c
}

// BuildUpdateTag builds the update mutation for Tag entity.
func BuildUpdateTag(client *generate.Client, e *entity.Tag, nextVersion int64) *generate.TagUpdateOne {
	u := client.Tag.UpdateOneID(e.ID).SetName(e.Name).SetVersion(nextVersion)
	u = u.ClearElements().AddElementIDs(e.ElementIDs()...)
	return u
}
