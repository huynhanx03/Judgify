package di

import (
	db "github.com/huynhanx03/judgify/internal/adapters/driven/db"
	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"
	"github.com/huynhanx03/judgify/internal/ports"
)

// CredentialContainer holds credential-related dependencies.
type CredentialContainer struct {
	Repository ports.CredentialRepository
}

// InitCredentialDependencies initializes credential dependencies.
func InitCredentialDependencies(client *dbEnt.EntClient) CredentialContainer {
	repository := db.NewCredentialRepository(client)

	return CredentialContainer{
		Repository: repository,
	}
}
