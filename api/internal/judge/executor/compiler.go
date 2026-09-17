package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/judge/supervisor"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
)

var (
	ErrInvalidCompileRequest = errors.New("judge executor: invalid compile request")
	ErrCompileInfrastructure = errors.New("judge executor: compile infrastructure failure")
)

type CompileStatus string

const (
	CompileStatusSucceeded     CompileStatus = "succeeded"
	CompileStatusRejected      CompileStatus = "compile_error"
	CompileStatusLimitExceeded CompileStatus = "compile_limit_exceeded"
)

type CompileRequest struct {
	Identity           supervisor.ExecutionIdentity
	RuntimeProfile     problementity.RuntimeProfile
	Source             []byte
	SourceChecksum     string
	MaximumSourceBytes int64
	Limits             supervisor.ResourceLimits
}

type CompileOutcome struct {
	Status     CompileStatus
	Execution  supervisor.ExecuteResult
	Diagnostic []byte
	Artifact   *CompiledArtifact
}

type Compiler struct {
	supervisor supervisor.Supervisor
	stager     *Stager
}

func NewCompiler(
	sandbox supervisor.Supervisor,
	stager *Stager,
) (*Compiler, error) {
	if sandbox == nil || stager == nil {
		return nil, ErrInvalidCompileRequest
	}
	return &Compiler{supervisor: sandbox, stager: stager}, nil
}

func (compiler *Compiler) Compile(
	ctx context.Context,
	request CompileRequest,
) (CompileOutcome, error) {
	if compiler == nil || compiler.supervisor == nil || compiler.stager == nil ||
		ctx == nil || ctx.Err() != nil || request.Identity.JobID == uuid.Nil ||
		request.Identity.AttemptID == uuid.Nil ||
		request.Identity.WorkerBootID == uuid.Nil ||
		!request.RuntimeProfile.Active ||
		request.RuntimeProfile.ImageDigest == "" ||
		request.RuntimeProfile.ManifestChecksum == "" ||
		request.MaximumSourceBytes < 1 ||
		int64(len(request.Source)) > request.MaximumSourceBytes ||
		len(request.Source) == 0 ||
		sourceChecksum(request.Source) != request.SourceChecksum {
		return CompileOutcome{}, ErrInvalidCompileRequest
	}
	rendered, err := renderRuntimeCommand(
		request.RuntimeProfile.Compile,
		request.RuntimeProfile.Workspace,
		runtimeNumericLimits(request.Limits.MemoryBytes),
	)
	if err != nil {
		return CompileOutcome{}, err
	}
	inputFiles, err := compileInputFiles(request.RuntimeProfile, request.Source)
	if err != nil {
		return CompileOutcome{}, err
	}
	maximumArchiveBytes, err := artifactArchiveMaximum(
		request.RuntimeProfile,
		request.Limits.FileBytes,
	)
	if err != nil {
		return CompileOutcome{}, err
	}
	execution, err := compiler.supervisor.Execute(ctx, supervisor.ExecuteRequest{
		Identity:    request.Identity,
		Boundary:    supervisor.BoundaryCompile,
		ImageDigest: request.RuntimeProfile.ImageDigest,
		Argv:        rendered.argv,
		Env:         rendered.environment,
		WorkingDir:  rendered.workingDir,
		Stdin:       bytes.NewReader(nil),
		StdinBytes:  0,
		InputFiles:  inputFiles,
		Capture: &supervisor.ArchiveCapture{
			Path:         "/workspace/artifact",
			MaximumBytes: maximumArchiveBytes,
		},
		Limits: request.Limits,
	})
	if err != nil {
		return CompileOutcome{}, errors.Join(ErrCompileInfrastructure, err)
	}
	outcome := CompileOutcome{
		Execution:  execution,
		Diagnostic: execution.Stderr,
	}
	if executionLimitExceededByCompiler(execution) {
		outcome.Status = CompileStatusLimitExceeded
		return outcome, nil
	}
	if execution.ExitCode != 0 {
		outcome.Status = CompileStatusRejected
		return outcome, nil
	}
	if len(execution.OutputArchive) == 0 {
		return CompileOutcome{}, errors.Join(
			ErrCompileInfrastructure,
			errors.New("successful compiler produced no artifact archive"),
		)
	}
	artifact, err := compiler.stager.Stage(ctx, StageRequest{
		AttemptID:        request.Identity.AttemptID,
		Archive:          execution.OutputArchive,
		RuntimeProfile:   request.RuntimeProfile,
		ManifestChecksum: request.RuntimeProfile.ManifestChecksum,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidArtifact) {
			outcome.Status = CompileStatusRejected
			return outcome, nil
		}
		return CompileOutcome{}, errors.Join(ErrCompileInfrastructure, err)
	}
	outcome.Status = CompileStatusSucceeded
	outcome.Artifact = &artifact
	outcome.Execution.OutputArchive = nil
	return outcome, nil
}

func compileInputFiles(
	profile problementity.RuntimeProfile,
	source []byte,
) ([]supervisor.InputFile, error) {
	workspace := profile.Workspace
	if workspace.SourcePath == "" {
		return nil, ErrInvalidCompileRequest
	}
	files := make([]supervisor.InputFile, 0, len(workspace.TrustedFiles)+1)
	files = append(files, supervisor.InputFile{
		Path:   workspace.SourcePath,
		Mode:   fs.FileMode(0o400),
		Size:   int64(len(source)),
		Reader: bytes.NewReader(source),
	})
	for _, trusted := range workspace.TrustedFiles {
		content := []byte(trusted.Content)
		files = append(files, supervisor.InputFile{
			Path:   trusted.Path,
			Mode:   fs.FileMode(0o400),
			Size:   int64(len(content)),
			Reader: bytes.NewReader(content),
		})
	}
	return files, nil
}

func sourceChecksum(source []byte) string {
	digest := sha256.Sum256(source)
	return fmt.Sprintf("sha256:%x", digest[:])
}

func executionLimitExceededByCompiler(result supervisor.ExecuteResult) bool {
	return result.OOMKilled || result.MemoryLimitExceeded ||
		result.ProcessLimitExceeded || result.OutputLimitExceeded ||
		result.CPUTimeLimitExceeded || result.WallTimeLimitExceeded
}
