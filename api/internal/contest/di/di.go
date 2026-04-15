package di

import (
	"github.com/huynhanx03/judgify/global"
	contestHttp "github.com/huynhanx03/judgify/internal/contest/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/contest/core/service"
	"github.com/huynhanx03/judgify/internal/contest/ports"
)

// ContestContainer holds all dependencies for the contest domain.
type ContestContainer struct {
	ContestHandlerGroup  *contestHttp.ContestHandlerGroup
	ContestRepo          ports.ContestRepository
	RegistrationRepo     ports.RegistrationRepository
	StandingRepo         ports.StandingRepository
	ContestService       ports.ContestService
	RegistrationService  ports.RegistrationService
	StandingService      ports.StandingService
	Orchestrator         *service.ContestOrchestrator
}

// NewContestContainer creates a new ContestContainer.
func NewContestContainer() *ContestContainer {
	client := global.EntClient

	// Repositories
	contestRepo := db.NewContestRepository(client)
	regRepo := db.NewRegistrationRepository(client)
	standingRepo := db.NewStandingRepository(client)

	// Services
	contestService := service.NewContestService(contestRepo, regRepo)
	regService := service.NewRegistrationService(regRepo, contestRepo)
	standingService := service.NewStandingService(standingRepo)

	// Orchestrator
	orchestrator := service.NewContestOrchestrator(contestRepo)

	// Handlers
	handlerGroup := &contestHttp.ContestHandlerGroup{
		ContestHandler:      contestHttp.NewContestHandler(contestService),
		RegistrationHandler: contestHttp.NewRegistrationHandler(regService),
		StandingHandler:     contestHttp.NewStandingHandler(standingService),
	}

	return &ContestContainer{
		ContestHandlerGroup:  handlerGroup,
		ContestRepo:          contestRepo,
		RegistrationRepo:     regRepo,
		StandingRepo:         standingRepo,
		ContestService:       contestService,
		RegistrationService:  regService,
		StandingService:      standingService,
		Orchestrator:         orchestrator,
	}
}
