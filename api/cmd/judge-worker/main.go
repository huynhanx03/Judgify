// Command judge-worker is the production worker-only entrypoint. Its import
// graph contains Forge and Docker but no product HTTP router.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/huynhanx03/judgify/internal/infrastructure/bootstrap"
	workerInfrastructure "github.com/huynhanx03/judgify/internal/infrastructure/worker"
	judgifyruntime "github.com/huynhanx03/judgify/internal/runtime"
)

func main() {
	if len(os.Args) != 1 {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: judge-worker")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "judgify judge-worker: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) (resultErr error) {
	process, err := bootstrap.PrepareForRole(
		bootstrap.ProfileJudgeWorker,
	)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, process.Close(), process.Sync()) }()
	config := judgifyruntime.NewConfig(string(process.Config.Server.Mode), judgifyruntime.Builders{
		JudgeWorker: process.Builder(workerInfrastructure.Build),
	})
	return judgifyruntime.Run(ctx, judgifyruntime.RoleJudgeWorker, config)
}
