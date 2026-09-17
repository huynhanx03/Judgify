// Command judgify is the developer/operator multiplexer. Production deploys
// the role-pure cmd/server, cmd/judge-worker, and cmd/migrate artifacts.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	migration "github.com/huynhanx03/judgify/internal/ent/migrate"
	"github.com/huynhanx03/judgify/internal/infrastructure"
	"github.com/huynhanx03/judgify/internal/infrastructure/bootstrap"
	workerInfrastructure "github.com/huynhanx03/judgify/internal/infrastructure/worker"
	judgifyruntime "github.com/huynhanx03/judgify/internal/runtime"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	exitCode := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(exitCode)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stderr)
		return 2
	}

	switch args[0] {
	case "inventory":
		return runInventory(ctx, args[1:], stdout, stderr)
	case "release-manifest":
		return runReleaseManifest(ctx, args[1:], stdout, stderr)
	case "judge-capability":
		return runJudgeCapability(ctx, args[1:], stdout, stderr)
	case string(judgifyruntime.RoleAPI),
		string(judgifyruntime.RoleJudgeWorker),
		string(judgifyruntime.RoleMigrate),
		string(judgifyruntime.RoleAll):
		if len(args) != 1 {
			_, _ = fmt.Fprintf(stderr, "judgify %s: unexpected arguments\n", args[0])
			return 2
		}
		role, err := judgifyruntime.ParseRole(args[0])
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "judgify: %v\n", err)
			return 2
		}
		if err := runRole(ctx, role); err != nil {
			_, _ = fmt.Fprintf(stderr, "judgify %s: %v\n", role, err)
			return 1
		}
		return 0
	case "help", "-h", "--help":
		writeUsage(stdout)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "judgify: unknown command %q\n", args[0])
		writeUsage(stderr)
		return 2
	}
}

func writeUsage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, "Usage: judgify <api|judge-worker|migrate|all|inventory|release-manifest|judge-capability>")
	_, _ = fmt.Fprintln(writer, "       judgify inventory [--format markdown|json] [--migrations path]")
	_, _ = fmt.Fprintln(writer, "       judgify release-manifest <template|sandbox-env|approve|verify> [options]")
	_, _ = fmt.Fprintln(writer, "       judgify judge-capability <activate|retire> [options]")
}

func runRole(ctx context.Context, role judgifyruntime.Role) (resultErr error) {
	profile := bootstrap.ProfileAPI
	switch role {
	case judgifyruntime.RoleJudgeWorker:
		profile = bootstrap.ProfileJudgeWorker
	case judgifyruntime.RoleMigrate:
		profile = bootstrap.ProfileMigrate
	case judgifyruntime.RoleAll:
		profile = bootstrap.ProfileAll
	}
	process, err := bootstrap.PrepareForRole(profile)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, process.Close(), process.Sync())
	}()
	if role == judgifyruntime.RoleMigrate {
		runner, err := migration.New(process.Logger.Logger, process.Config.Migration)
		if err != nil {
			return fmt.Errorf("configure migration runner: %w", err)
		}
		if err := process.SetMigrationRunner(runner); err != nil {
			return fmt.Errorf("install migration runner: %w", err)
		}
	}

	config := judgifyruntime.NewConfig(string(process.Config.Server.Mode), judgifyruntime.Builders{
		API:         process.Builder(infrastructure.BuildAPI),
		JudgeWorker: process.Builder(workerInfrastructure.Build),
		All:         process.Builder(infrastructure.BuildAPI, workerInfrastructure.Build),
		Migrate:     process.Migrate,
	})
	return judgifyruntime.Run(ctx, role, config)
}
