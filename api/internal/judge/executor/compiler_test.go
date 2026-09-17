package executor

import (
	"archive/tar"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/judge/capability"
	"github.com/huynhanx03/judgify/internal/judge/supervisor"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
)

func TestCompilerUsesOneDisposableBoundaryAndStagesOnlyDeclaredArtifact(t *testing.T) {
	t.Parallel()

	sandbox := &recordingSupervisor{compileArchive: artifactArchive(t, []artifactArchiveEntry{
		{name: "artifact/", mode: 0o700, kind: tar.TypeDir},
		{name: "artifact/main", mode: 0o755, kind: tar.TypeReg, body: []byte("ELF")},
	})}
	stager := newTestStager(t, 64<<20)
	compiler, err := NewCompiler(sandbox, stager)
	if err != nil {
		t.Fatalf("NewCompiler() error = %v", err)
	}
	profile := executableTestRuntimeProfile()
	source := []byte("int main(){return 0;}\n")
	identity := testExecutionIdentity()
	outcome, err := compiler.Compile(context.Background(), CompileRequest{
		Identity:           identity,
		RuntimeProfile:     profile,
		Source:             source,
		SourceChecksum:     sourceChecksum(source),
		MaximumSourceBytes: 1 << 20,
		Limits:             testResourceLimits(),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if outcome.Status != CompileStatusSucceeded || outcome.Artifact == nil ||
		len(outcome.Execution.OutputArchive) != 0 {
		t.Fatalf("Compile() = %+v", outcome)
	}
	requests := sandbox.requests()
	if len(requests) != 1 || requests[0].Boundary != supervisor.BoundaryCompile ||
		requests[0].Capture == nil ||
		requests[0].Capture.Path != "/workspace/artifact" {
		t.Fatalf("supervisor requests = %+v", requests)
	}
	if len(requests[0].InputFiles) != 1 ||
		requests[0].InputFiles[0].Path != profile.Workspace.SourcePath {
		t.Fatalf("compile inputs = %+v", requests[0].InputFiles)
	}
	if err := outcome.Artifact.Release(context.Background()); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
}

func TestCompilerDoesNotStageRejectedCompilation(t *testing.T) {
	t.Parallel()

	sandbox := &recordingSupervisor{exitCode: 1}
	stager := newTestStager(t, 64<<20)
	compiler, err := NewCompiler(sandbox, stager)
	if err != nil {
		t.Fatalf("NewCompiler() error = %v", err)
	}
	profile := executableTestRuntimeProfile()
	source := []byte("not c++")
	outcome, err := compiler.Compile(context.Background(), CompileRequest{
		Identity:           testExecutionIdentity(),
		RuntimeProfile:     profile,
		Source:             source,
		SourceChecksum:     sourceChecksum(source),
		MaximumSourceBytes: 1 << 20,
		Limits:             testResourceLimits(),
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if outcome.Status != CompileStatusRejected || outcome.Artifact != nil {
		t.Fatalf("Compile() = %+v", outcome)
	}
}

func TestTestcaseExecutorCreatesFreshMinimalBoundaryPerCase(t *testing.T) {
	t.Parallel()

	sandbox := &recordingSupervisor{}
	stager := newTestStager(t, 64<<20)
	profile := executableTestRuntimeProfile()
	attemptID := testExecutionIdentity().AttemptID
	artifact, err := stager.Stage(context.Background(), StageRequest{
		AttemptID: attemptID,
		Archive: artifactArchive(t, []artifactArchiveEntry{{
			name: "artifact/main", mode: 0o755, kind: tar.TypeReg, body: []byte("ELF"),
		}}),
		RuntimeProfile:   profile,
		ManifestChecksum: profile.ManifestChecksum,
	})
	if err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	t.Cleanup(func() { _ = artifact.Release(context.Background()) })
	executor, err := NewTestcaseExecutor(sandbox)
	if err != nil {
		t.Fatalf("NewTestcaseExecutor() error = %v", err)
	}
	first, err := executor.Run(context.Background(), TestcaseRequest{
		Identity:   testExecutionIdentity(),
		TestcaseID: uuid.MustParse("44444444-4444-4444-8444-444444444444"),
		Artifact:   &artifact,
		Input:      []byte("1 2\n"),
		Limits:     testResourceLimits(),
	})
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	second, err := executor.Run(context.Background(), TestcaseRequest{
		Identity:   testExecutionIdentity(),
		TestcaseID: uuid.MustParse("55555555-5555-4555-8555-555555555555"),
		Artifact:   &artifact,
		Input:      []byte("3 4\n"),
		Limits:     testResourceLimits(),
	})
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if first.Execution.ContainerID == second.Execution.ContainerID ||
		first.Execution.CgroupID == second.Execution.CgroupID {
		t.Fatalf("testcases reused boundary: first=%+v second=%+v", first, second)
	}
	requests := sandbox.requests()
	if len(requests) != 2 {
		t.Fatalf("supervisor request count = %d", len(requests))
	}
	for _, request := range requests {
		if request.Boundary != supervisor.BoundaryTestcase ||
			request.Capture != nil || len(request.InputFiles) != 1 ||
			request.InputFiles[0].Path != "artifact/main" {
			t.Fatalf("testcase received non-minimal namespace: %+v", request)
		}
	}
}

type recordingSupervisor struct {
	mu             sync.Mutex
	executions     []supervisor.ExecuteRequest
	compileArchive []byte
	exitCode       int
	nextBoundary   int
}

func (sandbox *recordingSupervisor) Execute(
	_ context.Context,
	request supervisor.ExecuteRequest,
) (supervisor.ExecuteResult, error) {
	sandbox.mu.Lock()
	defer sandbox.mu.Unlock()
	sandbox.nextBoundary++
	cloned := request
	cloned.Argv = append([]string(nil), request.Argv...)
	cloned.Env = append([]string(nil), request.Env...)
	cloned.InputFiles = append([]supervisor.InputFile(nil), request.InputFiles...)
	sandbox.executions = append(sandbox.executions, cloned)
	result := supervisor.ExecuteResult{
		ContainerID:        "container-" + string(rune('0'+sandbox.nextBoundary)),
		CgroupID:           "cgroup-" + string(rune('0'+sandbox.nextBoundary)),
		ExitCode:           sandbox.exitCode,
		Stdout:             []byte("answer\n"),
		AccountingComplete: true,
		CleanupComplete:    true,
	}
	if request.Capture != nil {
		result.OutputArchive = append([]byte(nil), sandbox.compileArchive...)
	}
	return result, nil
}

func (*recordingSupervisor) Probe(
	context.Context,
) (capability.CapabilityEvidence, error) {
	return capability.CapabilityEvidence{Passing: true}, nil
}

func (*recordingSupervisor) Shutdown(context.Context) error { return nil }

func (sandbox *recordingSupervisor) requests() []supervisor.ExecuteRequest {
	sandbox.mu.Lock()
	defer sandbox.mu.Unlock()
	return append([]supervisor.ExecuteRequest(nil), sandbox.executions...)
}

func executableTestRuntimeProfile() problementity.RuntimeProfile {
	return problementity.RuntimeProfile{
		Key: "cpp17", ProfileVersion: "2026.07.1",
		ImageDigest:      "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ManifestChecksum: "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Compile: problementity.CommandTemplate{
			Executable: "/usr/bin/g++",
			Arguments: []problementity.CommandArgument{
				{Kind: problementity.ArgumentWorkerPath, Value: "source"},
				{Kind: problementity.ArgumentLiteral, Value: "-o"},
				{Kind: problementity.ArgumentWorkerPath, Value: "output"},
			},
		},
		Run: problementity.CommandTemplate{Executable: "/workspace/artifact/main"},
		Artifacts: []problementity.RuntimeArtifact{{
			Path: "main", Executable: true, MaximumBytes: 32 << 20,
			MaximumFiles: 1, Required: true,
		}},
		Workspace: problementity.RuntimeWorkspace{
			SourcePath: "source/main.cpp",
			Values: []problementity.RuntimeWorkspaceValue{
				{Name: "source", Path: "/workspace/source/main.cpp"},
				{Name: "output", Path: "/workspace/artifact/main"},
			},
		},
		Active: true,
	}
}

func testExecutionIdentity() supervisor.ExecutionIdentity {
	return supervisor.ExecutionIdentity{
		JobID:        uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		AttemptID:    uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		WorkerBootID: uuid.MustParse("33333333-3333-4333-8333-333333333333"),
	}
}

func testResourceLimits() supervisor.ResourceLimits {
	return supervisor.ResourceLimits{
		WallTime:        2 * time.Second,
		CPUTime:         time.Second,
		MemoryBytes:     128 << 20,
		Pids:            64,
		OutputBytes:     1 << 20,
		DiagnosticBytes: 64 << 10,
		FileBytes:       64 << 20,
		TmpfsBytes:      128 << 20,
		OpenFiles:       64,
	}
}
