package di

import (
	db "github.com/huynhanx03/judgify/internal/adapters/driven/db"
	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"
	driverHttp "github.com/huynhanx03/judgify/internal/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/core/service"
	"github.com/huynhanx03/judgify/internal/ports"
)

// RoleContainer holds role-related dependencies.
type RoleContainer struct {
	Repository ports.RoleRepository
	Service    ports.RoleService
	Handler    driverHttp.RoleHandler
}

// InitRoleDependencies initializes role dependencies.
func InitRoleDependencies(
	client *dbEnt.EntClient,
	cacheService ports.CacheService,
) RoleContainer {
	repository := db.NewRoleRepository(client)
	service := service.NewRoleService(repository, cacheService)
	handler := driverHttp.NewRoleHandler(service)

	return RoleContainer{
		Repository: repository,
		Service:    service,
		Handler:    handler,
	}
}
