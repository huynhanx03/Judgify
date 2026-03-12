package di

import (
	db "github.com/huynhanx03/judgify/internal/adapters/driven/db"
	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"
	"github.com/huynhanx03/judgify/internal/ports"
)

// FederatedIdentityContainer holds federated identity related dependencies.
type FederatedIdentityContainer struct {
	Repository ports.FederatedIdentityRepository
}

// InitFederatedIdentityDependencies initializes federated identity dependencies.
func InitFederatedIdentityDependencies(client *dbEnt.EntClient) FederatedIdentityContainer {
	repository := db.NewFederatedIdentityRepository(client)

	return FederatedIdentityContainer{
		Repository: repository,
	}
}
