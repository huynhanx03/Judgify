package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/platform/observability"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	submissionentity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/pkg/judging"
	"github.com/huynhanx03/judgify/pkg/settings"
)

func TestProductionWorkerCompositionIsInertUntilRun(t *testing.T) {
	config := validProductionRuntimeConfig(t)
	database := ent.WrapClient(generate.NewClient(), zap.NewNop())
	metrics, err := observability.New(observability.Config{
		Role: "judge-worker", Environment: "test", Version: "test",
	})
	if err != nil {
		t.Fatalf("observability.New() error = %v", err)
	}

	runtime, err := newProductionRuntime(config, database, zap.NewNop(), metrics)
	if err != nil {
		t.Fatalf("newProductionRuntime() error = %v", err)
	}
	if runtime.started || runtime.durable != nil || runtime.supervisor != nil ||
		runtime.stager != nil || runtime.registry != nil {
		t.Fatal("worker composition initialized an owned runtime before Run")
	}
	if _, err := os.Stat(config.StagingRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("composition touched staging root: %v", err)
	}
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() before Run error = %v", err)
	}
}

func TestNormalizeJudgeConfigAppliesFailClosedSandboxDefaults(t *testing.T) {
	config := normalizeJudgeConfig(settings.Judge{})
	if !config.NetworkDisabled {
		t.Fatal("NetworkDisabled = false, contestant networking must fail closed")
	}
	wantContainerMemoryMb := problemcontract.MaximumMemoryLimitKilobytes / 1024
	if config.ContainerMemoryMb != wantContainerMemoryMb || config.SandboxTmpfsSizeMb != 64 {
		t.Fatalf(
			"sandbox memory defaults = (%d, %d), want (%d, 64)",
			config.ContainerMemoryMb,
			config.SandboxTmpfsSizeMb,
			wantContainerMemoryMb,
		)
	}
	if config.MaximumTestInputBytes != judging.MaximumTestInputBytes ||
		config.MaximumOutputBytes != problemcontract.MaximumOutputLimitBytes ||
		config.MaximumDiagnosticBytes != submissionentity.MaximumJudgeDiagnosticBytes {
		t.Fatalf("execution byte limits were not normalized: %+v", config)
	}
	if config.LeaseHeartbeat != config.LeaseDuration/5 {
		t.Fatalf(
			"lease heartbeat = %s, want one fifth of %s",
			config.LeaseHeartbeat,
			config.LeaseDuration,
		)
	}
}

func TestJudgeWorkerRejectsContentLimitsThatDivergeFromPersistence(t *testing.T) {
	t.Parallel()

	for _, config := range []settings.Judge{
		{
			MaximumTestInputBytes: judging.MaximumTestInputBytes - 1,
			MaximumOutputBytes:    problemcontract.MaximumOutputLimitBytes,
		},
		{
			MaximumTestInputBytes: judging.MaximumTestInputBytes,
			MaximumOutputBytes:    problemcontract.MaximumOutputLimitBytes + 1,
		},
	} {
		if err := validateJudgeContentLimits(
			normalizeJudgeConfig(config),
		); !errors.Is(err, ErrInvalidWorker) {
			t.Fatalf(
				"validateJudgeContentLimits(%+v) = %v, want ErrInvalidWorker",
				config,
				err,
			)
		}
	}

	if err := validateJudgeContentLimits(
		normalizeJudgeConfig(settings.Judge{}),
	); err != nil {
		t.Fatalf("canonical default limits rejected: %v", err)
	}
}

func TestProductionWorkerRejectsLeaseHeartbeatWithoutExpiryHeadroom(t *testing.T) {
	config := validProductionRuntimeConfig(t)
	config.LeaseHeartbeat = config.LeaseDuration/4 + time.Nanosecond

	if err := validateProductionJudgeConfig(config); !errors.Is(
		err,
		ErrInvalidWorker,
	) {
		t.Fatalf(
			"validateProductionJudgeConfig() error = %v, want ErrInvalidWorker",
			err,
		)
	}
}

func TestCanonicalWorkerKeyNormalizesHostIdentity(t *testing.T) {
	t.Parallel()

	if got := canonicalWorkerKey(" Judgify Worker ", "Host Name.local", 42); got !=
		"judgify-worker-host-name.local-42" {
		t.Fatalf("canonicalWorkerKey() = %q", got)
	}
	if got := canonicalWorkerKey("", "", 0); got != "0" {
		t.Fatalf("canonicalWorkerKey() empty identity = %q", got)
	}
}

func validProductionRuntimeConfig(t *testing.T) settings.Judge {
	t.Helper()
	directory := t.TempDir()
	manifestPath := filepath.Join(directory, "release.json")
	if err := os.WriteFile(manifestPath, []byte("{}\n"), 0o400); err != nil {
		t.Fatalf("write release manifest: %v", err)
	}
	return normalizeJudgeConfig(settings.Judge{
		ReleaseID:                       "judge-release.test",
		ReleaseManifestPath:             manifestPath,
		DockerSocketPath:                filepath.Join(directory, "docker.sock"),
		StagingRoot:                     filepath.Join(directory, "staging"),
		MaximumActiveSubmissionsPerUser: 1,
		MaximumReadyBacklog:             8,
		MaximumOldestReadyAge:           time.Minute,
		MinimumFreeDiskBytes:            1,
		AdmissionDiskPath:               directory,
		WorkerHeartbeatFreshness:        30 * time.Second,
		DockerImage: "example.invalid/judgify@sha256:" +
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		PoolSize:         1,
		WorkerCount:      1,
		CompileTimeoutMs: 1000,
	})
}
