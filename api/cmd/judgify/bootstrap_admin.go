package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/infrastructure/bootstrap"
	"github.com/huynhanx03/judgify/internal/operator/bootstrapadmin"
)

func bootstrapAdminCommand(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) int {
	return runBootstrapAdminWith(ctx, args, stdin, stdout, stderr, os.Getenv, bootstrapDatabase)
}

func runBootstrapAdminWith(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	getenv func(string) string,
	bootstrapFn func(context.Context, bootstrapadmin.Input) (uuid.UUID, error),
) int {
	input, err := bootstrapadmin.ParseInput(args, getenv, stdin)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "Usage: judgify bootstrap-admin --username USERNAME --password-stdin")
		_, _ = fmt.Fprintln(stderr, "       environment: JUDGIFY_BOOTSTRAP_ADMIN_USERNAME, JUDGIFY_BOOTSTRAP_ADMIN_PASSWORD")
		_, _ = fmt.Fprintf(stderr, "judgify bootstrap-admin: %v\n", err)
		return 2
	}
	defer input.Clear()
	if bootstrapFn == nil {
		_, _ = fmt.Fprintln(stderr, "judgify bootstrap-admin: unavailable")
		return 1
	}

	administratorID, err := bootstrapFn(ctx, input)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "judgify bootstrap-admin: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "initialized administrator %s\n", administratorID)
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
