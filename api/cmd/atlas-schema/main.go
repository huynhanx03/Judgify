package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/huynhanx03/judgify/internal/ent/desiredschema"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "atlas-schema: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, output io.Writer) error {
	state, err := desiredschema.Load(ctx)
	if err != nil {
		return err
	}
	ddl, err := state.RelationalDDL(ctx)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(output, ddl); err != nil {
		return fmt.Errorf("write desired schema DDL: %w", err)
	}
	return nil
}
