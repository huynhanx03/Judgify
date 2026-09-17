package db

import (
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateUserElementExp builds the create mutation for UserElementExp.
func BuildCreateUserElementExp(client *generate.Client, e *entity.UserElementExp) *generate.UserElementExpCreate {
	return client.UserElementExp.Create().
		SetUserID(e.UserID).
		SetElementID(e.ElementID).
		SetExp(e.Exp)
}
