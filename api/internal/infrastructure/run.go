package infrastructure

import (
	"context"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/di"
	"github.com/huynhanx03/judgify/internal/judge"
	"github.com/huynhanx03/judgify/internal/judge/executor"
	"github.com/huynhanx03/judgify/internal/submission/constant"
)

// Run starts the Identity service.
func Run() error {
	LoadConfig()
	SetupLogger()
	global.LoggerZap.Named("infra").Info("Config and Logger initialized")
	SetupEnt()
	SetupCache()
	global.LoggerZap.Named("infra").Info("Cache initialized")
	SetupKeys()
	container := di.SetupDependencies()

	Initialized()

	// Start judge worker
	judgeWorker, judgeCleanup := startJudgeWorker(container)
	defer judgeCleanup()
	_ = judgeWorker

	http := NewHTTPServer()
	return http.Run()
}

// startJudgeWorker creates Docker pool, MQ consumer, and starts the judge worker.
func startJudgeWorker(c *di.Container) (*judge.Worker, func()) {
	ctx := context.Background()
	cfg := global.Config.Judge

	// Create Docker container pool
	dockerPool, err := executor.NewDockerPool(ctx, cfg.DockerImage, cfg.PoolSize, cfg.NetworkDisabled)
	if err != nil {
		global.LoggerZap.Fatal("failed to create docker pool", zap.Error(err))
	}

	// Create MQ consumer
	consumer, err := c.Broker.NewConsumer("judge-worker", constant.TopicJudge)
	if err != nil {
		global.LoggerZap.Fatal("failed to create judge consumer", zap.Error(err))
	}

	// Create and start judge worker
	worker, err := judge.NewWorker(
		consumer,
		dockerPool,
		c.Submission.SubmissionRepo,
		c.Problem.ProblemRepo,
		c.Problem.TestCaseRepo,
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
