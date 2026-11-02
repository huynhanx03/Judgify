package supervisor

import (
	"context"
	"errors"
	"io"
)

var (
	ErrEngineContainerNotFound   = errors.New("sandbox engine: container not found")
	ErrEngineContainerNotRunning = errors.New("sandbox engine: container not running")
	ErrEngineUnavailable         = errors.New("sandbox engine: unavailable")
)

type Ulimit struct {
	Name string
	Soft int64
	Hard int64
}

// ContainerSpec is the complete closed Docker/OCI policy produced by the
// supervisor. No job-originated raw Docker option reaches this boundary.
type ContainerSpec struct {
	Name        string
	ImageDigest string
	Hostname    string
	User        string
	WorkingDir  string
	Command     []string
	Environment []string
	Labels      map[string]string

	NetworkMode      string
	ReadOnlyRoot     bool
	Privileged       bool
	DropCapabilities []string
	SecurityOptions  []string
	IPCMode          string
	PIDMode          string
	UTSMode          string

	MemoryBytes     int64
	MemorySwapBytes int64
	NanoCPUs        int64
	PidsLimit       int64
	Tmpfs           map[string]string
	Ulimits         []Ulimit
}

type ContainerState struct {
	Running   bool
	PID       int
	ExitCode  int
	OOMKilled bool
	Error     string
}

type WaitResult struct {
	StatusCode int
	Error      string
}

type AttachConnection interface {
	io.ReadWriteCloser
	CloseWrite() error
}

// Engine is intentionally small and typed so the production adapter can use
// Docker Engine HTTP while unit tests prove lifecycle behavior without Docker.
type Engine interface {
	Create(ctx context.Context, spec ContainerSpec) (string, error)
	PutArchive(ctx context.Context, containerID string, destination string, archive io.Reader) error
	GetArchive(ctx context.Context, containerID string, source string) (io.ReadCloser, error)
	Attach(ctx context.Context, containerID string) (AttachConnection, error)
	Start(ctx context.Context, containerID string) error
	Inspect(ctx context.Context, containerID string) (ContainerState, error)
	Wait(ctx context.Context, containerID string) (WaitResult, error)
	Kill(ctx context.Context, containerID string) error
	Remove(ctx context.Context, containerID string, force bool) error
	Close() error
}
