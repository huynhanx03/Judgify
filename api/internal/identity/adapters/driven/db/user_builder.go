package db

import (
	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// BuildCreateUser builds the create mutation for User entity.
func BuildCreateUser(client *generate.Client, e *entity.User) *generate.UserCreate {
	create := client.User.Create().SetUsername(e.Username)
	if e.ID != uuid.Nil {
		create.SetID(e.ID)
	}
	if e.SecurityStatus != "" {
		create.SetStatus(user.Status(e.SecurityStatus))
	}
	if e.SecurityVersion > 0 {
		create.SetSecurityVersion(int64(e.SecurityVersion))
	}
	return create
}
