package executor

import (
	"bytes"
	"context"
	"errors"
	"io/fs"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/judge/supervisor"
)

var (
	ErrInvalidTestcaseRequest = errors.New("judge executor: invalid testcase request")
	ErrTestcaseInfrastructure = errors.New("judge executor: testcase infrastructure failure")
)

type TestcaseRequest struct {
	Identity   supervisor.ExecutionIdentity
	TestcaseID uuid.UUID
	Artifact   *CompiledArtifact
	Input      []byte
	Limits     supervisor.ResourceLimits
}

type CaseResult struct {
	TestcaseID uuid.UUID
	Execution  supervisor.ExecuteResult
	Output     []byte
}

type TestcaseExecutor struct {
	supervisor supervisor.Supervisor
}

func NewTestcaseExecutor(sandbox supervisor.Supervisor) (*TestcaseExecutor, error) {
	if sandbox == nil {
		return nil, ErrInvalidTestcaseRequest
	}
	return &TestcaseExecutor{supervisor: sandbox}, nil
}

func (executor *TestcaseExecutor) Run(
	ctx context.Context,
	request TestcaseRequest,
) (CaseResult, error) {
	if executor == nil || executor.supervisor == nil || ctx == nil ||
		ctx.Err() != nil || request.TestcaseID == uuid.Nil ||
		request.Identity.JobID == uuid.Nil || request.Identity.AttemptID == uuid.Nil ||
		request.Identity.WorkerBootID == uuid.Nil || request.Artifact == nil ||
		request.Artifact.AttemptID != request.Identity.AttemptID ||
		request.Artifact.ManifestChecksum == "" ||
		request.Artifact.ManifestChecksum != request.Artifact.RuntimeProfile.ManifestChecksum ||
		int64(len(request.Input)) > request.Limits.FileBytes {
		return CaseResult{}, ErrInvalidTestcaseRequest
	}
	profile := request.Artifact.RuntimeProfile
	rendered, err := renderRuntimeCommand(
		profile.Run,
		profile.Workspace,
		runtimeNumericLimits(request.Limits.MemoryBytes),
	)
	if err != nil {
		return CaseResult{}, err
	}
	opened, err := request.Artifact.OpenFiles()
	if err != nil {
		return CaseResult{}, errors.Join(ErrTestcaseInfrastructure, err)
	}
	defer closeFiles(opened)
	inputFiles := make([]supervisor.InputFile, 0, len(opened))
	for index, file := range opened {
		metadata := request.Artifact.Files[index]
		mode := fs.FileMode(0o400)
		if metadata.Executable {
			mode = 0o500
		}
		inputFiles = append(inputFiles, supervisor.InputFile{
			Path:   "artifact/" + metadata.Path,
			Mode:   mode,
			Size:   metadata.Size,
			Reader: file,
		})
	}
	execution, err := executor.supervisor.Execute(ctx, supervisor.ExecuteRequest{
		Identity:    request.Identity,
		Boundary:    supervisor.BoundaryTestcase,
		ImageDigest: profile.ImageDigest,
		Argv:        rendered.argv,
		Env:         rendered.environment,
		WorkingDir:  rendered.workingDir,
		Stdin:       bytes.NewReader(request.Input),
		StdinBytes:  int64(len(request.Input)),
		InputFiles:  inputFiles,
		Limits:      request.Limits,
	})
	if err != nil {
		return CaseResult{}, errors.Join(ErrTestcaseInfrastructure, err)
	}
	return CaseResult{
		TestcaseID: request.TestcaseID,
		Execution:  execution,
		Output:     execution.Stdout,
	}, nil
}
