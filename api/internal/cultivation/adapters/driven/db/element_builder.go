package db

import (
	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateElement builds the create mutation for Element.
func BuildCreateElement(client *generate.Client, e *entity.Element) *generate.ElementCreate {
	b := client.Element.Create().
		SetName(e.Name).
		SetCode(e.Code).
		SetDescription(e.Description).
		SetVersion(1)

	if e.ID != uuid.Nil {
		b.SetID(e.ID)
	}
	return b
}

// BuildUpdateElement builds the update mutation for Element.
func BuildUpdateElement(client *generate.Client, e *entity.Element, nextVersion int64) *generate.ElementUpdateOne {
	return client.Element.UpdateOneID(e.ID).
		SetName(e.Name).
		SetCode(e.Code).
		SetDescription(e.Description).
		SetVersion(nextVersion)
}
