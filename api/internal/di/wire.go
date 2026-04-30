package di

import (
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/constant"
	contestConstant "github.com/huynhanx03/judgify/internal/contest/constant"
	contestDi "github.com/huynhanx03/judgify/internal/contest/di"
	cultivationDi "github.com/huynhanx03/judgify/internal/cultivation/di"
	internalEnt "github.com/huynhanx03/judgify/internal/ent"
	identityDi "github.com/huynhanx03/judgify/internal/identity/di"
	materialDi "github.com/huynhanx03/judgify/internal/material/di"
	problemDi "github.com/huynhanx03/judgify/internal/problem/di"
	submissionDi "github.com/huynhanx03/judgify/internal/submission/di"
	"github.com/huynhanx03/judgify/pkg/mq/forge"
)

// SetupDependencies initializes all domain dependencies and returns the global container.
func SetupDependencies() *Container {
	// Setup Forge MQ broker
	broker, err := forge.NewBroker("./storages/mq")
	if err != nil {
		global.LoggerZap.Fatal("failed to create forge broker", zap.Error(err))
	}

	// Create judge producer
	judgeProducer, err := broker.NewProducer(constant.TopicJudge)
	if err != nil {
		global.LoggerZap.Fatal("failed to create judge producer", zap.Error(err))
	}

	// Create contest judge producer
	contestJudgeProducer, err := broker.NewProducer(contestConstant.TopicContestJudge)
	if err != nil {
		global.LoggerZap.Fatal("failed to create contest judge producer", zap.Error(err))
	}

	cultivationContainer := cultivationDi.NewCultivationContainer()
	problemContainer := problemDi.NewProblemContainer()

	// Create transaction manager backed by EntClient
	txMgr := internalEnt.NewTxManager(global.EntClient)

	identityContainer := identityDi.NewIdentityContainer(
		cultivationContainer.UserTraitRepo,
		cultivationContainer.UserStatsRepo,
		cultivationContainer.UserElementExpRepo,
		cultivationContainer.ElementRepo,
		cultivationContainer.LevelRepo,
		cultivationContainer.RankRepo,
		cultivationContainer.UserDifficultyStatsRepo,
		cultivationContainer.UserTagStatsRepo,
		problemContainer.DifficultyRepo,
		txMgr,
	)
	submissionContainer := submissionDi.NewSubmissionContainer(judgeProducer, contestJudgeProducer)
	contestContainer := contestDi.NewContestContainer(cultivationContainer.UserStatsRepo, txMgr)
	materialContainer := materialDi.NewMaterialContainer()

	container := &Container{
		Identity:    identityContainer,
		Problem:     problemContainer,
		Cultivation: cultivationContainer,
		Submission:  submissionContainer,
		Contest:     contestContainer,
		Material:    materialContainer,
		Broker:      broker,
	}

	GlobalContainer = container
	global.LoggerZap.Named("di").Info("Dependencies initialized successfully")
	return container
}
