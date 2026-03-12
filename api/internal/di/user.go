package di

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"

	userDB "github.com/huynhanx03/judgify/internal/adapters/driven/db"
	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"
	"github.com/huynhanx03/judgify/internal/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/core/service"
	"github.com/huynhanx03/judgify/internal/ports"
)

type UserContainer struct {
	Repository ports.UserRepository
	Service    ports.UserService
	Handler    http.UserHandler
}

func InitUserRepository(client *dbEnt.EntClient) ports.UserRepository {
	return userDB.NewUserRepository(client)
}

func InitUserDependencies(
	client *dbEnt.EntClient,
	repo ports.UserRepository,
	authService ports.AuthenticationService,
	credentialRepo ports.CredentialRepository,
	roleRepo ports.RoleRepository,
	attrDefRepo ports.AttributeDefinitionRepository,
	attrValueRepo ports.UserAttributeValueRepository,
	localCache cache.LocalCache[string, any],
) UserContainer {
	svc := service.NewUserService(
		repo,
		credentialRepo,
		roleRepo,
		attrDefRepo,
		attrValueRepo,
		localCache,
	)
	handler := http.NewUserHandler(svc, authService)

	return UserContainer{
		Repository: repo,
		Service:    svc,
		Handler:    handler,
	}
}
