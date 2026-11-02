// Command migrate is the production operator-only schema migration entrypoint.
// It embeds the immutable Atlas chain and never composes API, worker, cache, or
// Ent application resources.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	migration "github.com/huynhanx03/judgify/internal/ent/migrate"
	"github.com/huynhanx03/judgify/internal/infrastructure/configuration"
	"github.com/huynhanx03/judgify/internal/infrastructure/logging"
	judgifyruntime "github.com/huynhanx03/judgify/internal/runtime"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	if len(os.Args) > 1 {
		code := runCommand(ctx, os.Args[1:], os.Stdout, os.Stderr)
		stop()
		os.Exit(code)
	}
	err := run(ctx)
	stop()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "judgify migrate: %v\n", err)
		os.Exit(1)
	}
}

func runCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "Usage: migrate [schema|inventory]")
		return 2
	}

	switch args[0] {
	case "schema":
		if len(args) != 1 {
			_, _ = fmt.Fprintln(stderr, "Usage: migrate schema")
			return 2
		}
		if err := writeAtlasSchema(ctx, stdout); err != nil {
			_, _ = fmt.Fprintf(stderr, "migrate schema: %v\n", err)
			return 1
		}
		return 0
	case "inventory":
		return inventoryCommand(ctx, args[1:], nil, stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "migrate: unknown command %q\n", args[0])
		_, _ = fmt.Fprintln(stderr, "Usage: migrate [schema|inventory]")
		return 2
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
	runner, err := migration.New(zap.New(logger.Core()), config.Migration)
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
	}, logger)
	return judgifyruntime.Run(ctx, judgifyruntime.RoleMigrate, runtimeConfig)
}
