package di

import (
	"context"
	"crypto/rsa"
	"fmt"
	nethttp "net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/cache"
	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"
	"github.com/huynhanx03/go-common/pkg/common/lifecycle"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/go-common/pkg/oauth"
	authenticationjwt "github.com/huynhanx03/go-common/pkg/security/authentication/jwt"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	"github.com/huynhanx03/judgify/internal/application/workflow/attributeadmin"
	"github.com/huynhanx03/judgify/internal/application/workflow/authorizationadmin"
	"github.com/huynhanx03/judgify/internal/application/workflow/rolemetadata"
	"github.com/huynhanx03/judgify/internal/application/workflow/userinvitation"
	"github.com/huynhanx03/judgify/internal/application/workflow/userlifecycle"
	appcache "github.com/huynhanx03/judgify/internal/cache"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db"
	identityDelivery "github.com/huynhanx03/judgify/internal/identity/adapters/driven/delivery"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/securityjournal"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/identity/attributemigration"
	service "github.com/huynhanx03/judgify/internal/identity/usecase"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	"github.com/huynhanx03/judgify/internal/ports"
	rootports "github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/pkg/settings"
)

type RegistrationLifecycleFactory func() (rootports.RegistrationLifecycle, error)

// IdentityDependencies are the process-owned resources and immutable policy required
// by identity composition. The domain never reads ambient mutable state.
type IdentityDependencies struct {
	Database       *internalent.EntClient
	Cache          cache.LocalCache[string, any]
	Logger         *commonlogger.LoggerZap
	Authentication settings.Authentication
	JWT            settings.JWT
	AdminCommand   settings.AdminCommand
	Google         settings.Google
	GitHub         settings.GitHub
	Resend         settings.Resend
	SecurityEmail  settings.SecurityEmail
	Server         settings.Server
}

// IdentityContainer holds the composed Identity domain dependencies.
type IdentityContainer struct {
	IdentityHandler *http.IdentityHandler

	UserRepo                ports.UserRepository
	RoleRepo                ports.RoleRepository
	AttrDefRepo             ports.AttributeDefinitionRepository
	AttrValueRepo           ports.UserAttributeValueRepository
	ProfileSchemaRepo       ports.ProfileSchemaRepository
	PublicProfileRepo       ports.PublicProfileRepository
	CredentialRepo          ports.CredentialRepository
	FederatedIdRepo         ports.FederatedIdentityRepository
	OAuthDurableRepo        ports.OAuthDurableRepository
	SessionRepo             ports.SessionRepository
	RefreshTokenRepo        ports.RefreshTokenRepository
	AuthorizationService    ports.AuthorizationService
	AuthorizationPolicy     ports.AuthorizationPolicyRepository
	AuthorizationRuntime    lifecycle.Component
	AuthorizationHealth     commonhealth.Checker
	SessionStateCache       ports.SessionStateCache
	SessionManagement       *service.SessionManagementService
	SessionAuthenticator    *service.SessionAuthenticator
	TokenLifecycle          ports.TokenLifecycle
	PasswordReset           ports.PasswordResetLifecycle
	PasswordResetRepo       ports.PasswordResetRepository
	RecoveryContact         ports.RecoveryContactLifecycle
	RecoveryContactRepo     ports.RecoveryContactStore
	SecurityEmailRepo       ports.SecurityEmailDeliveryRepository
	SecurityEmailDispatcher *service.SecurityEmailDispatcher
	SecurityEmailHealth     commonhealth.Checker
	ReceiptCleanupRepo      ports.ReceiptCleanupRepository
	ReceiptJanitor          *service.ReceiptJanitor
	SecurityMutationJournal ports.SecurityMutationJournal

	AuthService          ports.AuthenticationService
	UserService          ports.UserService
	UserInvitation       ports.UserInvitationService
	ProfileSchemaService ports.ProfileSchemaService
	PublicProfileService ports.PublicProfileService
	CacheService         ports.CacheService
}

// NewIdentityContainer creates a new IdentityContainer.
func NewIdentityContainer(
	ctx context.Context,
	dependencies IdentityDependencies,
	txMgr tx.Manager,
	registrationFactory RegistrationLifecycleFactory,
	operationComponents *operation.Components,
	operationRegistry *operation.Registry,
) (*IdentityContainer, error) {
	if ctx == nil {
		return nil, fmt.Errorf("initialize identity: nil context")
	}
	if dependencies.Database == nil || dependencies.Database.Client() == nil ||
		dependencies.Cache == nil || dependencies.Logger == nil || dependencies.Logger.Logger == nil ||
		txMgr == nil || registrationFactory == nil || operationComponents == nil ||
		operationComponents.Starter == nil || operationRegistry == nil {
		return nil, fmt.Errorf("initialize identity: required application dependencies are unavailable")
	}
	client := dependencies.Database
	localCache := dependencies.Cache
	config := dependencies

	// Repositories
	userRepo := db.NewUserRepository(client)
	roleRepo := db.NewRoleRepository(client)
	attrDefRepo := db.NewAttributeDefinitionRepository(client)
	profileSchemaRepo := db.NewProfileSchemaRepository(client)
	if err := profileSchemaRepo.Ensure(ctx); err != nil {
		return nil, fmt.Errorf("initialize profile schema fence: %w", err)
	}
	publicProfileRepo, err := db.NewPublicProfileRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize public profile repository: %w", err)
	}
	attributeMigrationRepo := db.NewAttributeValueMigrationRepository(client)
	attrValueRepo := db.NewUserAttributeValueRepository(client)
	credentialRepo := db.NewCredentialRepository(client)
	federatedIdRepo := db.NewFederatedIdentityRepository(client)
	oauthDurableRepo := db.NewOAuthDurableRepository(client)
	sessionRepo := db.NewSessionRepository(client)
	refreshTokenRepo := db.NewRefreshTokenRepository(client)
	issuanceReceiptRepo := db.NewSessionIssuanceReceiptRepository(client)
	refreshRotationRepo := db.NewRefreshRotationRepository(client)
	passwordResetRepo := db.NewPasswordResetRepository(client)
	recoveryContactRepo := db.NewRecoveryContactRepository(client)
	securityEmailRepo := db.NewSecurityEmailRepository(client)
	receiptCleanupRepo := db.NewReceiptCleanupRepository(client)
	authorizationPolicyRepo := db.NewAuthorizationPolicyRepository(client)
	registrationLifecycle, err := registrationFactory()
	if err != nil {
		return nil, fmt.Errorf("initialize identity registration lifecycle: %w", err)
	}
	if registrationLifecycle == nil {
		return nil, fmt.Errorf("initialize identity registration lifecycle: nil lifecycle")
	}

	// Services
	cacheService := service.NewCacheService(localCache)
	auditRepository := audit.NewRepository()
	securityMutationJournal, err := securityjournal.New(
		auditRepository,
		eventing.NewOutbox(),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize identity security journal: %w", err)
	}
	sessionStateCache, err := service.NewSessionStateCache(service.SessionStateCacheConfig{})
	if err != nil {
		return nil, fmt.Errorf("initialize session state cache: %w", err)
	}
	sessionManagementRepository := db.NewSessionManagementRepository(client)
	sessionManagement, err := service.NewSessionManagementService(
		sessionManagementRepository,
		txMgr,
		securityMutationJournal,
		sessionStateCache,
		service.SessionManagementConfig{
			ReceiptTTL: config.Authentication.RecoveryCommandReceiptTTL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize session management: %w", err)
	}
	authorizationRuntime, err := service.NewAuthorizationService(
		ctx,
		authorizationPolicyRepo,
		txMgr,
		auditRepository,
		eventing.NewOutbox(),
		dependencies.Logger.Named("authorization"),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize authorization service: %w", err)
	}
	var authzService ports.AuthorizationService = authorizationRuntime
	accessIssuer, accessVerifier, err := accessTokenCodec(config.Authentication, config.JWT)
	if err != nil {
		return nil, err
	}
	sessionAuthenticator, err := service.NewSessionAuthenticator(
		accessVerifier,
		sessionRepo,
		sessionStateCache,
		service.SessionAuthenticatorConfig{},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize session authenticator: %w", err)
	}
	tokenLifecycle, err := service.NewTokenLifecycleService(
		accessIssuer,
		txMgr,
		sessionRepo,
		refreshTokenRepo,
		issuanceReceiptRepo,
		refreshRotationRepo,
		sessionAuthenticator,
		service.TokenLifecycleConfig{
			AccessTTL:              config.Authentication.AccessTTL,
			RefreshTTL:             config.Authentication.RefreshTTL,
			SessionIdleTTL:         config.Authentication.SessionIdleTTL,
			SessionAbsoluteTTL:     config.Authentication.SessionAbsoluteTTL,
			RecoveryReceiptTTL:     config.Authentication.RecoveryReceiptTTL,
			Secret:                 []byte(config.JWT.Secret),
			SecretKeyID:            config.Authentication.SecretKeyID,
			MaximumCredentialBytes: config.Authentication.MaximumCredentialBytes,
			MaximumUserAgentBytes:  config.Authentication.MaximumUserAgentBytes,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize token lifecycle: %w", err)
	}
	passwordResetDelivery, err := configuredPasswordResetDelivery(config.Resend, config.Server)
	if err != nil {
		return nil, err
	}
	securityEmailCodec, err := service.NewSecurityEmailPayloadCodec(
		service.SecurityEmailPayloadCodecConfig{
			ActiveKeyID: config.Authentication.SecretKeyID,
			Keys: map[string][]byte{
				config.Authentication.SecretKeyID: []byte(config.JWT.Secret),
			},
			MaximumCiphertextBytes: config.SecurityEmail.MaximumCiphertextBytes,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize security email payload codec: %w", err)
	}
	recoveryContactProtector, err := service.NewRecoveryContactProtector(
		service.RecoveryContactProtectorConfig{
			ActiveKeyID: config.Authentication.SecretKeyID,
			Keys: map[string][]byte{
				config.Authentication.SecretKeyID: []byte(config.JWT.Secret),
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize recovery contact protection: %w", err)
	}
	recoveryContact, err := service.NewRecoveryContactService(
		recoveryContactRepo,
		securityEmailRepo,
		securityEmailCodec,
		recoveryContactProtector,
		txMgr,
		securityMutationJournal,
		service.RecoveryContactConfig{
			VerificationTTL:         config.Authentication.RecoveryVerificationTTL,
			ReauthenticationTTL:     config.Authentication.RecoveryReauthenticationTTL,
			ResendCooldown:          config.Authentication.RecoveryResendCooldown,
			CommandReceiptTTL:       config.Authentication.RecoveryCommandReceiptTTL,
			ReplacementDailyLimit:   config.Authentication.RecoveryReplacementDailyLimit,
			VerificationHourlyLimit: config.Authentication.RecoveryVerificationHourlyLimit,
			VerificationDailyLimit:  config.Authentication.RecoveryVerificationDailyLimit,
			MaximumTokenBytes:       config.Authentication.MaximumCredentialBytes,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize recovery contact lifecycle: %w", err)
	}
	if err := recoveryContact.ConfigureManagement(credentialRepo, sessionRepo); err != nil {
		return nil, fmt.Errorf("initialize recovery contact management: %w", err)
	}
	passwordReset, err := service.NewPasswordResetService(
		passwordResetRepo,
		credentialRepo,
		tokenLifecycle,
		securityEmailRepo,
		securityEmailCodec,
		recoveryContactProtector,
		txMgr,
		securityMutationJournal,
		service.PasswordResetConfig{
			TTL:                   config.Authentication.PasswordResetTTL,
			MaximumBytes:          config.Authentication.MaximumCredentialBytes,
			ShortWindow:           config.Authentication.PasswordResetShortWindow,
			MaximumPerShortWindow: config.Authentication.PasswordResetShortLimit,
			MaximumPerDay:         config.Authentication.PasswordResetDailyLimit,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize password reset lifecycle: %w", err)
	}
	dispatcherID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("initialize security email dispatcher owner: %w", err)
	}
	securityEmailConfig := config.SecurityEmail
	securityEmailDispatcher, err := service.NewSecurityEmailDispatcher(
		securityEmailRepo,
		passwordResetDelivery,
		securityEmailCodec,
		txMgr,
		service.SecurityEmailDispatcherConfig{
			Owner:           fmt.Sprintf("%s/%s", securityEmailConfig.OwnerPrefix, dispatcherID),
			PollInterval:    securityEmailConfig.PollInterval,
			ClaimTimeout:    securityEmailConfig.ClaimTimeout,
			DeliveryTimeout: securityEmailConfig.DeliveryTimeout,
			FinalizeTimeout: securityEmailConfig.FinalizeTimeout,
			LeaseDuration:   securityEmailConfig.LeaseDuration,
			BatchSize:       securityEmailConfig.BatchSize,
			Concurrency:     securityEmailConfig.Concurrency,
			MaximumAttempts: securityEmailConfig.MaximumAttempts,
			RetryInitial:    securityEmailConfig.RetryInitial,
			RetryMaximum:    securityEmailConfig.RetryMaximum,
			RetryMultiplier: securityEmailConfig.RetryMultiplier,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize security email dispatcher: %w", err)
	}
	receiptJanitor, err := service.NewReceiptJanitor(
		receiptCleanupRepo,
		txMgr,
		service.ReceiptJanitorConfig{
			Interval:          config.Authentication.ReceiptCleanupInterval,
			Timeout:           config.Authentication.ReceiptCleanupTimeout,
			BatchSize:         config.Authentication.ReceiptCleanupBatch,
			MaxBatches:        config.Authentication.ReceiptCleanupBatches,
			TerminalRetention: config.Authentication.RecoveryTerminalRetention,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize receipt janitor: %w", err)
	}
	browserSession, err := browserSessionConfig(config.Authentication, config.JWT, config.Server)
	if err != nil {
		return nil, err
	}
	oauthProviders, err := configuredOAuthProviders(config.Google, config.GitHub, config.Server)
	if err != nil {
		return nil, err
	}
	oauthProtector, err := service.NewOAuthDurableProtector(service.OAuthDurableProtectorConfig{
		Secret: []byte(config.JWT.Secret),
		KeyID:  config.Authentication.SecretKeyID,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize durable oauth protection: %w", err)
	}
	profileSchemaService := service.NewProfileSchemaService(profileSchemaRepo, txMgr)
	userInvitationService, err := service.NewUserInvitationService(
		userRepo,
		credentialRepo,
		roleRepo,
		authzService,
		attrValueRepo,
		profileSchemaService,
		recoveryContact,
		recoveryContactRepo,
		securityEmailRepo,
		recoveryContactProtector,
		txMgr,
		auditRepository,
		eventing.NewOutbox(),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize user invitation service: %w", err)
	}
	caches := appcache.New(localCache)
	authService := service.NewAuthenticationService(
		userRepo,
		credentialRepo,
		roleRepo,
		authzService,
		attrValueRepo,
		federatedIdRepo,
		oauthProviders,
		caches.IdentityRole,
		localCache,
		cacheService,
		txMgr,
		service.WithTokenLifecycle(tokenLifecycle),
		service.WithPasswordResetLifecycle(passwordReset),
		service.WithRegistrationLifecycle(registrationLifecycle),
		service.WithProfileSchema(profileSchemaService),
		service.WithRecoveryContactLifecycle(recoveryContact),
		service.WithSecurityMutationJournal(securityMutationJournal),
		service.WithOAuthDurableLifecycle(oauthDurableRepo, oauthProtector),
	)
	roleService := service.NewRoleService(
		roleRepo, authzService, cacheService, txMgr, caches.IdentityRole,
		service.WithRoleLifecycle(auditRepository, eventing.NewOutbox()),
	)
	authorizationConfirmations, err := admincommand.NewConfirmationProtector(
		[]byte(config.JWT.Secret),
		config.AdminCommand.ConfirmationTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize authorization confirmations: %w", err)
	}
	authorizationWorkflow, err := authorizationadmin.New(
		roleService,
		userRepo,
		authorizationRuntime,
		authorizationConfirmations,
		authorizationadmin.Config{
			RecentAuthenticationTTL: config.AdminCommand.RecentAuthenticationTTL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize authorization workflow: %w", err)
	}
	roleMetadataWorkflow, err := rolemetadata.New(
		roleService,
		authorizationConfirmations,
		rolemetadata.Config{
			RecentAuthenticationTTL: config.AdminCommand.RecentAuthenticationTTL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize role metadata workflow: %w", err)
	}
	attrDefService := service.NewAttributeDefinitionService(
		attrDefRepo,
		caches.IdentityAttributeDefinition,
		service.WithAttributeDefinitionLifecycle(
			txMgr,
			auditRepository,
			eventing.NewOutbox(),
		),
		service.WithAttributeDefinitionMigration(
			attributeMigrationRepo,
			operationComponents.Starter,
			attributemigration.DefaultBatchSize,
		),
		service.WithAttributeDefinitionProfileSchema(profileSchemaRepo),
	)
	attributeMigrationExecutor, err := attributemigration.NewExecutor(
		attributeMigrationRepo,
		attrDefService,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize attribute migration executor: %w", err)
	}
	if err := attributeMigrationExecutor.Register(operationRegistry); err != nil {
		return nil, fmt.Errorf("register attribute migration executor: %w", err)
	}
	attributeWorkflow, err := attributeadmin.New(
		attrDefService,
		authorizationConfirmations,
		attributeadmin.Config{
			RecentAuthenticationTTL: config.AdminCommand.RecentAuthenticationTTL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize attribute definition workflow: %w", err)
	}
	userService := service.NewUserService(
		userRepo,
		roleRepo,
		authzService,
		tokenLifecycle,
		attrValueRepo,
		txMgr,
		service.WithUserLifecycle(
			auditRepository,
			eventing.NewOutbox(),
			recoveryContactRepo,
		),
	)
	userLifecycleWorkflow, err := userlifecycle.New(
		userService,
		authorizationConfirmations,
		userlifecycle.Config{
			RecentAuthenticationTTL: config.AdminCommand.RecentAuthenticationTTL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize user lifecycle workflow: %w", err)
	}
	userInvitationWorkflow, err := userinvitation.New(
		userInvitationService,
		authorizationConfirmations,
		userinvitation.Config{
			RecentAuthenticationTTL: config.AdminCommand.RecentAuthenticationTTL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize user invitation workflow: %w", err)
	}
	publicProfileService, err := service.NewPublicProfileService(
		publicProfileRepo,
		internalent.NewReadSnapshotTxManager(client),
		txMgr,
		auditRepository,
		eventing.NewOutbox(),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize public profile service: %w", err)
	}
	publicProfileHandler, err := http.NewPublicProfileHandler(publicProfileService)
	if err != nil {
		return nil, fmt.Errorf("initialize public profile handler: %w", err)
	}

	// Handlers
	identityHandler := &http.IdentityHandler{
		AuthorizationHandler:       http.NewAuthorizationHandler(authorizationRuntime),
		RoleHandler:                http.NewRoleHandler(roleService, authorizationWorkflow, roleMetadataWorkflow),
		AttributeDefinitionHandler: http.NewAttributeDefinitionHandler(attrDefService, attributeWorkflow),
		AuthenticationHandler:      http.NewAuthenticationHandler(authService),
		SessionManagementHandler:   http.NewSessionManagementHandler(sessionManagement),
		RecoveryContactHandler:     http.NewRecoveryContactHandler(recoveryContact),
		UserHandler: http.NewUserHandler(
			userService,
			authorizationWorkflow,
			userLifecycleWorkflow,
			userInvitationWorkflow,
		),
		ProfileSchemaHandler: http.NewProfileSchemaHandler(profileSchemaService),
		PublicProfileHandler: publicProfileHandler,
		UserInvitation:       userInvitationService,
		BrowserSession:       browserSession,
	}

	return &IdentityContainer{
		IdentityHandler: identityHandler,

		UserRepo:                userRepo,
		RoleRepo:                roleRepo,
		AttrDefRepo:             attrDefRepo,
		AttrValueRepo:           attrValueRepo,
		ProfileSchemaRepo:       profileSchemaRepo,
		PublicProfileRepo:       publicProfileRepo,
		CredentialRepo:          credentialRepo,
		FederatedIdRepo:         federatedIdRepo,
		OAuthDurableRepo:        oauthDurableRepo,
		SessionRepo:             sessionRepo,
		RefreshTokenRepo:        refreshTokenRepo,
		AuthorizationService:    authzService,
		AuthorizationPolicy:     authorizationPolicyRepo,
		AuthorizationRuntime:    authorizationRuntime,
		AuthorizationHealth:     authorizationRuntime,
		SessionStateCache:       sessionStateCache,
		SessionManagement:       sessionManagement,
		SessionAuthenticator:    sessionAuthenticator,
		TokenLifecycle:          tokenLifecycle,
		PasswordReset:           passwordReset,
		PasswordResetRepo:       passwordResetRepo,
		RecoveryContact:         recoveryContact,
		RecoveryContactRepo:     recoveryContactRepo,
		SecurityEmailRepo:       securityEmailRepo,
		SecurityEmailDispatcher: securityEmailDispatcher,
		SecurityEmailHealth:     securityEmailDispatcher,
		ReceiptCleanupRepo:      receiptCleanupRepo,
		ReceiptJanitor:          receiptJanitor,
		SecurityMutationJournal: securityMutationJournal,

		AuthService:          authService,
		UserService:          userService,
		UserInvitation:       userInvitationService,
		ProfileSchemaService: profileSchemaService,
		PublicProfileService: publicProfileService,
		CacheService:         cacheService,
	}, nil
}

func browserSessionConfig(
	config settings.Authentication,
	jwtConfig settings.JWT,
	serverConfig settings.Server,
) (http.BrowserSessionConfig, error) {
	sameSite, err := parseSameSite(config.RefreshCookieSameSite)
	if err != nil {
		return http.BrowserSessionConfig{}, err
	}
	result, err := http.NewBrowserSessionConfig(http.BrowserSessionOptions{
		Access: http.BrowserCookiePolicy{
			Name: config.AccessCookieName, Path: "/", Secure: config.RefreshCookieSecure, SameSite: sameSite,
		},
		Refresh: http.BrowserCookiePolicy{
			Name: config.RefreshCookieName, Path: config.RefreshCookiePath, Secure: config.RefreshCookieSecure, SameSite: nethttp.SameSiteStrictMode,
		},
		CSRF: http.BrowserCookiePolicy{
			Name: config.CSRFCookieName, Path: "/", Secure: config.RefreshCookieSecure, SameSite: nethttp.SameSiteStrictMode,
		},
		PreSession: http.BrowserCookiePolicy{
			Name: config.PreSessionCookieName, Path: "/", Secure: config.RefreshCookieSecure, SameSite: nethttp.SameSiteStrictMode,
		},
		OAuthLoginTicket: http.BrowserCookiePolicy{
			Name: config.OAuthLoginTicketCookieName, Path: config.OAuthLoginTicketCookiePath, Secure: config.RefreshCookieSecure, SameSite: nethttp.SameSiteStrictMode,
		},
		OAuthLinkTicket: http.BrowserCookiePolicy{
			Name: config.OAuthLinkTicketCookieName, Path: config.OAuthLinkTicketCookiePath, Secure: config.RefreshCookieSecure, SameSite: nethttp.SameSiteStrictMode,
		},
		PreSessionTTL:          config.PreSessionTTL,
		OAuthCompletionURL:     config.OAuthCompletionURL,
		AllowedOrigins:         append([]string(nil), config.BrowserOrigins...),
		MaximumCredentialBytes: config.MaximumCredentialBytes,
		MaximumUserAgentBytes:  config.MaximumUserAgentBytes,
		Secret:                 []byte(jwtConfig.Secret),
	})
	if err != nil {
		return http.BrowserSessionConfig{}, fmt.Errorf("configure browser session: %w", err)
	}
	if !oauthDevelopmentMode(serverConfig) && !result.Access.Secure {
		return http.BrowserSessionConfig{}, fmt.Errorf("configure browser session: secure cookies are required outside development")
	}
	return result, nil
}

func accessTokenCodec(
	config settings.Authentication,
	jwtConfig settings.JWT,
) (*authenticationjwt.Issuer, *authenticationjwt.Verifier, error) {
	issuer, err := authenticationjwt.NewIssuer(authenticationjwt.IssuerOptions{
		Issuer:     config.Issuer,
		Audience:   config.Audience,
		KeyID:      config.SigningKeyID,
		PrivateKey: jwtConfig.PrivateKey,
		MaxTTL:     config.AccessTTL,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("initialize access token issuer: %w", err)
	}
	verifier, err := authenticationjwt.NewVerifier(
		authenticationjwt.KeySet{
			Active: map[string]*rsa.PublicKey{config.SigningKeyID: jwtConfig.PublicKey},
		},
		authenticationjwt.VerifierOptions{
			Issuer:   config.Issuer,
			Audience: config.Audience,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize access token verifier: %w", err)
	}
	return issuer, verifier, nil
}

func parseSameSite(value string) (nethttp.SameSite, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return nethttp.SameSiteStrictMode, nil
	case "lax":
		return nethttp.SameSiteLaxMode, nil
	default:
		return nethttp.SameSiteDefaultMode, fmt.Errorf("configure refresh cookie: same_site must be strict or lax")
	}
}

func configuredOAuthProviders(
	googleCfg settings.Google,
	githubCfg settings.GitHub,
	serverConfig settings.Server,
) (map[string]service.ConfiguredOAuthProvider, error) {
	providers := make(map[string]service.ConfiguredOAuthProvider)
	allowDevelopment := oauthDevelopmentMode(serverConfig)

	if isConfiguredSecret(googleCfg.ClientID) && isConfiguredSecret(googleCfg.ClientSecret) && isConfiguredSecret(googleCfg.RedirectURL) {
		provider, err := oauth.NewGoogleProvider(oauth.Options{
			ClientID:                 googleCfg.ClientID,
			ClientSecret:             googleCfg.ClientSecret,
			RedirectURI:              googleCfg.RedirectURL,
			HTTPClient:               &nethttp.Client{},
			AllowInsecureDevelopment: allowDevelopment,
		})
		if err != nil {
			return nil, fmt.Errorf("configure Google OAuth: %w", err)
		}
		providers[provider.Name()] = service.ConfiguredOAuthProvider{
			Provider:    provider,
			RedirectURI: googleCfg.RedirectURL,
		}
	}

	if isConfiguredSecret(githubCfg.ClientID) && isConfiguredSecret(githubCfg.ClientSecret) && isConfiguredSecret(githubCfg.RedirectURL) {
		provider, err := oauth.NewGitHubProvider(oauth.Options{
			ClientID:                 githubCfg.ClientID,
			ClientSecret:             githubCfg.ClientSecret,
			RedirectURI:              githubCfg.RedirectURL,
			HTTPClient:               &nethttp.Client{},
			AllowInsecureDevelopment: allowDevelopment,
		})
		if err != nil {
			return nil, fmt.Errorf("configure GitHub OAuth: %w", err)
		}
		providers[provider.Name()] = service.ConfiguredOAuthProvider{
			Provider:    provider,
			RedirectURI: githubCfg.RedirectURL,
		}
	}

	return providers, nil
}

func oauthDevelopmentMode(config settings.Server) bool {
	switch strings.ToLower(strings.TrimSpace(string(config.Mode))) {
	case "dev", "development", "test":
		return true
	default:
		return false
	}
}

func isConfiguredSecret(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.HasPrefix(value, "${")
}

func configuredPasswordResetDelivery(
	config settings.Resend,
	serverConfig settings.Server,
) (ports.SecurityEmailDelivery, error) {
	if isConfiguredSecret(config.APIKey) {
		delivery, err := identityDelivery.NewResendPasswordResetDelivery(
			identityDelivery.ResendPasswordResetConfig{
				APIKey: config.APIKey, FromEmail: config.FromEmail, FromName: config.FromName,
				ResetURL: config.ResetURL, VerificationURL: config.VerificationURL,
				InvitationURL:            config.InvitationURL,
				AllowInsecureDevelopment: oauthDevelopmentMode(serverConfig),
			},
		)
		if err != nil {
			return nil, fmt.Errorf("configure password reset delivery: %w", err)
		}
		return delivery, nil
	}
	if oauthDevelopmentMode(serverConfig) {
		return identityDelivery.DiscardPasswordResetDelivery{}, nil
	}
	return nil, fmt.Errorf("configure password reset delivery: Resend credentials are required outside development")
}
