package di

import (
	db "github.com/huynhanx03/judgify/internal/adapters/driven/db"
	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"
	"github.com/huynhanx03/judgify/internal/ports"
)

// UserAttributeValueContainer holds user attribute value related dependencies.
type UserAttributeValueContainer struct {
	Repository ports.UserAttributeValueRepository
}

// InitUserAttributeValueDependencies initializes user attribute value dependencies.
func InitUserAttributeValueDependencies(client *dbEnt.EntClient) UserAttributeValueContainer {
	repository := db.NewUserAttributeValueRepository(client)

	return UserAttributeValueContainer{
		Repository: repository,
	}
}
