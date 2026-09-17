// Command server is the production API-only entrypoint. It shares the runtime
// lifecycle implementation without importing the worker graph.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/huynhanx03/judgify/internal/infrastructure"
	"github.com/huynhanx03/judgify/internal/infrastructure/bootstrap"
	judgifyruntime "github.com/huynhanx03/judgify/internal/runtime"
)

func main() {
	if len(os.Args) != 1 {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: server")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "judgify api: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) (resultErr error) {
	process, err := bootstrap.PrepareForRole(bootstrap.ProfileAPI)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, process.Close(), process.Sync()) }()
	config := judgifyruntime.NewConfig(string(process.Config.Server.Mode), judgifyruntime.Builders{
		API: process.Builder(infrastructure.BuildAPI),
	})
	return judgifyruntime.Run(ctx, judgifyruntime.RoleAPI, config)
}
