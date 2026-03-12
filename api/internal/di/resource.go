package di

import (
	db "github.com/huynhanx03/judgify/internal/adapters/driven/db"
	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"
	driverHttp "github.com/huynhanx03/judgify/internal/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/core/service"
	"github.com/huynhanx03/judgify/internal/ports"
)

// ResourceContainer holds resource-related dependencies.
type ResourceContainer struct {
	Repository ports.ResourceRepository
	Service    ports.ResourceService
	Handler    driverHttp.ResourceHandler
}

// InitResourceDependencies initializes resource dependencies.
func InitResourceDependencies(
	client *dbEnt.EntClient,
	cacheService ports.CacheService,
) ResourceContainer {
	repository := db.NewResourceRepository(client)
	service := service.NewResourceService(repository, cacheService)
	handler := driverHttp.NewResourceHandler(service)

	return ResourceContainer{
		Repository: repository,
		Service:    service,
		Handler:    handler,
	}
}
