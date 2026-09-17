package di

import (
	"fmt"

	"github.com/huynhanx03/go-common/pkg/common/cache"
	appcache "github.com/huynhanx03/judgify/internal/cache"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driver/http"
	service "github.com/huynhanx03/judgify/internal/cultivation/usecase"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

// CultivationContainer holds the composed Cultivation domain dependencies.
type CultivationContainer struct {
	CultivationHandlerGroup *http.CultivationHandlerGroup
	TraitOfferRepository    ports.TraitOfferRepository
	TraitOfferService       ports.TraitOfferService
	RewardProfileRepository ports.RewardProfileRepository
	UserStatsRepo           ports.UserStatsRepository
	UserElementExpRepo      ports.UserElementExpRepository
	ElementRepo             ports.ElementRepository
	UserDifficultyStatsRepo ports.UserDifficultyStatsRepository
}

// NewCultivationContainer creates a new CultivationContainer.
func NewCultivationContainer(
	client *internalent.EntClient,
	localCache cache.LocalCache[string, any],
	offerOptions ports.TraitOfferServiceOptions,
	operations *operation.Components,
	operationRegistry *operation.Registry,
) (*CultivationContainer, error) {
	if client == nil || client.Client() == nil || localCache == nil {
		return nil, fmt.Errorf("initialize cultivation domain: database and cache are required")
	}
	if operations == nil || operations.Repository == nil || operations.Starter == nil ||
		operations.Transactions == nil || operationRegistry == nil {
		return nil, fmt.Errorf("initialize cultivation domain: operation control plane is required")
	}
	// Repositories
	elementRepo := db.NewElementRepository(client)
	traitRepo := db.NewTraitRepository(client)
	traitEffectRepository, err := db.NewTraitEffectRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation trait effect repository: %w", err)
	}
	userTraitRepo := db.NewUserTraitRepository(client)
	userElementExpRepo := db.NewUserElementExpRepository(client)
	levelRepo := db.NewLevelRepository(client)
	rankRepo := db.NewRankRepository(client)
	userStatsRepo := db.NewUserStatsRepository(client)
	rarityRepo := db.NewRarityRepository(client)
	userDiffStatsRepo := db.NewUserDifficultyStatsRepository(client)
	traitOfferRepository, err := db.NewTraitOfferRepository(client)
	if err != nil {
		return nil, err
	}
	transactions := platformtx.NewEntUnitOfWork(client.Client())
	journal := platformmutation.NewAuditJournal(
		transactions,
		audit.NewRepository(),
	)
	productionJournal := platformmutation.NewJournal(
		transactions,
		audit.NewRepository(),
		eventing.NewOutbox(),
	)
	progressionRepository, err := db.NewProgressionRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation progression repository: %w", err)
	}
	rewardRuleRepository, err := db.NewRewardRuleRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation reward rule repository: %w", err)
	}
	rewardProfileRepository, err := db.NewRewardProfileRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation reward profile repository: %w", err)
	}
	rankingProjectionRepository, err := db.NewRankingProjectionRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation ranking projection repository: %w", err)
	}
	traitOfferService, err := service.NewTraitOfferService(
		traitOfferRepository,
		transactions,
		offerOptions,
	)
	if err != nil {
		return nil, err
	}

	// Services
	caches := appcache.New(localCache)
	elementService := service.NewElementService(elementRepo, caches.CultivationElement, journal)
	traitService := service.NewTraitService(
		traitRepo,
		traitOfferService,
		caches.CultivationTrait,
		traitEffectRepository,
		productionJournal,
	)
	userTraitService := service.NewUserTraitService(userTraitRepo)
	levelService := service.NewLevelService(levelRepo, caches.CultivationLevel, journal)
	rankService := service.NewRankService(rankRepo, caches.CultivationRank, journal)
	rarityService := service.NewRarityService(rarityRepo, traitOfferService, caches.CultivationRarity, journal)
	rankingService, err := service.NewRankingService(
		rankingProjectionRepository, rankRepo, levelRepo, productionJournal,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation ranking service: %w", err)
	}
	progressionService, err := service.NewProgressionService(
		progressionRepository, productionJournal, service.ProgressionServiceOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation progression service: %w", err)
	}
	rewardRuleService, err := service.NewRewardRuleService(rewardRuleRepository, productionJournal)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation reward rule service: %w", err)
	}
	rewardProfileService, err := service.NewRewardProfileService(
		rewardProfileRepository,
		productionJournal,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation reward profile service: %w", err)
	}
	rankingReconciliation, err := service.NewRankingReconciliationWorkflow(
		rankingProjectionRepository,
		operations.Repository,
		operations.Starter,
		operations.Transactions,
		eventing.NewOutbox(),
		service.RankingReconciliationOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize cultivation ranking reconciliation: %w", err)
	}
	if err := rankingReconciliation.Register(operationRegistry); err != nil {
		return nil, fmt.Errorf("register cultivation ranking reconciliation: %w", err)
	}

	// Handlers
	handlerGroup := &http.CultivationHandlerGroup{
		ElementHandler:               http.NewElementHandler(elementService),
		TraitHandler:                 http.NewTraitHandler(traitService),
		UserTraitHandler:             http.NewUserTraitHandler(userTraitService),
		LevelHandler:                 http.NewLevelHandler(levelService),
		RankHandler:                  http.NewRankHandler(rankService),
		RarityHandler:                http.NewRarityHandler(rarityService),
		RankingHandler:               http.NewRankingHandler(rankingService),
		ProgressionHandler:           http.NewProgressionHandler(progressionService),
		RewardRuleHandler:            http.NewRewardRuleHandler(rewardRuleService),
		RewardProfileHandler:         http.NewRewardProfileHandler(rewardProfileService),
		RankingReconciliationHandler: http.NewRankingReconciliationHandler(rankingReconciliation),
	}

	return &CultivationContainer{
		CultivationHandlerGroup: handlerGroup,
		TraitOfferRepository:    traitOfferRepository,
		TraitOfferService:       traitOfferService,
		RewardProfileRepository: rewardProfileRepository,
		UserStatsRepo:           userStatsRepo,
		UserElementExpRepo:      userElementExpRepo,
		ElementRepo:             elementRepo,
		UserDifficultyStatsRepo: userDiffStatsRepo,
	}, nil
}
