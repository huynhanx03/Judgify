package di

import (
	"github.com/huynhanx03/judgify/global"
	contestHttp "github.com/huynhanx03/judgify/internal/contest/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/contest/core/service"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	"github.com/huynhanx03/judgify/internal/contest/store"
	cultivationPorts "github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// ContestContainer holds all dependencies for the contest domain.
type ContestContainer struct {
	ContestHandlerGroup  *contestHttp.ContestHandlerGroup
	ContestRepo          ports.ContestRepository
	RegistrationRepo     ports.RegistrationRepository
	StandingRepo         ports.StandingRepository
	RatingHistoryRepo    ports.RatingHistoryRepository
	ContestService       ports.ContestService
	RegistrationService  ports.RegistrationService
	StandingService      ports.StandingService
	RatingService        ports.RatingService
	Orchestrator         *service.ContestOrchestrator
	Hub                  *store.LeaderboardHub
}

// NewContestContainer creates a new ContestContainer.
func NewContestContainer(userStatsRepo cultivationPorts.UserStatsRepository) *ContestContainer {
	client := global.EntClient

	// SSE Hub
	hub := store.NewLeaderboardHub()

	// Repositories
	contestRepo := db.NewContestRepository(client)
	regRepo := db.NewRegistrationRepository(client)
	standingRepo := db.NewStandingRepository(client)
	ratingHistoryRepo := db.NewRatingHistoryRepository(client)

	// Services
	contestService := service.NewContestService(contestRepo, regRepo)
	regService := service.NewRegistrationService(regRepo, contestRepo)
	standingService := service.NewStandingService(standingRepo, hub)
	ratingSvc := service.NewRatingService(standingRepo, ratingHistoryRepo, userStatsRepo)

	// Orchestrator
	orchestrator := service.NewContestOrchestrator(contestRepo, ratingSvc)

	// Handlers
	handlerGroup := &contestHttp.ContestHandlerGroup{
		ContestHandler:      contestHttp.NewContestHandler(contestService),
		RegistrationHandler: contestHttp.NewRegistrationHandler(regService),
		StandingHandler:     contestHttp.NewStandingHandler(standingService, hub),
		RatingHandler:       contestHttp.NewRatingHandler(ratingSvc),
	}

	return &ContestContainer{
		ContestHandlerGroup:  handlerGroup,
		ContestRepo:          contestRepo,
		RegistrationRepo:     regRepo,
		StandingRepo:         standingRepo,
		RatingHistoryRepo:    ratingHistoryRepo,
		ContestService:       contestService,
		RegistrationService:  regService,
		StandingService:      standingService,
		RatingService:        ratingSvc,
		Orchestrator:         orchestrator,
		Hub:                  hub,
	}
}
