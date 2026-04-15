package di

import (
	"github.com/huynhanx03/judgify/global"
	contestHttp "github.com/huynhanx03/judgify/internal/contest/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/contest/core/service"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	"github.com/huynhanx03/judgify/internal/contest/store"
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
	Hub                  *store.LeaderboardHub
}

// NewContestContainer creates a new ContestContainer.
func NewContestContainer() *ContestContainer {
	client := global.EntClient

	// SSE Hub
	hub := store.NewLeaderboardHub()

	// Repositories
	contestRepo := db.NewContestRepository(client)
	regRepo := db.NewRegistrationRepository(client)
	standingRepo := db.NewStandingRepository(client)

	// Services
	contestService := service.NewContestService(contestRepo, regRepo)
	regService := service.NewRegistrationService(regRepo, contestRepo)
	standingService := service.NewStandingService(standingRepo, hub)

	// Orchestrator
	orchestrator := service.NewContestOrchestrator(contestRepo)

	// Handlers
	handlerGroup := &contestHttp.ContestHandlerGroup{
		ContestHandler:      contestHttp.NewContestHandler(contestService),
		RegistrationHandler: contestHttp.NewRegistrationHandler(regService),
		StandingHandler:     contestHttp.NewStandingHandler(standingService, hub),
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
		Hub:                  hub,
	}
}
