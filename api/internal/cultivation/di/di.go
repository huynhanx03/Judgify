package di

import (
	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/cultivation/core/service"
)

// CultivationContainer holds all dependencies for the cultivation domain.
type CultivationContainer struct {
	CultivationHandlerGroup *http.CultivationHandlerGroup
}

// NewCultivationContainer creates a new CultivationContainer.
func NewCultivationContainer() *CultivationContainer {
	client := global.EntClient

	// Repositories
	elementRepo := db.NewElementRepository(client)
	traitRepo := db.NewTraitRepository(client)
	userTraitRepo := db.NewUserTraitRepository(client)
	userElementExpRepo := db.NewUserElementExpRepository(client)
	levelRepo := db.NewLevelRepository(client)
	rankRepo := db.NewRankRepository(client)
	userStatsRepo := db.NewUserStatsRepository(client)

	// Services
	elementService := service.NewElementService(elementRepo)
	traitService := service.NewTraitService(traitRepo)
	userTraitService := service.NewUserTraitService(userTraitRepo)
	userElementExpService := service.NewUserElementExpService(userElementExpRepo)
	levelService := service.NewLevelService(levelRepo)
	rankService := service.NewRankService(rankRepo)
	userStatsService := service.NewUserStatsService(userStatsRepo)

	// Handlers
	handlerGroup := &http.CultivationHandlerGroup{
		ElementHandler:        http.NewElementHandler(elementService),
		TraitHandler:          http.NewTraitHandler(traitService),
		UserTraitHandler:      http.NewUserTraitHandler(userTraitService),
		UserElementExpHandler: http.NewUserElementExpHandler(userElementExpService),
		LevelHandler:          http.NewLevelHandler(levelService),
		RankHandler:           http.NewRankHandler(rankService),
		UserStatsHandler:      http.NewUserStatsHandler(userStatsService),
	}

	return &CultivationContainer{
		CultivationHandlerGroup: handlerGroup,
	}
}
