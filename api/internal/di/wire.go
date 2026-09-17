package di

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	commonobservability "github.com/huynhanx03/go-common/pkg/common/observability"

	debuglevel "github.com/huynhanx03/judgify/internal/application/workflow/debuglevel"
	contestdb "github.com/huynhanx03/judgify/internal/contest/adapters/driven/db"
	cultivationdb "github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db"
	internalEnt "github.com/huynhanx03/judgify/internal/ent"
	identitydb "github.com/huynhanx03/judgify/internal/identity/adapters/driven/db"
	identityhttp "github.com/huynhanx03/judgify/internal/identity/adapters/driver/http"
	judgeOperations "github.com/huynhanx03/judgify/internal/judge/operations"
	"github.com/huynhanx03/judgify/internal/onboarding"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	audithttp "github.com/huynhanx03/judgify/internal/platform/audit/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	operationhttp "github.com/huynhanx03/judgify/internal/platform/operation/adapters/driver/http"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	cultivationports "github.com/huynhanx03/judgify/internal/ports"
	identityports "github.com/huynhanx03/judgify/internal/ports"
	problemdb "github.com/huynhanx03/judgify/internal/problem/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/profile"
	profiledb "github.com/huynhanx03/judgify/internal/profile/adapters/driven/db"
	profilehttp "github.com/huynhanx03/judgify/internal/profile/adapters/driver/http"
	realtimeDi "github.com/huynhanx03/judgify/internal/realtime/di"
)

// SetupDependencies initializes API domain dependencies without starting
// goroutines or constructing worker-only Forge/Docker resources.
func SetupDependencies(ctx context.Context, dependencies Dependencies) (*Container, error) {
	if ctx == nil {
		return nil, fmt.Errorf("setup dependencies: nil context")
	}
	if err := dependencies.validate(); err != nil {
		return nil, err
	}
	database := dependencies.Database
	localCache := dependencies.Cache
	config := dependencies.Config
	logger := dependencies.Logger
	if err := onboarding.ValidateRegistrationRecoveryWindow(
		config.Authentication.TraitOfferTTL,
		config.Authentication.RecoveryReceiptTTL,
	); err != nil {
		return nil, fmt.Errorf("setup dependencies: registration recovery policy: %w", err)
	}
	operationComponents, err := operation.NewComponents(database)
	if err != nil {
		return nil, fmt.Errorf(
			"initialize operation control plane: %w",
			err,
		)
	}
	operationHandler, err := operationhttp.NewHandler(
		operationComponents.Repository,
		operationComponents.Controller,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize operation handler: %w", err)
	}
	operationRegistry := operation.NewRegistry()
	cultivationContainer, err := NewCultivationContainer(
		database,
		localCache,
		cultivationports.TraitOfferServiceOptions{
			MaximumOffers: 3,
			OfferTTL:      config.Authentication.TraitOfferTTL,
		},
		operationComponents,
		operationRegistry,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation domain: %w", err)
	}
	if err := cultivationdb.InstallDefaultRewardRule(ctx, database); err != nil {
		return nil, fmt.Errorf("initialize cultivation reward rule: %w", err)
	}

	// Create transaction manager backed by EntClient
	txMgr := internalEnt.NewTxManager(database)
	problemContainer, err := NewProblemContainer(
		ctx,
		database,
		localCache,
		txMgr,
		config.Judge,
		!config.Server.Mode.IsProd(),
	)
	if err != nil {
		return nil, err
	}
	identityContainer, err := NewIdentityContainer(
		ctx,
		IdentityDependencies{
			Database:       database,
			Cache:          localCache,
			Logger:         logger,
			Authentication: config.Authentication,
			JWT:            config.JWT,
			AdminCommand:   config.AdminCommand,
			Google:         config.Google,
			GitHub:         config.GitHub,
			Resend:         config.Resend,
			SecurityEmail:  config.SecurityEmail,
			Server:         config.Server,
		},
		txMgr,
		func() (identityports.RegistrationLifecycle, error) {
			return onboarding.NewRegistrationLifecycle(onboarding.RegistrationDependencies{
				OfferRepository: cultivationContainer.TraitOfferRepository,
				RewardProfiles:  cultivationContainer.RewardProfileRepository,
				UserStats:       cultivationContainer.UserStatsRepo,
				UserElementExp:  cultivationContainer.UserElementExpRepo,
				Elements:        cultivationContainer.ElementRepo,
				UserDifficulty:  cultivationContainer.UserDifficultyStatsRepo,
				Difficulties:    problemContainer.DifficultyRepo,
				Clock:           time.Now,
			})
		},
		operationComponents,
		operationRegistry,
	)
	if err != nil {
		return nil, err
	}
	identityContainer.IdentityHandler.OnboardingTraitOffer = identityhttp.OnboardingTraitOfferIssuer(
		func(
			requestContext context.Context,
			preSessionID uuid.UUID,
			idempotencyKey uuid.UUID,
		) (any, error) {
			return cultivationContainer.TraitOfferService.Create(
				requestContext,
				cultivationports.CreateOfferCommand{
					PreSessionID:   preSessionID,
					IdempotencyKey: idempotencyKey,
				},
			)
		},
	)
	profileSource, err := profiledb.NewSource(database)
	if err != nil {
		return nil, fmt.Errorf("initialize profile source: %w", err)
	}
	profileQuery, err := profile.NewQuery(
		identityContainer.UserService,
		profileSource,
		internalEnt.NewReadSnapshotTxManager(database),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize profile query: %w", err)
	}
	profileHandler, err := profilehttp.NewHandler(profileQuery)
	if err != nil {
		return nil, fmt.Errorf("initialize profile handler: %w", err)
	}
	identityContainer.IdentityHandler.ProfileHandler = profileHandler
	identityEligibility, err := identitydb.NewSubmissionEligibility(database)
	if err != nil {
		return nil, fmt.Errorf("initialize submission identity eligibility: %w", err)
	}
	contestReservation, err := contestdb.NewSubmissionReservationRepository(database)
	if err != nil {
		return nil, fmt.Errorf("initialize submission contest reservation: %w", err)
	}
	problemSnapshot, err := problemdb.NewSubmissionSnapshotResolver(database)
	if err != nil {
		return nil, fmt.Errorf("initialize submission problem snapshot: %w", err)
	}
	submissionContainer, err := NewSubmissionContainer(
		database,
		config.Judge,
		problemContainer.ReleaseID,
		problemContainer.ApprovedReleaseManifestChecksum,
		IntakeParticipants{
			Identity: identityEligibility,
			Contest:  contestReservation,
			Problem:  problemSnapshot,
		},
	)
	if err != nil {
		return nil, err
	}
	contestContainer, err := NewContestContainer(
		database,
		logger,
		txMgr,
		problemContainer.ImmutableReader,
		operationComponents,
		operationRegistry,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize contest domain: %w", err)
	}
	materialContainer, err := NewMaterialContainer(
		database,
		localCache,
		[]byte(config.JWT.Secret),
		config.AdminCommand,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize material domain: %w", err)
	}
	realtimeContainer, err := realtimeDi.NewRealtimeContainer(
		database,
		contestContainer.RegistrationService,
		identityContainer.AuthorizationService,
		config.Authentication.BrowserOrigins,
		config.Server.TrustedProxies,
		logger.Named("realtime"),
	)
	if err != nil {
		return nil, err
	}
	notificationContainer, err := NewNotificationContainer(
		database,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize notification domain: %w", err)
	}
	campaignContainer, err := NewCampaignContainer(
		database,
		notificationContainer.Service,
		identityContainer.AuthorizationService,
		operationComponents,
		operationRegistry,
		CampaignConfig{
			MaximumRecipients:       config.NotificationCampaign.MaximumRecipients,
			BatchSize:               config.NotificationCampaign.BatchSize,
			ConfirmationTTL:         config.NotificationCampaign.ConfirmationTTL,
			RecentAuthenticationTTL: config.NotificationCampaign.RecentAuthenticationTTL,
			ConfirmationSecret:      []byte(config.JWT.Secret),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize notification campaigns: %w", err)
	}
	debugWindow, err := commonobservability.NewDebugWindow(
		logger,
		config.Logger.LogLevel,
		config.Observability.DebugMaximumDuration,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize debug window: %w", err)
	}
	debugWorkflow, err := debuglevel.New(
		identityContainer.AuthorizationService,
		campaignContainer.Audience,
		platformtx.NewEntUnitOfWork(database.Client()),
		audit.NewRepository(),
		debugWindow,
		debuglevel.Config{
			Component:               apiDebugComponent,
			MaximumDuration:         config.Observability.DebugMaximumDuration,
			RecentAuthenticationTTL: config.Observability.DebugRecentAuthenticationTTL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize debug-level workflow: %w", err)
	}
	debugHandler, err := debuglevel.NewHTTPHandler(
		debugWorkflow,
		apiDebugComponent,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize debug-level HTTP handler: %w", err)
	}
	operationRunner, err := operation.NewRunner(
		operationComponents.Repository,
		operationComponents.Transactions,
		operationRegistry,
		operation.RunnerOptions{
			Owner:             localOperationOwner(config.Operation.OwnerPrefix),
			LeaseDuration:     config.Operation.LeaseDuration,
			BatchTimeout:      config.Operation.BatchTimeout,
			IdleWait:          config.Operation.IdleWait,
			RetryBaseDelay:    config.Operation.RetryInitial,
			RetryMaximumDelay: config.Operation.RetryMaximum,
			MaximumAttempts:   config.Operation.MaximumAttempts,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize operation runner: %w", err)
	}
	auditQuery, err := audit.NewQuery(database)
	if err != nil {
		return nil, fmt.Errorf("initialize audit query: %w", err)
	}
	auditHandler, err := audithttp.NewHandler(auditQuery)
	if err != nil {
		return nil, fmt.Errorf("initialize audit handler: %w", err)
	}
	judgeOperationsQuery, err := judgeOperations.NewQuery(
		database,
		internalEnt.NewReadSnapshotTxManager(database),
		config.Judge.WorkerHeartbeatFreshness,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize judge operations query: %w", err)
	}
	judgeOperationsHandler, err := judgeOperations.NewHTTPHandler(judgeOperationsQuery)
	if err != nil {
		return nil, fmt.Errorf("initialize judge operations handler: %w", err)
	}

	container := &Container{
		Audit:                auditHandler,
		Identity:             identityContainer,
		Judge:                judgeOperationsHandler,
		Problem:              problemContainer,
		Cultivation:          cultivationContainer,
		Submission:           submissionContainer,
		Contest:              contestContainer,
		Material:             materialContainer,
		Notification:         notificationContainer,
		NotificationCampaign: campaignContainer,
		Operation:            operationComponents,
		OperationHTTP:        operationHandler,
		OperationRunner:      operationRunner,
		Realtime:             realtimeContainer,
		DebugLevel:           debugHandler,
		DebugWindow:          debugWindow,
	}

	logger.Named("di").Info("Dependencies initialized successfully")
	return container, nil
}

const apiDebugComponent = "api"

func localOperationOwner(prefix string) string {
	prefix = canonicalOperationOwnerPart(prefix)
	if prefix == "" {
		prefix = "judgify-api"
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	hostname = canonicalOperationOwnerPart(hostname)
	if hostname == "" {
		hostname = "unknown-host"
	}
	owner := prefix + "." + hostname + "." + strconv.Itoa(os.Getpid())
	if len(owner) <= 128 {
		return owner
	}
	available := 128 - len(prefix) - len(strconv.Itoa(os.Getpid())) - 2
	if available < 1 {
		return ""
	}
	return prefix + "." + hostname[:min(available, len(hostname))] +
		"." + strconv.Itoa(os.Getpid())
}

func canonicalOperationOwnerPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	result.Grow(len(value))
	lastSeparator := false
	for _, character := range value {
		valid := character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '_' || character == '.'
		if valid {
			result.WriteRune(character)
			lastSeparator = false
			continue
		}
		if !lastSeparator && result.Len() > 0 {
			result.WriteByte('-')
			lastSeparator = true
		}
	}
	return strings.Trim(result.String(), "-.")
}
