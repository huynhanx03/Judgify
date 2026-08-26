package di

import (
	"github.com/huynhanx03/judgify/global"
	cultivationPorts "github.com/huynhanx03/judgify/internal/cultivation/ports"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/identity/core/service"
	"github.com/huynhanx03/judgify/internal/identity/ports"
	problemPorts "github.com/huynhanx03/judgify/internal/problem/ports"
	"github.com/huynhanx03/judgify/pkg/common/tx"
	"github.com/huynhanx03/judgify/pkg/oauth"
)

// IdentityContainer holds all dependencies for the identity domain.
type IdentityContainer struct {
	IdentityHandler *http.IdentityHandler

	UserRepo         ports.UserRepository
	RoleRepo         ports.RoleRepository
	PermissionRepo   ports.PermissionRepository
	ResourceRepo     ports.ResourceRepository
	AttrDefRepo      ports.AttributeDefinitionRepository
	AttrValueRepo    ports.UserAttributeValueRepository
	CredentialRepo   ports.CredentialRepository
	FederatedIdRepo  ports.FederatedIdentityRepository

	AuthService  ports.AuthenticationService
	UserService  ports.UserService
	CacheService ports.CacheService
}

// NewIdentityContainer creates a new IdentityContainer.
func NewIdentityContainer(
	userTraitRepo cultivationPorts.UserTraitRepository,
	userStatsRepo cultivationPorts.UserStatsRepository,
	userElementExpRepo cultivationPorts.UserElementExpRepository,
	elementRepo cultivationPorts.ElementRepository,
	levelRepo cultivationPorts.LevelRepository,
	rankRepo cultivationPorts.RankRepository,
	userDiffStatsRepo cultivationPorts.UserDifficultyStatsRepository,
	userTagStatsRepo cultivationPorts.UserTagStatsRepository,
	difficultyRepo problemPorts.DifficultyRepository,
	txMgr tx.Manager,
) *IdentityContainer {
	client := global.EntClient
	localCache := global.Ember

	// Repositories
	userRepo := db.NewUserRepository(client)
	roleRepo := db.NewRoleRepository(client)
	permRepo := db.NewPermissionRepository(client)
	resourceRepo := db.NewResourceRepository(client)
	attrDefRepo := db.NewAttributeDefinitionRepository(client)
	attrValueRepo := db.NewUserAttributeValueRepository(client)
	credentialRepo := db.NewCredentialRepository(client)
	federatedIdRepo := db.NewFederatedIdentityRepository(client)

	// OAuth Providers (can be extended)
	oauthProviders := make(map[string]oauth.Provider)

	// Services
	cacheService := service.NewCacheService(localCache)
	authService := service.NewAuthenticationService(
		userRepo,
		credentialRepo,
		roleRepo,
		permRepo,
		resourceRepo,
		attrDefRepo,
		attrValueRepo,
		federatedIdRepo,
		userTraitRepo,
		userStatsRepo,
		userElementExpRepo,
		elementRepo,
		userDiffStatsRepo,
		difficultyRepo,
		oauthProviders,
		localCache,
		cacheService,
		txMgr,
	)
	roleService := service.NewRoleService(roleRepo, cacheService, txMgr)
	permService := service.NewPermissionService(permRepo, cacheService)
	resourceService := service.NewResourceService(resourceRepo, cacheService)
	attrDefService := service.NewAttributeDefinitionService(attrDefRepo, localCache)
	userService := service.NewUserService(
		userRepo,
		credentialRepo,
		roleRepo,
		attrDefRepo,
		attrValueRepo,
		localCache,
		txMgr,
	)

	// Handlers
	profileHandler := http.NewProfileHandler(
		userService,
		userStatsRepo,
		userTraitRepo,
		userElementExpRepo,
		levelRepo,
		rankRepo,
		userDiffStatsRepo,
		userTagStatsRepo,
	)
	identityHandler := &http.IdentityHandler{
		RoleHandler:                http.NewRoleHandler(roleService),
		PermissionHandler:          http.NewPermissionHandler(permService),
		ResourceHandler:            http.NewResourceHandler(resourceService),
		AttributeDefinitionHandler: http.NewAttributeDefinitionHandler(attrDefService),
		AuthenticationHandler:      http.NewAuthenticationHandler(authService),
		UserHandler:                http.NewUserHandler(userService, authService),
		ProfileHandler:             profileHandler,
	}

	return &IdentityContainer{
		IdentityHandler: identityHandler,

		UserRepo:        userRepo,
		RoleRepo:        roleRepo,
		PermissionRepo:  permRepo,
		ResourceRepo:    resourceRepo,
		AttrDefRepo:     attrDefRepo,
		AttrValueRepo:   attrValueRepo,
		CredentialRepo:  credentialRepo,
		FederatedIdRepo: federatedIdRepo,

		AuthService:  authService,
		UserService:  userService,
		CacheService: cacheService,
	}
}
