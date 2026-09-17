// Command migrate is the production operator-only schema migration entrypoint.
// It embeds the immutable Atlas chain and never composes API, worker, cache, or
// Ent application resources.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	migration "github.com/huynhanx03/judgify/internal/ent/migrate"
	"github.com/huynhanx03/judgify/internal/infrastructure/configuration"
	"github.com/huynhanx03/judgify/internal/infrastructure/logging"
	judgifyruntime "github.com/huynhanx03/judgify/internal/runtime"
)

func main() {
	if len(os.Args) != 1 {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: migrate")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "judgify migrate: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) (resultErr error) {
	config, err := configuration.Load(
		ctx,
		configuration.ProfileMigrate,
		configuration.OSFileSecretSource{},
	)
	if err != nil {
		return err
	}
	logger, err := logging.New(config)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, logger.Sync()) }()
	runner, err := migration.New(logger.Logger, config.Migration)
	if err != nil {
		return fmt.Errorf("configure migration runner: %w", err)
	}
	runtimeConfig := judgifyruntime.NewConfig(string(config.Server.Mode), judgifyruntime.Builders{
		Migrate: func(ctx context.Context) error {
			if err := runner.Apply(ctx, config); err != nil {
				return fmt.Errorf("apply checked-in migrations: %w", err)
			}
			return nil
		},
	})
	return judgifyruntime.Run(ctx, judgifyruntime.RoleMigrate, runtimeConfig)
}
