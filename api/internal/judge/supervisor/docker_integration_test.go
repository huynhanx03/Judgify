//go:build sandbox

package supervisor

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/judge/capability"
)

const (
	sandboxSocketEnvironment = "JUDGIFY_DOCKER_SOCKET"
	sandboxImageEnvironment  = "JUDGIFY_SANDBOX_IMAGE_DIGEST"
)

func TestDockerExecutesAndDestroysDisposableSandboxBoundary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("sandbox integration requires a Linux host with delegated cgroup v2")
	}
	socket := os.Getenv(sandboxSocketEnvironment)
	imageDigest := os.Getenv(sandboxImageEnvironment)
	if socket == "" || imageDigest == "" {
		t.Skipf(
			"sandbox integration disabled: set %s and %s after the rootless cgroup-v2 capability probe passes",
			sandboxSocketEnvironment,
			sandboxImageEnvironment,
		)
	}
	info, err := os.Stat(socket)
	if err != nil {
		t.Fatalf("configured rootless Docker socket is unavailable: %v", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("configured Docker endpoint is not a Unix socket: %s", socket)
	}

	engine, err := NewDockerEngine(DockerEngineConfig{SocketPath: socket})
	if err != nil {
		t.Fatalf("NewDockerEngine() error = %v", err)
	}
	bootID := uuid.New()
	supervisor, err := NewSandboxSupervisor(Config{
		Engine:         engine,
		Cgroups:        NewProductionCgroupV2Resolver(),
		Probe:          integrationCapabilityProbe{},
		BootID:         bootID,
		CleanupTimeout: 15 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewSandboxSupervisor() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			20*time.Second,
		)
		defer cancel()
		if err := supervisor.Shutdown(shutdownContext); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	executionContext, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()
	result, err := supervisor.Execute(executionContext, ExecuteRequest{
		Identity: ExecutionIdentity{
			JobID:        uuid.New(),
			AttemptID:    uuid.New(),
			WorkerBootID: bootID,
		},
		Boundary:    BoundaryTestcase,
		ImageDigest: imageDigest,
		Argv:        []string{"/bin/sleep", "0.05"},
		WorkingDir:  "/workspace",
		Stdin:       bytes.NewReader(nil),
		Limits: ResourceLimits{
			WallTime:        2 * time.Second,
			CPUTime:         time.Second,
			MemoryBytes:     64 << 20,
			Pids:            16,
			OutputBytes:     1 << 20,
			DiagnosticBytes: 64 << 10,
			FileBytes:       8 << 20,
			TmpfsBytes:      16 << 20,
			OpenFiles:       64,
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ContainerID == "" || result.CgroupID == "" ||
		result.ExitCode != 0 || !result.AccountingComplete ||
		!result.CleanupComplete {
		t.Fatalf("Execute() returned incomplete trusted evidence: %+v", result)
	}
}

type integrationCapabilityProbe struct{}

func (integrationCapabilityProbe) Probe(
	context.Context,
	capability.CapabilityTuple,
	uuid.UUID,
) (capability.CapabilityEvidence, error) {
	return capability.CapabilityEvidence{Passing: true}, nil
}
