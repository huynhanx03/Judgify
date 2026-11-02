package supervisor

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/judge/capability"
)

func TestSupervisorCreatesOneDisposableLeastPrivilegeBoundary(t *testing.T) {
	t.Parallel()

	engine := newFakeEngine(multiplexedOutput(1, []byte("answer\n")))
	cgroups := &fakeCgroupResolver{snapshots: []ResourceSnapshot{
		{CPUUsage: time.Second, MemoryCurrentBytes: 1024, MemoryPeakBytes: 2048},
		{CPUUsage: 1100 * time.Millisecond, MemoryCurrentBytes: 2048, MemoryPeakBytes: 4096},
	}}
	supervisor := newTestSupervisor(t, engine, cgroups)
	request := validExecuteRequest()
	result, err := supervisor.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if string(result.Stdout) != "answer\n" || result.ExitCode != 0 ||
		result.CPUTime != 100*time.Millisecond ||
		result.PeakMemoryBytes != 4096 ||
		!result.AccountingComplete || !result.CleanupComplete {
		t.Fatalf("Execute() = %+v", result)
	}
	spec := engine.createdSpec()
	if spec.ImageDigest != request.ImageDigest || spec.User != sandboxUser ||
		spec.NetworkMode != "none" || !spec.ReadOnlyRoot || spec.Privileged ||
		spec.MemoryBytes != request.Limits.MemoryBytes ||
		spec.MemorySwapBytes != request.Limits.MemoryBytes ||
		spec.PidsLimit != request.Limits.Pids ||
		spec.Command[0] != sandboxInitPath {
		t.Fatalf("Create spec is not the closed profile: %+v", spec)
	}
	if len(spec.DropCapabilities) != 1 || spec.DropCapabilities[0] != "ALL" ||
		len(spec.SecurityOptions) != 2 ||
		spec.Labels["io.judgify.managed"] != "true" ||
		spec.Labels["io.judgify.job_id"] != request.Identity.JobID.String() {
		t.Fatalf("Create security/labels = %+v", spec)
	}
	if got := engine.eventNames(); !equalStrings(got, []string{
		"create", "archive", "attach", "start", "inspect-running",
		"wait", "inspect-exited", "remove",
	}) {
		t.Fatalf("engine events = %v", got)
	}
	if written := engine.attachedInput(); !bytes.Equal(
		written,
		append(append([]byte(nil), sandboxStartMagic...), []byte("1 2\n")...),
	) {
		t.Fatalf("attached stdin = %q", written)
	}
}

func TestSupervisorKillsAndRemovesOnOutputLimit(t *testing.T) {
	t.Parallel()

	engine := newFakeEngine(multiplexedOutput(1, bytes.Repeat([]byte("x"), 32)))
	engine.waitForKill = true
	supervisor := newTestSupervisor(t, engine, &fakeCgroupResolver{
		snapshots: []ResourceSnapshot{
			{MemoryCurrentBytes: 1, MemoryPeakBytes: 1},
			{MemoryCurrentBytes: 1, MemoryPeakBytes: 1},
		},
	})
	request := validExecuteRequest()
	request.Limits.OutputBytes = 8
	result, err := supervisor.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.OutputLimitExceeded || !result.StdoutTruncated ||
		len(result.Stdout) != 8 || !result.CleanupComplete {
		t.Fatalf("Execute() = %+v", result)
	}
	if !containsString(engine.eventNames(), "kill") ||
		!containsString(engine.eventNames(), "remove") {
		t.Fatalf("engine events = %v, want kill/remove", engine.eventNames())
	}
}

func TestSupervisorKillsAndRemovesOnWallDeadline(t *testing.T) {
	t.Parallel()

	engine := newFakeEngine(nil)
	engine.waitForKill = true
	supervisor := newTestSupervisor(t, engine, &fakeCgroupResolver{
		snapshots: []ResourceSnapshot{
			{MemoryCurrentBytes: 1, MemoryPeakBytes: 1},
			{MemoryCurrentBytes: 1, MemoryPeakBytes: 1},
		},
	})
	request := validExecuteRequest()
	request.Limits.WallTime = 10 * time.Millisecond
	result, err := supervisor.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.WallTimeLimitExceeded || !result.CleanupComplete {
		t.Fatalf("Execute() = %+v", result)
	}
}

func TestSupervisorTreatsAccountingAndCleanupUncertaintyAsInfrastructure(t *testing.T) {
	t.Parallel()

	engine := newFakeEngine(nil)
	engine.removeErr = errors.New("daemon refused cleanup")
	supervisor := newTestSupervisor(t, engine, &fakeCgroupResolver{
		err: errors.New("cgroup disappeared"),
	})
	result, err := supervisor.Execute(context.Background(), validExecuteRequest())
	if !errors.Is(err, ErrSupervisorInfrastructure) ||
		result.AccountingComplete || result.CleanupComplete {
		t.Fatalf("Execute() = (%+v, %v), want fail-closed infrastructure error", result, err)
	}
}

func TestSupervisorUsesFreshContainerForEveryBoundary(t *testing.T) {
	t.Parallel()

	engine := newFakeEngine(nil)
	supervisor := newTestSupervisor(t, engine, &fakeCgroupResolver{
		snapshots: []ResourceSnapshot{
			{MemoryCurrentBytes: 1, MemoryPeakBytes: 1},
			{MemoryCurrentBytes: 1, MemoryPeakBytes: 1},
		},
	})
	first, err := supervisor.Execute(context.Background(), validExecuteRequest())
	if err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	secondRequest := validExecuteRequest()
	secondRequest.Identity.AttemptID = uuid.MustParse("44444444-4444-4444-8444-444444444444")
	second, err := supervisor.Execute(context.Background(), secondRequest)
	if err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}
	if first.ContainerID == second.ContainerID || engine.createCount() != 2 ||
		engine.removeCount() != 2 {
		t.Fatalf(
			"container reuse detected: first=%q second=%q creates=%d removes=%d",
			first.ContainerID,
			second.ContainerID,
			engine.createCount(),
			engine.removeCount(),
		)
	}
}

func TestSupervisorDefaultAccountingSupportsPublishedMinimumCPUContract(t *testing.T) {
	t.Parallel()

	engine := newFakeEngine(nil)
	supervisor, err := NewSandboxSupervisor(Config{
		Engine: engine,
		Cgroups: &fakeCgroupResolver{snapshots: []ResourceSnapshot{
			{MemoryCurrentBytes: 1, MemoryPeakBytes: 1},
			{MemoryCurrentBytes: 1, MemoryPeakBytes: 1},
		}},
		Probe:          fakeCapabilityProbe{},
		BootID:         uuid.MustParse("33333333-3333-4333-8333-333333333333"),
		CleanupTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewSandboxSupervisor() error = %v", err)
	}
	t.Cleanup(func() { _ = supervisor.Shutdown(context.Background()) })

	request := validExecuteRequest()
	request.Limits.CPUTime = 10 * time.Millisecond
	request.Limits.WallTime = 10 * time.Millisecond
	if _, err := supervisor.Execute(context.Background(), request); err != nil {
		t.Fatalf(
			"Execute() error = %v, default accounting must support the published minimum CPU limit",
			err,
		)
	}
}

func TestSupervisorShutdownInterruptsBoundariesAndClosesEngineAfterDrain(t *testing.T) {
	t.Parallel()

	engine := newFakeEngine(nil)
	engine.waitForKill = true
	supervisor := newTestSupervisor(t, engine, &fakeCgroupResolver{
		snapshots: []ResourceSnapshot{
			{MemoryCurrentBytes: 1, MemoryPeakBytes: 1},
			{MemoryCurrentBytes: 1, MemoryPeakBytes: 1},
		},
	})
	executeDone := make(chan error, 1)
	go func() {
		_, err := supervisor.Execute(context.Background(), validExecuteRequest())
		executeDone <- err
	}()
	waitForFakeEngineEvent(t, engine, "start")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := supervisor.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	select {
	case err := <-executeDone:
		if err == nil {
			t.Fatal("Execute() succeeded after shutdown interrupted its boundary")
		}
	case <-time.After(time.Second):
		t.Fatal("Execute() did not drain before Shutdown() returned")
	}
	events := engine.eventNames()
	closeIndex := indexOfString(events, "close")
	removeIndex := indexOfString(events, "remove")
	if closeIndex < 0 || removeIndex < 0 || closeIndex < removeIndex {
		t.Fatalf("engine events = %v, close must follow boundary removal", events)
	}
}

func TestSupervisorShutdownDrainsContainerCreationBeforeEngineClose(t *testing.T) {
	t.Parallel()

	engine := newFakeEngine(nil)
	engine.createEntered = make(chan struct{})
	engine.continueCreate = make(chan struct{})
	supervisor := newTestSupervisor(t, engine, &fakeCgroupResolver{})

	executeDone := make(chan error, 1)
	go func() {
		_, err := supervisor.Execute(context.Background(), validExecuteRequest())
		executeDone <- err
	}()
	select {
	case <-engine.createEntered:
	case <-time.After(time.Second):
		t.Fatal("Execute() did not enter container creation")
	}

	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		shutdownDone <- supervisor.Shutdown(ctx)
	}()
	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown() returned before create ownership transferred: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(engine.continueCreate)

	if err := <-executeDone; !errors.Is(err, ErrSupervisorClosed) {
		t.Fatalf("Execute() error = %v, want ErrSupervisorClosed", err)
	}
	if err := <-shutdownDone; err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	events := engine.eventNames()
	removeIndex := indexOfString(events, "remove")
	closeIndex := indexOfString(events, "close")
	if removeIndex < 0 || closeIndex < 0 || closeIndex < removeIndex {
		t.Fatalf("engine events = %v, cleanup must precede close", events)
	}
}

func newTestSupervisor(
	t *testing.T,
	engine Engine,
	cgroups CgroupResolver,
) *SandboxSupervisor {
	t.Helper()
	value, err := NewSandboxSupervisor(Config{
		Engine:             engine,
		Cgroups:            cgroups,
		Probe:              fakeCapabilityProbe{},
		BootID:             uuid.MustParse("33333333-3333-4333-8333-333333333333"),
		AccountingInterval: time.Millisecond,
		CleanupTimeout:     time.Second,
	})
	if err != nil {
		t.Fatalf("NewSandboxSupervisor() error = %v", err)
	}
	t.Cleanup(func() { _ = value.Shutdown(context.Background()) })
	return value
}

type fakeCapabilityProbe struct{}

func (fakeCapabilityProbe) Probe(
	context.Context,
	capability.CapabilityTuple,
	uuid.UUID,
) (capability.CapabilityEvidence, error) {
	return capability.CapabilityEvidence{Passing: true}, nil
}

type fakeEngine struct {
	mu             sync.Mutex
	events         []string
	specs          []ContainerSpec
	attach         *fakeAttach
	nextID         int
	running        map[string]bool
	waitForKill    bool
	killed         chan struct{}
	removeErr      error
	createEntered  chan struct{}
	continueCreate chan struct{}
}

func newFakeEngine(output []byte) *fakeEngine {
	return &fakeEngine{
		attach:  &fakeAttach{reader: bytes.NewReader(output)},
		killed:  make(chan struct{}),
		running: make(map[string]bool),
	}
}

func (engine *fakeEngine) Create(ctx context.Context, spec ContainerSpec) (string, error) {
	engine.mu.Lock()
	engine.nextID++
	engine.specs = append(engine.specs, spec)
	engine.events = append(engine.events, "create")
	nextID := engine.nextID
	createEntered := engine.createEntered
	continueCreate := engine.continueCreate
	engine.mu.Unlock()
	if createEntered != nil {
		select {
		case <-createEntered:
		default:
			close(createEntered)
		}
	}
	if continueCreate != nil {
		select {
		case <-continueCreate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "container-" + string(rune('0'+nextID)), nil
}

func (engine *fakeEngine) PutArchive(_ context.Context, _ string, _ string, archive io.Reader) error {
	engine.record("archive")
	_, err := io.Copy(io.Discard, archive)
	return err
}

func (engine *fakeEngine) GetArchive(context.Context, string, string) (io.ReadCloser, error) {
	engine.record("get-archive")
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (engine *fakeEngine) Attach(context.Context, string) (AttachConnection, error) {
	engine.record("attach")
	return engine.attach, nil
}

func (engine *fakeEngine) Start(_ context.Context, containerID string) error {
	engine.mu.Lock()
	engine.running[containerID] = true
	engine.events = append(engine.events, "start")
	engine.mu.Unlock()
	return nil
}

func (engine *fakeEngine) Inspect(_ context.Context, containerID string) (ContainerState, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if !engine.running[containerID] {
		engine.events = append(engine.events, "inspect-exited")
		return ContainerState{ExitCode: 0}, nil
	}
	engine.events = append(engine.events, "inspect-running")
	return ContainerState{Running: true, PID: 42}, nil
}

func (engine *fakeEngine) Wait(ctx context.Context, containerID string) (WaitResult, error) {
	if engine.waitForKill {
		select {
		case <-engine.killed:
		case <-ctx.Done():
			return WaitResult{}, ctx.Err()
		}
	}
	engine.mu.Lock()
	engine.running[containerID] = false
	engine.events = append(engine.events, "wait")
	engine.mu.Unlock()
	return WaitResult{StatusCode: 0}, nil
}

func (engine *fakeEngine) Kill(_ context.Context, containerID string) error {
	engine.mu.Lock()
	engine.running[containerID] = false
	engine.events = append(engine.events, "kill")
	engine.mu.Unlock()
	select {
	case <-engine.killed:
	default:
		close(engine.killed)
	}
	return nil
}

func (engine *fakeEngine) Remove(context.Context, string, bool) error {
	engine.record("remove")
	return engine.removeErr
}

func (engine *fakeEngine) Close() error {
	engine.record("close")
	return nil
}

func (engine *fakeEngine) record(event string) {
	engine.mu.Lock()
	engine.events = append(engine.events, event)
	engine.mu.Unlock()
}

func (engine *fakeEngine) eventNames() []string {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return append([]string(nil), engine.events...)
}

func (engine *fakeEngine) createdSpec() ContainerSpec {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return engine.specs[0]
}

func (engine *fakeEngine) attachedInput() []byte {
	engine.attach.mu.Lock()
	defer engine.attach.mu.Unlock()
	return append([]byte(nil), engine.attach.written.Bytes()...)
}

func (engine *fakeEngine) createCount() int {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return len(engine.specs)
}

func (engine *fakeEngine) removeCount() int {
	count := 0
	for _, event := range engine.eventNames() {
		if event == "remove" {
			count++
		}
	}
	return count
}

type fakeAttach struct {
	mu      sync.Mutex
	reader  *bytes.Reader
	written bytes.Buffer
}

func (attach *fakeAttach) Read(value []byte) (int, error) {
	return attach.reader.Read(value)
}

func (attach *fakeAttach) Write(value []byte) (int, error) {
	attach.mu.Lock()
	defer attach.mu.Unlock()
	return attach.written.Write(value)
}

func (*fakeAttach) CloseWrite() error { return nil }
func (*fakeAttach) Close() error      { return nil }

type fakeCgroupResolver struct {
	mu        sync.Mutex
	snapshots []ResourceSnapshot
	err       error
}

func (resolver *fakeCgroupResolver) Resolve(
	context.Context,
	string,
	int,
) (CgroupAccount, error) {
	if resolver.err != nil {
		return nil, resolver.err
	}
	resolver.mu.Lock()
	snapshots := append([]ResourceSnapshot(nil), resolver.snapshots...)
	resolver.mu.Unlock()
	return &fakeCgroupAccount{snapshots: snapshots}, nil
}

type fakeCgroupAccount struct {
	mu        sync.Mutex
	snapshots []ResourceSnapshot
	index     int
}

func (*fakeCgroupAccount) ID() string { return "cg-test" }

func (account *fakeCgroupAccount) Snapshot(context.Context) (ResourceSnapshot, error) {
	account.mu.Lock()
	defer account.mu.Unlock()
	if len(account.snapshots) == 0 {
		return ResourceSnapshot{}, nil
	}
	index := min(account.index, len(account.snapshots)-1)
	value := account.snapshots[index]
	account.index++
	return value, nil
}

func (*fakeCgroupAccount) Close() error { return nil }

func multiplexedOutput(stream byte, payload []byte) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
	return append(header, payload...)
}

func equalStrings(first []string, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func indexOfString(values []string, wanted string) int {
	for index, value := range values {
		if value == wanted {
			return index
		}
	}
	return -1
}

func waitForFakeEngineEvent(t *testing.T, engine *fakeEngine, wanted string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if containsString(engine.eventNames(), wanted) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("engine events = %v, want %q", engine.eventNames(), wanted)
}
