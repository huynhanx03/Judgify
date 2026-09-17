// Package supervisor owns the trusted contestant-boundary lifecycle. Docker
// and cgroup implementations are hidden behind this package.
package supervisor

import (
	"context"
	"io"
	"io/fs"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/judge/capability"
)

type BoundaryKind string

const (
	BoundaryCompile  BoundaryKind = "compile"
	BoundaryTestcase BoundaryKind = "testcase"
	BoundaryChecker  BoundaryKind = "checker"
)

type ResourceLimits struct {
	WallTime        time.Duration
	CPUTime         time.Duration
	MemoryBytes     int64
	Pids            int64
	OutputBytes     int64
	DiagnosticBytes int64
	FileBytes       int64
	TmpfsBytes      int64
	OpenFiles       int64
}

// InputFile is trusted worker-owned content. Hostile compiler archives are
// validated and rebuilt before they can become InputFile values.
type InputFile struct {
	Path   string
	Mode   fs.FileMode
	Size   int64
	Reader io.Reader
}

// ArchiveCapture requests one bounded Docker archive only after a successful
// compile boundary. The executor still treats every archive entry as hostile.
type ArchiveCapture struct {
	Path         string
	MaximumBytes int64
}

type ExecutionIdentity struct {
	JobID        uuid.UUID
	AttemptID    uuid.UUID
	WorkerBootID uuid.UUID
}

type ExecuteRequest struct {
	Identity    ExecutionIdentity
	Boundary    BoundaryKind
	ImageDigest string
	Argv        []string
	Env         []string
	WorkingDir  string
	Stdin       io.Reader
	StdinBytes  int64
	InputFiles  []InputFile
	Capture     *ArchiveCapture
	Limits      ResourceLimits
}

// ExecuteResult contains only trusted Engine/cgroup observations. Execute
// returns an error instead of a contestant result whenever either completeness
// flag cannot be established.
type ExecuteResult struct {
	ContainerID string
	CgroupID    string

	ExitCode int
	Signal   string

	WallTime        time.Duration
	CPUTime         time.Duration
	PeakMemoryBytes int64

	Stdout        []byte
	Stderr        []byte
	OutputArchive []byte

	StdoutTruncated bool
	StderrTruncated bool

	OOMKilled             bool
	MemoryLimitExceeded   bool
	ProcessLimitExceeded  bool
	OutputLimitExceeded   bool
	CPUTimeLimitExceeded  bool
	WallTimeLimitExceeded bool
	AccountingComplete    bool
	CleanupComplete       bool
}

type Supervisor interface {
	Execute(ctx context.Context, request ExecuteRequest) (ExecuteResult, error)
	Probe(ctx context.Context) (capability.CapabilityEvidence, error)
	Shutdown(ctx context.Context) error
}
