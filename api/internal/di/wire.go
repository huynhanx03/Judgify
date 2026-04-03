package di

import (
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/constant"
	cultivationDi "github.com/huynhanx03/judgify/internal/cultivation/di"
	identityDi "github.com/huynhanx03/judgify/internal/identity/di"
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

	cultivationContainer := cultivationDi.NewCultivationContainer()
	problemContainer := problemDi.NewProblemContainer()
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
	)
	submissionContainer := submissionDi.NewSubmissionContainer(judgeProducer)

	container := &Container{
		Identity:    identityContainer,
		Problem:     problemContainer,
		Cultivation: cultivationContainer,
		Submission:  submissionContainer,
		Broker:      broker,
	}

	GlobalContainer = container
	global.LoggerZap.Named("di").Info("Dependencies initialized successfully")
	return container
}
