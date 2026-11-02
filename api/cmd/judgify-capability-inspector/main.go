package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/huynhanx03/judgify/internal/judge/capability/inspector"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := inspector.RunCLI(ctx, os.Args[1:], os.Stdout, inspector.NewOSRunner())
	stop()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "capability inspection denied")
		os.Exit(1)
	}
}
