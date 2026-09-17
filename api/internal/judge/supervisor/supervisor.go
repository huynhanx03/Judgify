package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/judge/capability"
)

const (
	defaultAccountingInterval = time.Millisecond
	defaultCleanupTimeout     = 15 * time.Second
	sandboxCPUNanoQuota       = int64(1_000_000_000)
)

var (
	ErrSupervisorClosed         = errors.New("sandbox supervisor: closed")
	ErrSupervisorInfrastructure = errors.New("sandbox supervisor: infrastructure failure")
)

var sandboxStartMagic = []byte("JUDGIFY1")

type CapabilityProbe interface {
	Probe(
		ctx context.Context,
		tuple capability.CapabilityTuple,
		bootID uuid.UUID,
	) (capability.CapabilityEvidence, error)
}

type Config struct {
	Engine             Engine
	Cgroups            CgroupResolver
	Probe              CapabilityProbe
	CapabilityTuple    capability.CapabilityTuple
	BootID             uuid.UUID
	AccountingInterval time.Duration
	CleanupTimeout     time.Duration
}

type SandboxSupervisor struct {
	engine             Engine
	cgroups            CgroupResolver
	probe              CapabilityProbe
	capabilityTuple    capability.CapabilityTuple
	bootID             uuid.UUID
	accountingInterval time.Duration
	cleanupTimeout     time.Duration

	shutdownMu sync.Mutex
	mu         sync.Mutex
	closed     bool
	creating   int
	active     map[string]*ownedBoundary
	drained    chan struct{}
	drainOnce  sync.Once
	metrics    sandboxMetrics
}

type ownedBoundary struct {
	id                 string
	mu                 sync.Mutex
	started            bool
	terminated         bool
	removed            bool
	completed          bool
	shutdownRequested  bool
	currentMemoryBytes int64
}

// MetricsSnapshot contains only aggregate cgroup accounting. Boundary,
// container, job, attempt, and boot identities never leave the supervisor.
type MetricsSnapshot struct {
	CPUUsage           time.Duration
	MemoryCurrentBytes int64
	OOMEvents          uint64
	OOMKills           uint64
	CPUThrottledEvents uint64
	PIDLimitEvents     uint64
}

type sandboxMetrics struct {
	cpuNanoseconds     atomic.Uint64
	oomEvents          atomic.Uint64
	oomKills           atomic.Uint64
	cpuThrottledEvents atomic.Uint64
	pidLimitEvents     atomic.Uint64
}

func NewSandboxSupervisor(config Config) (*SandboxSupervisor, error) {
	if nilInterface(config.Engine) || nilInterface(config.Cgroups) ||
		nilInterface(config.Probe) || config.BootID == uuid.Nil {
		return nil, ErrSupervisorInfrastructure
	}
	if config.AccountingInterval == 0 {
		config.AccountingInterval = defaultAccountingInterval
	}
	if config.CleanupTimeout == 0 {
		config.CleanupTimeout = defaultCleanupTimeout
	}
	if config.AccountingInterval < time.Millisecond ||
		config.AccountingInterval > 100*time.Millisecond ||
		config.CleanupTimeout < time.Second ||
		config.CleanupTimeout > time.Minute {
		return nil, ErrSupervisorInfrastructure
	}
	return &SandboxSupervisor{
		engine:             config.Engine,
		cgroups:            config.Cgroups,
		probe:              config.Probe,
		capabilityTuple:    config.CapabilityTuple,
		bootID:             config.BootID,
		accountingInterval: config.AccountingInterval,
		cleanupTimeout:     config.CleanupTimeout,
		active:             make(map[string]*ownedBoundary),
		drained:            make(chan struct{}),
	}, nil
}

func (supervisor *SandboxSupervisor) Probe(
	ctx context.Context,
) (capability.CapabilityEvidence, error) {
	if supervisor == nil || ctx == nil {
		return capability.CapabilityEvidence{}, ErrSupervisorInfrastructure
	}
	supervisor.mu.Lock()
	closed := supervisor.closed
	supervisor.mu.Unlock()
	if closed {
		return capability.CapabilityEvidence{}, ErrSupervisorClosed
	}
	return supervisor.probe.Probe(ctx, supervisor.capabilityTuple, supervisor.bootID)
}

func (supervisor *SandboxSupervisor) Execute(
	ctx context.Context,
	request ExecuteRequest,
) (result ExecuteResult, resultErr error) {
	if supervisor == nil || ctx == nil {
		return ExecuteResult{}, ErrSupervisorInfrastructure
	}
	normalized, err := normalizeExecuteRequest(request)
	if err != nil {
		return ExecuteResult{}, err
	}
	if normalized.Identity.WorkerBootID != supervisor.bootID {
		return ExecuteResult{}, ErrInvalidExecuteRequest
	}
	if supervisor.accountingInterval > normalized.Limits.CPUTime/4 {
		return ExecuteResult{}, ErrInvalidExecuteRequest
	}
	if err := supervisor.beginCreate(); err != nil {
		return ExecuteResult{}, err
	}
	createPending := true
	defer func() {
		if createPending {
			_, _ = supervisor.finishCreate(nil)
		}
	}()

	boundaryID, err := uuid.NewV7()
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("%w: create boundary identity: %v", ErrSupervisorInfrastructure, err)
	}
	spec := buildContainerSpec(normalized, boundaryID)
	ctx = logger.WithFields(
		ctx,
		logger.String("worker_boot_id", normalized.Identity.WorkerBootID.String()),
		logger.String("boundary_id", boundaryID.String()),
		logger.String("boundary_kind", string(normalized.Boundary)),
	)
	log := logger.FromContext(ctx)
	log.Debug("creating disposable sandbox boundary")
	containerID, err := supervisor.engine.Create(ctx, spec)
	if containerID == "" {
		_, _ = supervisor.finishCreate(nil)
		createPending = false
		return ExecuteResult{}, fmt.Errorf(
			"%w: create container: %v",
			ErrSupervisorInfrastructure,
			err,
		)
	}
	result.ContainerID = containerID
	owned := &ownedBoundary{id: containerID}
	closedDuringCreate, registrationErr := supervisor.finishCreate(owned)
	createPending = false
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), supervisor.cleanupTimeout)
		defer cancel()
		cleanupErr := supervisor.cleanupOwned(cleanupCtx, owned)
		// Keep the last observed charge visible while cleanup is still running;
		// only clear active memory after the owned boundary is stopped/removed or
		// the bounded cleanup attempt has completed.
		supervisor.updateBoundaryMemory(owned, 0)
		owned.mu.Lock()
		owned.completed = true
		owned.mu.Unlock()
		if cleanupErr == nil {
			result.CleanupComplete = true
			supervisor.forget(owned)
		} else {
			resultErr = errors.Join(
				resultErr,
				fmt.Errorf("%w: cleanup container: %v", ErrSupervisorInfrastructure, cleanupErr),
			)
		}
	}()
	if err != nil || registrationErr != nil {
		return result, fmt.Errorf(
			"%w: create container: %v",
			ErrSupervisorInfrastructure,
			errors.Join(err, registrationErr),
		)
	}
	if closedDuringCreate {
		return result, ErrSupervisorClosed
	}

	if err := supervisor.putInputArchive(
		ctx,
		containerID,
		normalized.InputFiles,
		normalized.Capture != nil,
	); err != nil {
		return result, err
	}
	attach, err := supervisor.engine.Attach(ctx, containerID)
	if err != nil {
		return result, fmt.Errorf("%w: attach container: %v", ErrSupervisorInfrastructure, err)
	}
	defer func() { _ = attach.Close() }()
	overflow := make(chan struct{}, 1)
	outputDone := make(chan attachOutput, 1)
	go func() {
		outputDone <- drainDockerAttach(
			attach,
			normalized.Limits.OutputBytes,
			normalized.Limits.DiagnosticBytes,
			overflow,
		)
	}()
	if err := supervisor.engine.Start(ctx, containerID); err != nil {
		return result, fmt.Errorf("%w: start container: %v", ErrSupervisorInfrastructure, err)
	}
	owned.mu.Lock()
	owned.started = true
	owned.mu.Unlock()
	running, err := supervisor.engine.Inspect(ctx, containerID)
	if err != nil || !running.Running || running.PID <= 0 || running.Error != "" {
		return result, fmt.Errorf("%w: inspect running container", ErrSupervisorInfrastructure)
	}
	account, err := supervisor.cgroups.Resolve(ctx, containerID, running.PID)
	if err != nil {
		return result, fmt.Errorf("%w: resolve cgroup: %v", ErrSupervisorInfrastructure, err)
	}
	defer func() { _ = account.Close() }()
	result.CgroupID = account.ID()
	if result.CgroupID == "" {
		return result, ErrSupervisorInfrastructure
	}
	baseline, err := account.Snapshot(ctx)
	if err != nil {
		return result, fmt.Errorf("%w: baseline accounting: %v", ErrSupervisorInfrastructure, err)
	}
	supervisor.updateBoundaryMemory(owned, baseline.MemoryCurrentBytes)

	lifecycleCtx, lifecycleCancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		normalized.Limits.WallTime+supervisor.cleanupTimeout,
	)
	defer lifecycleCancel()
	waitDone := make(chan waitOutcome, 1)
	go func() {
		wait, waitErr := supervisor.engine.Wait(lifecycleCtx, containerID)
		waitDone <- waitOutcome{result: wait, err: waitErr}
	}()
	stdinDone := make(chan error, 1)
	go func() {
		stdinDone <- writeSandboxInput(
			attach,
			normalized.Stdin,
			normalized.StdinBytes,
		)
	}()

	startedAt := time.Now()
	timer := time.NewTimer(normalized.Limits.WallTime)
	defer timer.Stop()
	ticker := time.NewTicker(supervisor.accountingInterval)
	defer ticker.Stop()

	var wait waitOutcome
	var waitComplete bool
	var infrastructureErr error
	var lastSnapshot = baseline
	var capturedOutput *attachOutput
	callerDone := ctx.Done()
	var killOnce sync.Once
	kill := func() {
		killOnce.Do(func() {
			if killErr := supervisor.engine.Kill(lifecycleCtx, containerID); killErr != nil &&
				!errors.Is(killErr, ErrEngineContainerNotRunning) &&
				!errors.Is(killErr, ErrEngineContainerNotFound) {
				infrastructureErr = errors.Join(infrastructureErr, killErr)
			}
		})
	}

	for !waitComplete {
		select {
		case wait = <-waitDone:
			waitComplete = true
		case <-overflow:
			result.OutputLimitExceeded = true
			kill()
		case <-timer.C:
			result.WallTimeLimitExceeded = true
			kill()
		case <-ticker.C:
			snapshot, snapshotErr := account.Snapshot(lifecycleCtx)
			if snapshotErr != nil {
				infrastructureErr = errors.Join(infrastructureErr, snapshotErr)
				kill()
				continue
			}
			lastSnapshot = snapshot
			supervisor.updateBoundaryMemory(owned, snapshot.MemoryCurrentBytes)
			delta, deltaErr := accountingDelta(baseline, snapshot)
			if deltaErr != nil {
				infrastructureErr = errors.Join(infrastructureErr, deltaErr)
				kill()
				continue
			}
			applyResourceLimits(&result, normalized.Limits, delta, kill)
		case <-callerDone:
			infrastructureErr = errors.Join(infrastructureErr, ctx.Err())
			kill()
			callerDone = nil
		case output := <-outputDone:
			if output.err != nil {
				infrastructureErr = errors.Join(infrastructureErr, output.err)
				kill()
			}
			capturedOutput = &output
			outputDone = nil
		case inputErr := <-stdinDone:
			if inputErr != nil && !benignAttachWriteError(inputErr) {
				infrastructureErr = errors.Join(infrastructureErr, inputErr)
				kill()
			}
			stdinDone = nil
		}
	}
	owned.mu.Lock()
	owned.terminated = true
	shutdownRequested := owned.shutdownRequested
	owned.mu.Unlock()
	result.WallTime = time.Since(startedAt)
	if shutdownRequested {
		infrastructureErr = errors.Join(infrastructureErr, ErrSupervisorClosed)
	}
	if wait.err != nil || wait.result.Error != "" || wait.result.StatusCode < 0 {
		infrastructureErr = errors.Join(infrastructureErr, wait.err)
	}

	finalSnapshot, err := account.Snapshot(lifecycleCtx)
	if err != nil {
		infrastructureErr = errors.Join(infrastructureErr, err)
		finalSnapshot = lastSnapshot
	}
	delta, err := accountingDelta(baseline, finalSnapshot)
	if err != nil {
		infrastructureErr = errors.Join(infrastructureErr, err)
	} else {
		supervisor.updateBoundaryMemory(owned, finalSnapshot.MemoryCurrentBytes)
		supervisor.recordAccounting(delta)
		applyResourceLimits(&result, normalized.Limits, delta, func() {})
		result.CPUTime = delta.CPUUsage
		result.PeakMemoryBytes = delta.MemoryPeakBytes
		result.OOMKilled = delta.OOMKills != 0
		result.ProcessLimitExceeded = result.ProcessLimitExceeded || delta.PIDLimitEvents != 0
	}
	exited, err := supervisor.engine.Inspect(lifecycleCtx, containerID)
	if err != nil || exited.Running || exited.PID != 0 || exited.Error != "" ||
		exited.ExitCode != wait.result.StatusCode {
		infrastructureErr = errors.Join(infrastructureErr, err, ErrSupervisorInfrastructure)
	} else {
		result.ExitCode = exited.ExitCode
		result.OOMKilled = result.OOMKilled || exited.OOMKilled
	}

	if capturedOutput == nil {
		select {
		case output := <-outputDone:
			capturedOutput = &output
		case <-lifecycleCtx.Done():
			infrastructureErr = errors.Join(infrastructureErr, lifecycleCtx.Err())
		}
	}
	if capturedOutput != nil {
		output := *capturedOutput
		if output.err != nil {
			infrastructureErr = errors.Join(infrastructureErr, output.err)
		}
		result.Stdout = output.stdout
		result.Stderr = output.stderr
		result.StdoutTruncated = output.stdoutTruncated
		result.StderrTruncated = output.stderrTruncated
		result.OutputLimitExceeded = result.OutputLimitExceeded ||
			output.stdoutTruncated || output.stderrTruncated
	}
	if normalized.Capture != nil && infrastructureErr == nil &&
		result.ExitCode == 0 && !executionLimitExceeded(result) {
		archive, archiveErr := supervisor.engine.GetArchive(
			lifecycleCtx,
			containerID,
			normalized.Capture.Path,
		)
		if archiveErr == nil {
			result.OutputArchive, archiveErr = readArchiveBounded(
				archive,
				normalized.Capture.MaximumBytes,
			)
			archiveErr = errors.Join(archiveErr, archive.Close())
		}
		if archiveErr != nil {
			infrastructureErr = errors.Join(infrastructureErr, archiveErr)
		}
	}
	if stdinDone != nil {
		_ = attach.Close()
		select {
		case inputErr := <-stdinDone:
			if inputErr != nil && !benignAttachWriteError(inputErr) {
				infrastructureErr = errors.Join(infrastructureErr, inputErr)
			}
		case <-lifecycleCtx.Done():
			infrastructureErr = errors.Join(infrastructureErr, lifecycleCtx.Err())
		}
	}
	result.MemoryLimitExceeded = result.MemoryLimitExceeded || result.OOMKilled
	result.AccountingComplete = infrastructureErr == nil
	if infrastructureErr != nil {
		log.Error("sandbox boundary failed closed", zap.Error(infrastructureErr))
		return result, fmt.Errorf("%w: %v", ErrSupervisorInfrastructure, infrastructureErr)
	}
	log.Debug(
		"sandbox boundary completed",
		zap.Int("exit_code", result.ExitCode),
		zap.Duration("wall_time", result.WallTime),
		zap.Duration("cpu_time", result.CPUTime),
		zap.Int64("peak_memory_bytes", result.PeakMemoryBytes),
	)
	return result, nil
}

// Metrics returns a lock-consistent aggregate over active boundaries plus
// monotonic completed accounting.
func (supervisor *SandboxSupervisor) Metrics() MetricsSnapshot {
	if supervisor == nil {
		return MetricsSnapshot{}
	}
	supervisor.mu.Lock()
	boundaries := make([]*ownedBoundary, 0, len(supervisor.active))
	for _, owned := range supervisor.active {
		boundaries = append(boundaries, owned)
	}
	supervisor.mu.Unlock()
	var currentMemory int64
	for _, owned := range boundaries {
		owned.mu.Lock()
		value := owned.currentMemoryBytes
		owned.mu.Unlock()
		if value > 0 && currentMemory <= int64(^uint64(0)>>1)-value {
			currentMemory += value
		}
	}
	return MetricsSnapshot{
		CPUUsage:           time.Duration(supervisor.metrics.cpuNanoseconds.Load()),
		MemoryCurrentBytes: currentMemory,
		OOMEvents:          supervisor.metrics.oomEvents.Load(),
		OOMKills:           supervisor.metrics.oomKills.Load(),
		CPUThrottledEvents: supervisor.metrics.cpuThrottledEvents.Load(),
		PIDLimitEvents:     supervisor.metrics.pidLimitEvents.Load(),
	}
}

func (supervisor *SandboxSupervisor) updateBoundaryMemory(owned *ownedBoundary, value int64) {
	if supervisor == nil || owned == nil || value < 0 {
		return
	}
	owned.mu.Lock()
	owned.currentMemoryBytes = value
	owned.mu.Unlock()
}

func (supervisor *SandboxSupervisor) recordAccounting(delta ResourceDelta) {
	if supervisor == nil || delta.CPUUsage < 0 {
		return
	}
	supervisor.metrics.cpuNanoseconds.Add(uint64(delta.CPUUsage))
	supervisor.metrics.oomEvents.Add(delta.OOMEvents)
	supervisor.metrics.oomKills.Add(delta.OOMKills)
	supervisor.metrics.cpuThrottledEvents.Add(delta.CPUThrottledEvents)
	supervisor.metrics.pidLimitEvents.Add(delta.PIDLimitEvents)
}

func readArchiveBounded(source io.Reader, maximumBytes int64) ([]byte, error) {
	if source == nil || maximumBytes < 1 {
		return nil, ErrSupervisorInfrastructure
	}
	value, err := io.ReadAll(io.LimitReader(source, maximumBytes+1))
	if err != nil || int64(len(value)) > maximumBytes {
		return nil, fmt.Errorf("%w: output archive exceeds bound", ErrSupervisorInfrastructure)
	}
	return value, nil
}

func executionLimitExceeded(result ExecuteResult) bool {
	return result.OOMKilled || result.MemoryLimitExceeded ||
		result.ProcessLimitExceeded || result.OutputLimitExceeded ||
		result.CPUTimeLimitExceeded || result.WallTimeLimitExceeded
}

type waitOutcome struct {
	result WaitResult
	err    error
}

func writeSandboxInput(
	destination AttachConnection,
	source io.Reader,
	size int64,
) error {
	if err := writeAll(destination, sandboxStartMagic); err != nil {
		return err
	}
	written, err := io.CopyN(destination, source, size)
	if err != nil || written != size {
		return fmt.Errorf("%w: stream testcase input", ErrSupervisorInfrastructure)
	}
	return destination.CloseWrite()
}

func writeAll(destination io.Writer, value []byte) error {
	for len(value) != 0 {
		written, err := destination.Write(value)
		if err != nil {
			return err
		}
		if written <= 0 || written > len(value) {
			return io.ErrShortWrite
		}
		value = value[written:]
	}
	return nil
}

func applyResourceLimits(
	result *ExecuteResult,
	limits ResourceLimits,
	delta ResourceDelta,
	kill func(),
) {
	if delta.OOMKills != 0 || delta.MemoryPeakBytes > limits.MemoryBytes {
		result.MemoryLimitExceeded = true
		kill()
	}
	if delta.PIDLimitEvents != 0 {
		result.ProcessLimitExceeded = true
		kill()
	}
	if delta.CPUUsage >= limits.CPUTime {
		result.CPUTimeLimitExceeded = true
		kill()
	}
}

func buildContainerSpec(request normalizedRequest, boundaryID uuid.UUID) ContainerSpec {
	tmpfs := fmt.Sprintf(
		"rw,exec,nosuid,nodev,size=%d,nr_inodes=4096,uid=65532,gid=65532,mode=0700",
		request.Limits.TmpfsBytes,
	)
	return ContainerSpec{
		Name:        "judgify-" + string(request.Boundary) + "-" + boundaryID.String(),
		ImageDigest: request.ImageDigest,
		Hostname:    "sandbox",
		User:        request.User,
		WorkingDir:  request.WorkingDir,
		Command:     append([]string(nil), request.Command...),
		Environment: append([]string(nil), request.Env...),
		Labels: map[string]string{
			"io.judgify.managed":        "true",
			"io.judgify.job_id":         request.Identity.JobID.String(),
			"io.judgify.attempt_id":     request.Identity.AttemptID.String(),
			"io.judgify.worker_boot_id": request.Identity.WorkerBootID.String(),
			"io.judgify.boundary_id":    boundaryID.String(),
			"io.judgify.boundary_kind":  string(request.Boundary),
		},
		NetworkMode:      request.NetworkMode,
		ReadOnlyRoot:     request.ReadOnlyRoot,
		Privileged:       request.Privileged,
		DropCapabilities: append([]string(nil), request.Capabilities...),
		SecurityOptions:  append([]string(nil), request.SecurityOpts...),
		IPCMode:          "none",
		MemoryBytes:      request.Limits.MemoryBytes,
		MemorySwapBytes:  request.Limits.MemoryBytes,
		NanoCPUs:         sandboxCPUNanoQuota,
		PidsLimit:        request.Limits.Pids,
		Tmpfs: map[string]string{
			"/workspace": tmpfs,
			"/tmp": fmt.Sprintf(
				"rw,noexec,nosuid,nodev,size=%d,nr_inodes=4096,uid=65532,gid=65532,mode=0700",
				request.Limits.TmpfsBytes,
			),
		},
		Ulimits: []Ulimit{
			{Name: "core", Soft: 0, Hard: 0},
			{Name: "fsize", Soft: request.Limits.FileBytes, Hard: request.Limits.FileBytes},
			{Name: "nofile", Soft: request.Limits.OpenFiles, Hard: request.Limits.OpenFiles},
		},
	}
}

func (supervisor *SandboxSupervisor) beginCreate() error {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if supervisor.closed {
		return ErrSupervisorClosed
	}
	supervisor.creating++
	return nil
}

// finishCreate atomically transfers one in-progress create into owned active
// state. If shutdown began meanwhile, the caller still owns cleanup and must
// return before drained can close.
func (supervisor *SandboxSupervisor) finishCreate(
	owned *ownedBoundary,
) (closed bool, err error) {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if supervisor.creating <= 0 {
		return supervisor.closed, ErrSupervisorInfrastructure
	}
	supervisor.creating--
	if owned != nil {
		if _, duplicate := supervisor.active[owned.id]; duplicate {
			err = ErrSupervisorInfrastructure
		} else {
			supervisor.active[owned.id] = owned
		}
		if supervisor.closed {
			owned.mu.Lock()
			owned.shutdownRequested = true
			owned.mu.Unlock()
		}
	}
	supervisor.signalDrainedLocked()
	return supervisor.closed, err
}

func (supervisor *SandboxSupervisor) forget(owned *ownedBoundary) {
	supervisor.mu.Lock()
	if supervisor.active[owned.id] == owned {
		delete(supervisor.active, owned.id)
	}
	supervisor.signalDrainedLocked()
	supervisor.mu.Unlock()
}

func (supervisor *SandboxSupervisor) signalDrainedLocked() {
	if supervisor.closed && supervisor.creating == 0 &&
		len(supervisor.active) == 0 {
		supervisor.drainOnce.Do(func() { close(supervisor.drained) })
	}
}

func (supervisor *SandboxSupervisor) cleanupOwned(
	ctx context.Context,
	owned *ownedBoundary,
) error {
	owned.mu.Lock()
	defer owned.mu.Unlock()
	if owned.removed {
		return nil
	}
	var cleanupErr error
	if owned.started && !owned.terminated {
		if err := supervisor.engine.Kill(ctx, owned.id); err != nil &&
			!errors.Is(err, ErrEngineContainerNotRunning) &&
			!errors.Is(err, ErrEngineContainerNotFound) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
		if _, err := supervisor.engine.Wait(ctx, owned.id); err != nil &&
			!errors.Is(err, ErrEngineContainerNotFound) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	if err := supervisor.engine.Remove(ctx, owned.id, true); err != nil &&
		!errors.Is(err, ErrEngineContainerNotFound) {
		cleanupErr = errors.Join(cleanupErr, err)
	} else {
		owned.removed = true
	}
	return cleanupErr
}

func (supervisor *SandboxSupervisor) Shutdown(ctx context.Context) error {
	if supervisor == nil {
		return nil
	}
	if ctx == nil {
		return ErrSupervisorInfrastructure
	}
	supervisor.shutdownMu.Lock()
	defer supervisor.shutdownMu.Unlock()
	supervisor.mu.Lock()
	supervisor.closed = true
	active := make([]*ownedBoundary, 0, len(supervisor.active))
	for _, owned := range supervisor.active {
		active = append(active, owned)
	}
	supervisor.signalDrainedLocked()
	supervisor.mu.Unlock()

	var failures []error
	for _, owned := range active {
		owned.mu.Lock()
		owned.shutdownRequested = true
		owned.mu.Unlock()
		if err := supervisor.cleanupOwned(ctx, owned); err != nil {
			failures = append(failures, err)
			continue
		}
		owned.mu.Lock()
		completed := owned.completed
		owned.mu.Unlock()
		if completed {
			supervisor.forget(owned)
		}
	}
	if len(failures) != 0 {
		return errors.Join(failures...)
	}
	select {
	case <-supervisor.drained:
	case <-ctx.Done():
		return ctx.Err()
	}
	return supervisor.engine.Close()
}

func benignAttachWriteError(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.EPIPE)
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var _ Supervisor = (*SandboxSupervisor)(nil)
