// Command judgify holds developer and operator tools used by migrations and judge operations.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

type command struct {
	name string
	run  func(context.Context, []string, io.Reader, io.Writer, io.Writer) int
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || isHelp(args[0]) {
		writeUsage(stdout)
		if len(args) == 0 {
			return 2
		}
		return 0
	}

	for _, command := range commands() {
		if command.name == args[0] {
			return command.run(ctx, args[1:], stdin, stdout, stderr)
		}
	}

	_, _ = fmt.Fprintf(stderr, "judgify: unknown command %q\n", args[0])
	writeUsage(stderr)
	return 2
}

func commands() []command {
	return []command{
		{name: "bootstrap-admin", run: bootstrapAdminCommand},
		{name: "release-manifest", run: releaseManifestCommand},
		{name: "judge-capability", run: judgeCapabilityCommand},
	}
}

func releaseManifestCommand(
	ctx context.Context,
	args []string,
	_ io.Reader,
	stdout, stderr io.Writer,
) int {
	return runReleaseManifest(ctx, args, stdout, stderr)
}

func judgeCapabilityCommand(
	ctx context.Context,
	args []string,
	_ io.Reader,
	stdout, stderr io.Writer,
) int {
	return runJudgeCapability(ctx, args, stdout, stderr)
}

func writeUsage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, "Usage: judgify <bootstrap-admin|release-manifest|judge-capability>")
	_, _ = fmt.Fprintln(writer, "       judgify bootstrap-admin [options]")
	_, _ = fmt.Fprintln(writer, "       judgify release-manifest <template|sandbox-env|approve|verify> [options]")
	_, _ = fmt.Fprintln(writer, "       judgify judge-capability <activate|retire> [options]")
}

func isHelp(value string) bool {
	return value == "help" || value == "-h" || value == "--help"
}
