package di

import (
	"github.com/huynhanx03/judgify/global"
)

// SetupDependencies initializes all dependencies and returns the container.
func SetupDependencies() *Container {
	client := global.EntClient

	credentialContainer := InitCredentialDependencies(client)
	cacheContainer := InitCacheDependencies(global.Tinylfu)
	roleContainer := InitRoleDependencies(client, cacheContainer.Service)
	attrValueContainer := InitUserAttributeValueDependencies(client)
	attrDefinitionContainer := InitAttributeDefinitionDependencies(client, global.Tinylfu)
	permissionContainer := InitPermissionDependencies(client, cacheContainer.Service)
	resourceContainer := InitResourceDependencies(client, cacheContainer.Service)
	federatedIdentityContainer := InitFederatedIdentityDependencies(client)

	userRepo := InitUserRepository(client)



	authContainer := InitAuthenticationDependencies(
		userRepo,
		credentialContainer.Repository,
		roleContainer.Repository,
		permissionContainer.Repository,
		resourceContainer.Repository,
		attrDefinitionContainer.Repository,
		attrValueContainer.Repository,
		federatedIdentityContainer.Repository,

		global.Tinylfu,
		cacheContainer.Service,
	)

	userContainer := InitUserDependencies(
		client,
		userRepo,
		authContainer.Service,
		credentialContainer.Repository,
		roleContainer.Repository,
		attrDefinitionContainer.Repository,
		attrValueContainer.Repository,
		global.Tinylfu,
	)

	container := &Container{
		RoleContainer:                roleContainer,
		PermissionContainer:          permissionContainer,
		ResourceContainer:            resourceContainer,
		AuthenticationContainer:      authContainer,
		UserContainer:                userContainer,
		CredentialContainer:          credentialContainer,
		FederatedIdentityContainer:   federatedIdentityContainer,
		AttributeDefinitionContainer: attrDefinitionContainer,
		UserAttributeValueContainer:  attrValueContainer,
	}

	GlobalContainer = container
	return container
}
