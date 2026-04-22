package di

import (
	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/cultivation/core/service"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// CultivationContainer holds all dependencies for the cultivation domain.
type CultivationContainer struct {
	CultivationHandlerGroup *http.CultivationHandlerGroup
	UserTraitRepo           ports.UserTraitRepository
	GachaService            ports.GachaService
	RarityRepo              ports.RarityRepository
	UserStatsRepo           ports.UserStatsRepository
	UserElementExpRepo      ports.UserElementExpRepository
	ElementRepo             ports.ElementRepository
	LevelRepo               ports.LevelRepository
	RankRepo                ports.RankRepository
	UserDifficultyStatsRepo ports.UserDifficultyStatsRepository
	UserTagStatsRepo        ports.UserTagStatsRepository
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
	rarityRepo := db.NewRarityRepository(client)
	userDiffStatsRepo := db.NewUserDifficultyStatsRepository(client)
	userTagStatsRepo := db.NewUserTagStatsRepository(client)

	// Services
	elementService := service.NewElementService(elementRepo)
	traitService := service.NewTraitService(traitRepo)
	gachaService := service.NewGachaService(traitRepo)
	userTraitService := service.NewUserTraitService(userTraitRepo)
	userElementExpService := service.NewUserElementExpService(userElementExpRepo)
	levelService := service.NewLevelService(levelRepo)
	rankService := service.NewRankService(rankRepo)
	userStatsService := service.NewUserStatsService(userStatsRepo)
	rarityService := service.NewRarityService(rarityRepo)
	rankingService := service.NewRankingService(userStatsRepo, rankRepo, levelRepo, global.Ember)

	// Handlers
	handlerGroup := &http.CultivationHandlerGroup{
		ElementHandler:        http.NewElementHandler(elementService),
		TraitHandler:          http.NewTraitHandler(traitService),
		GachaHandler:          http.NewGachaHandler(gachaService),
		UserTraitHandler:      http.NewUserTraitHandler(userTraitService),
		UserElementExpHandler: http.NewUserElementExpHandler(userElementExpService),
		LevelHandler:          http.NewLevelHandler(levelService),
		RankHandler:           http.NewRankHandler(rankService),
		UserStatsHandler:      http.NewUserStatsHandler(userStatsService),
		RarityHandler:         http.NewRarityHandler(rarityService),
		RankingHandler:        http.NewRankingHandler(rankingService),
	}

	return &CultivationContainer{
		CultivationHandlerGroup: handlerGroup,
		UserTraitRepo:           userTraitRepo,
		GachaService:            gachaService,
		RarityRepo:              rarityRepo,
		UserStatsRepo:           userStatsRepo,
		UserElementExpRepo:      userElementExpRepo,
		ElementRepo:             elementRepo,
		LevelRepo:               levelRepo,
		RankRepo:                rankRepo,
		UserDifficultyStatsRepo: userDiffStatsRepo,
		UserTagStatsRepo:        userTagStatsRepo,
	}
}
