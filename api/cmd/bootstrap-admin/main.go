// Command bootstrap-admin creates the first protected administrator in an
// already-migrated, otherwise empty production database. It is deliberately a
// finite operator command and never composes API, worker, migration, cache, or
// authorization runtime roles.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/infrastructure/bootstrap"
	"github.com/huynhanx03/judgify/internal/operator/bootstrapadmin"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	exitCode := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(exitCode)
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runWith(ctx, args, stdin, stdout, stderr, os.Getenv, bootstrapDatabase)
}

func runWith(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	getenv func(string) string,
	bootstrapFn func(context.Context, bootstrapadmin.Input) (uuid.UUID, error),
) int {
	input, err := bootstrapadmin.ParseInput(args, getenv, stdin)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Usage: bootstrap-admin --username USERNAME --password-stdin")
		_, _ = fmt.Fprintln(stderr, "       environment: JUDGIFY_BOOTSTRAP_ADMIN_USERNAME, JUDGIFY_BOOTSTRAP_ADMIN_PASSWORD")
		_, _ = fmt.Fprintf(stderr, "bootstrap-admin: %v\n", err)
		return 2
	}
	defer input.Clear()
	if bootstrapFn == nil {
		_, _ = fmt.Fprintln(stderr, "bootstrap-admin: unavailable")
		return 1
	}
	administratorID, err := bootstrapFn(ctx, input)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bootstrap-admin: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "bootstrap-admin: initialized administrator %s\n", administratorID)
	return 0
}

func bootstrapDatabase(ctx context.Context, input bootstrapadmin.Input) (administratorID uuid.UUID, resultErr error) {
	process, err := bootstrap.PrepareForRole(bootstrap.ProfileOperator)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, process.Close(), process.Sync())
	}()
	client, err := process.OpenDatabase()
	if err != nil {
		return uuid.Nil, fmt.Errorf("open migrated database: %w", err)
	}
	return bootstrapadmin.Bootstrap(ctx, client.Client(), input)
}
