package di

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"

	db "github.com/huynhanx03/judgify/internal/adapters/driven/db"
	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"
	driverHttp "github.com/huynhanx03/judgify/internal/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/core/service"
	"github.com/huynhanx03/judgify/internal/ports"
)

// AttributeDefinitionContainer holds attribute definition related dependencies.
type AttributeDefinitionContainer struct {
	Repository ports.AttributeDefinitionRepository
	Service    ports.AttributeDefinitionService
	Handler    driverHttp.AttributeDefinitionHandler
}

// InitAttributeDefinitionDependencies initializes attribute definition dependencies.
func InitAttributeDefinitionDependencies(
	client *dbEnt.EntClient,
	cache cache.LocalCache[string, any],
) AttributeDefinitionContainer {
	repository := db.NewAttributeDefinitionRepository(client)
	service := service.NewAttributeDefinitionService(repository, cache)
	handler := driverHttp.NewAttributeDefinitionHandler(service)

	return AttributeDefinitionContainer{
		Repository: repository,
		Service:    service,
		Handler:    handler,
	}
}
