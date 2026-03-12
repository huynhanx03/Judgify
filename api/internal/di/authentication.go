package di

import (
	"github.com/huynhanx03/judgify/pkg/common/cache"

	"github.com/huynhanx03/judgify/global"
	driverHttp "github.com/huynhanx03/judgify/internal/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/core/service"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/pkg/oauth"
)

// AuthenticationContainer holds authentication dependencies.
type AuthenticationContainer struct {
	Service ports.AuthenticationService
	Handler driverHttp.AuthenticationHandler
}

// InitAuthenticationDependencies initializes authentication dependencies.
func InitAuthenticationDependencies(
	userRepo ports.UserRepository,
	credentialRepo ports.CredentialRepository,
	roleRepo ports.RoleRepository,
	permissionRepo ports.PermissionRepository,
	resourceRepo ports.ResourceRepository,
	attrDefinitionRepo ports.AttributeDefinitionRepository,
	attrValueRepo ports.UserAttributeValueRepository,
	fedIdentityRepo ports.FederatedIdentityRepository,
	cache cache.LocalCache[string, any],
	cacheService ports.CacheService,
) AuthenticationContainer {
	oauthProviders := map[string]oauth.Provider{
		"google": oauth.NewGoogleProvider(
			global.Config.Google.ClientID,
			global.Config.Google.ClientSecret,
			global.Config.Google.RedirectURL,
		),
	}

	service := service.NewAuthenticationService(
		userRepo,
		credentialRepo,
		roleRepo,
		permissionRepo,
		resourceRepo,
		attrDefinitionRepo,
		attrValueRepo,
		fedIdentityRepo,
		oauthProviders,
		cache,
		cacheService,
	)
	handler := driverHttp.NewAuthenticationHandler(service)

	return AuthenticationContainer{
		Service: service,
		Handler: handler,
	}
}
