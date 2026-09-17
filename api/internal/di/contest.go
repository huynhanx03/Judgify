package di

import (
	"fmt"

	"github.com/huynhanx03/go-common/pkg/common/tx"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db"
	contestHttp "github.com/huynhanx03/judgify/internal/contest/adapters/driver/http"
	service "github.com/huynhanx03/judgify/internal/contest/usecase"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	problemPorts "github.com/huynhanx03/judgify/internal/ports"
)

// ContestContainer holds the composed Contest domain dependencies.
type ContestContainer struct {
	ContestHandlerGroup  *contestHttp.ContestHandlerGroup
	ContestRepo          ports.ContestRepository
	RegistrationRepo     ports.RegistrationRepository
	StandingRepo         ports.StandingRepository
	RatingHistoryRepo    ports.RatingHistoryRepository
	RatingSettlementRepo ports.RatingSettlementRepository
	RatingLedgerRepo     ports.RatingLedgerRepository
	RatingReratingRepo   ports.RatingReratingRepository
	ContestService       ports.ContestService
	RegistrationService  ports.RegistrationService
	LifecycleService     ports.LifecycleService
	RegistrationCommands ports.RegistrationCommandService
	SubmissionReserver   ports.ContestSubmissionReserver
	StandingService      ports.StandingService
	RatingService        ports.RatingService
	RatingRerating       *service.RatingReratingWorkflow
	ContentService       ports.ContentService
	Orchestrator         *service.ContestOrchestrator
}

// NewContestContainer creates a new ContestContainer.
func NewContestContainer(
	client *internalent.EntClient,
	logger *commonlogger.LoggerZap,
	txMgr tx.Manager,
	immutableProblems problemPorts.ImmutableProblemReader,
	operations *operation.Components,
	operationRegistry *operation.Registry,
) (*ContestContainer, error) {
	if client == nil || client.Client() == nil || logger == nil || logger.Logger == nil ||
		operations == nil || operations.Repository == nil ||
		operations.Starter == nil || operationRegistry == nil {
		return nil, fmt.Errorf(
			"initialize contest domain: operation control plane is required",
		)
	}
	// Repositories
	contestRepo := db.NewContestRepository(client)
	regRepo := db.NewRegistrationRepository(client)
	standingRepo := db.NewStandingRepository(client)
	ratingHistoryRepo := db.NewRatingHistoryRepository(client)
	ratingSettlementRepo := db.NewRatingSettlementRepository(client)
	ratingLedgerRepo := db.NewRatingLedgerRepository(client)
	lifecycleRepo, err := db.NewLifecycleRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize contest lifecycle repository: %w", err)
	}
	registrationCommandRepo, err := db.NewRegistrationCommandRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize contest registration command repository: %w", err)
	}
	submissionReserver, err := db.NewSubmissionReservationRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize contest submission reservation repository: %w", err)
	}
	contentRepo, err := db.NewContentRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize contest content repository: %w", err)
	}
	renderContent, err := contentrender.NewEntRepository(client.Client())
	if err != nil {
		return nil, fmt.Errorf("initialize contest render repository: %w", err)
	}
	renderer, err := contentrender.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("initialize contest content renderer: %w", err)
	}

	commandDependencies := service.LifecycleCommandDependencies{
		Transactions: platformtx.NewEntUnitOfWork(client.Client()), Repository: lifecycleRepo,
		Audit: audit.NewRepository(), Outbox: eventing.NewOutbox(),
	}

	// Services
	contestService := service.NewContestServiceWithJournal(
		contestRepo,
		regRepo,
		immutableProblems,
		mutation.NewJournal(
			commandDependencies.Transactions,
			commandDependencies.Audit,
			commandDependencies.Outbox,
		),
	)
	regService := service.NewRegistrationService(regRepo)
	standingService := service.NewStandingService(standingRepo, contestRepo)
	ratingSvc := service.NewRatingService(
		ratingLedgerRepo,
		txMgr,
		commandDependencies.Outbox,
	)
	ratingRerating, err := service.NewRatingReratingWorkflow(
		service.RatingReratingDependencies{
			Repository:   ratingLedgerRepo,
			Operations:   operations.Repository,
			Starter:      operations.Starter,
			Transactions: commandDependencies.Transactions,
			Outbox:       commandDependencies.Outbox,
		},
		service.RatingReratingConfig{},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"initialize historical rating rebuild: %w",
			err,
		)
	}
	if err := ratingRerating.Register(operationRegistry); err != nil {
		return nil, fmt.Errorf(
			"register historical rating rebuild: %w",
			err,
		)
	}
	lifecycleService, err := service.NewLifecycleCommandService(commandDependencies)
	if err != nil {
		return nil, fmt.Errorf("initialize contest lifecycle service: %w", err)
	}
	registrationCommandService, err := service.NewRegistrationCommandService(service.RegistrationCommandDependencies{
		Transactions: commandDependencies.Transactions, Repository: registrationCommandRepo,
		Audit: commandDependencies.Audit, Outbox: commandDependencies.Outbox,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize contest registration command service: %w", err)
	}
	contentService, err := service.NewContentService(service.ContentDependencies{Transactions: commandDependencies.Transactions, Repository: contentRepo, Renderer: renderer, Content: renderContent, Audit: commandDependencies.Audit, Outbox: commandDependencies.Outbox})
	if err != nil {
		return nil, fmt.Errorf("initialize contest content service: %w", err)
	}

	// Orchestrator
	orchestrator := service.NewContestOrchestrator(
		contestRepo,
		ratingSvc,
		ratingSettlementRepo,
		logger.Named("contest-orchestrator"),
		lifecycleService,
	)

	// Handlers
	handlerGroup := &contestHttp.ContestHandlerGroup{
		ContestHandler: contestHttp.NewContestHandler(
			contestService,
			lifecycleService,
		),
		RegistrationHandler: contestHttp.NewRegistrationHandler(
			registrationCommandService,
		),
		StandingHandler: contestHttp.NewStandingHandler(standingService),
		RatingHandler: contestHttp.NewRatingHandler(
			ratingSvc,
			ratingRerating,
		),
		ContentHandler: contestHttp.NewContentHandler(contentService),
	}

	return &ContestContainer{
		ContestHandlerGroup:  handlerGroup,
		ContestRepo:          contestRepo,
		RegistrationRepo:     regRepo,
		StandingRepo:         standingRepo,
		RatingHistoryRepo:    ratingHistoryRepo,
		RatingSettlementRepo: ratingSettlementRepo,
		RatingLedgerRepo:     ratingLedgerRepo,
		RatingReratingRepo:   ratingLedgerRepo,
		ContestService:       contestService,
		RegistrationService:  regService,
		LifecycleService:     lifecycleService,
		RegistrationCommands: registrationCommandService,
		SubmissionReserver:   submissionReserver,
		StandingService:      standingService,
		RatingService:        ratingSvc,
		RatingRerating:       ratingRerating,
		ContentService:       contentService,
		Orchestrator:         orchestrator,
	}, nil
}
