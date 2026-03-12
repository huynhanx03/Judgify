package di

import (
	db "github.com/huynhanx03/judgify/internal/adapters/driven/db"
	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"
	driverHttp "github.com/huynhanx03/judgify/internal/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/core/service"
	"github.com/huynhanx03/judgify/internal/ports"
)

// PermissionContainer holds permission-related dependencies.
type PermissionContainer struct {
	Repository ports.PermissionRepository
	Service    ports.PermissionService
	Handler    driverHttp.PermissionHandler
}

// InitPermissionDependencies initializes permission dependencies.
func InitPermissionDependencies(
	client *dbEnt.EntClient,
	cacheService ports.CacheService,
) PermissionContainer {
	repository := db.NewPermissionRepository(client)
	service := service.NewPermissionService(repository, cacheService)
	handler := driverHttp.NewPermissionHandler(service)

	return PermissionContainer{
		Repository: repository,
		Service:    service,
		Handler:    handler,
	}
}
