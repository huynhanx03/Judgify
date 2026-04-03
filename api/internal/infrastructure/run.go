package infrastructure

import (
	"context"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/di"
	"github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/judge"
	"github.com/huynhanx03/judgify/internal/judge/executor"
)

// Run starts the Identity service.
func Run() error {
	LoadConfig()
	SetupLogger()
	log := global.LoggerZap.Named("infra")
	log.Info("Config and Logger initialized")
	SetupEnt()
	SetupCache()
	log.Info("Cache initialized")
	SetupKeys()
	container := di.SetupDependencies()

	// Pre-load gacha pools on startup
	if err := container.Cultivation.GachaService.LoadPool(context.Background()); err != nil {
		log.Warn("failed to pre-load gacha pools", zap.Error(err))
	} else {
		log.Info("Gacha pools loaded")
	}

	Initialized()

	// Start judge worker
	judgeWorker, judgeCleanup := startJudgeWorker(container)
	defer judgeCleanup()
	_ = judgeWorker

	// Start EXP reward worker
	expWorker, expCleanup := startExpRewardWorker(container)
	defer expCleanup()
	_ = expWorker

	// Start scheduler for stats recalculation
	scheduler := StartScheduler(container.Problem.ProblemRepo)
	defer func() { _ = scheduler.Shutdown() }()

	http := NewHTTPServer()
	return http.Run()
}

// startJudgeWorker creates Docker pool, MQ consumer, and starts the judge worker.
func startJudgeWorker(c *di.Container) (*judge.Worker, func()) {
	ctx := context.Background()
	cfg := global.Config.Judge

	// Create Docker container pool
	dockerPool, err := executor.NewDockerPool(ctx, cfg.DockerImage, cfg.PoolSize, cfg.NetworkDisabled, cfg.ContainerMemoryMb, cfg.SandboxTmpfsSizeMb)
	if err != nil {
		global.LoggerZap.Fatal("failed to create docker pool", zap.Error(err))
	}

	// Create MQ consumer for judge jobs
	consumer, err := c.Broker.NewConsumer("judge-worker", constant.TopicJudge)
	if err != nil {
		global.LoggerZap.Fatal("failed to create judge consumer", zap.Error(err))
	}

	// Create MQ producer for EXP reward events
	expProducer, err := c.Broker.NewProducer(constant.TopicExpReward)
	if err != nil {
		global.LoggerZap.Fatal("failed to create exp reward producer", zap.Error(err))
	}

	// Create and start judge worker
	worker, err := judge.NewWorker(
		consumer,
		dockerPool,
		expProducer,
		c.Submission.SubmissionRepo,
		c.Problem.ProblemRepo,
		c.Problem.TestCaseRepo,
		c.Cultivation.UserStatsRepo,
		cfg,
	)
	if err != nil {
		global.LoggerZap.Fatal("failed to create judge worker", zap.Error(err))
	}

	worker.Start(ctx)
	global.LoggerZap.Info("Judge worker started")

	cleanup := func() {
		worker.Stop()
		dockerPool.Shutdown(context.Background())
		global.LoggerZap.Info("Judge worker stopped")
	}

	return worker, cleanup
}

// startExpRewardWorker creates and starts the EXP reward event consumer.
func startExpRewardWorker(c *di.Container) (*judge.ExpRewardWorker, func()) {
	ctx := context.Background()

	// Create MQ consumer for EXP reward events
	consumer, err := c.Broker.NewConsumer("exp-reward-worker", constant.TopicExpReward)
	if err != nil {
		global.LoggerZap.Fatal("failed to create exp reward consumer", zap.Error(err))
	}

	worker, err := judge.NewExpRewardWorker(
		consumer,
		c.Problem.ProblemRepo,
		c.Cultivation.UserStatsRepo,
		c.Cultivation.UserElementExpRepo,
		c.Cultivation.UserDifficultyStatsRepo,
		c.Cultivation.UserTagStatsRepo,
		c.Cultivation.UserTraitRepo,
	)
	if err != nil {
		global.LoggerZap.Fatal("failed to create exp reward worker", zap.Error(err))
	}

	worker.Start(ctx)
	global.LoggerZap.Info("EXP reward worker started")

	cleanup := func() {
		worker.Stop()
		global.LoggerZap.Info("EXP reward worker stopped")
	}

	return worker, cleanup
}
