package worker

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"

	"github.com/huynhanx03/judgify/internal/judge/capability"
	submissionservice "github.com/huynhanx03/judgify/internal/submission/usecase"
)

const maximumReadinessCapabilities = 64

// CapacitySnapshotter is the read-only host-capacity boundary used by worker
// readiness. judge.HostCapacityProbe satisfies this interface.
type CapacitySnapshotter interface {
	Snapshot(context.Context) (submissionservice.CapacitySnapshot, error)
}

// RuntimeReadinessSnapshot is safe process-local diagnostic state. It omits
// worker/boot identities, tuple hashes, filesystem paths, and dependency
// errors so it cannot accidentally become a high-cardinality metric payload.
type RuntimeReadinessSnapshot struct {
	Bound                   bool
	Initialized             bool
	Draining                bool
	ClaimsEnabled           bool
	CapacityProbeBound      bool
	ExpectedCapabilityCount int
	PassingCapabilityCount  int
	SandboxProbePassing     bool
	CgroupV2ProbePassing    bool
}

// CapacityReadinessSnapshot is safe for trusted gauges. Readiness uses only
// DiskHealthy: zero free execution slots is normal while the worker is busy
// and must not cause an orchestrator restart loop.
type CapacityReadinessSnapshot struct {
	FreeSandboxes      int
	DiskAvailableBytes int64
	DiskHealthy        bool
}

// RuntimeReadinessState is the worker-owned, fail-closed handoff between
// lifecycle initialization and health checkers. It owns no goroutine.
//
// Capability observations are immutable proof for this exact boot. They are
// recorded once during initialization; rolling freshness belongs only to the
// worker and active lease heartbeats checked from durable storage.
type RuntimeReadinessState struct {
	mu sync.RWMutex

	heartbeatFreshness   time.Duration
	minimumFreeDiskBytes uint64

	bound         bool
	initialized   bool
	draining      bool
	claimsEnabled bool
	worker        capability.WorkerInstance
	expected      map[string]capability.CapabilityTuple
	probes        map[string]time.Time
	capacity      CapacitySnapshotter
}

func NewRuntimeReadinessState(
	heartbeatFreshness time.Duration,
	minimumFreeDiskBytes uint64,
) (*RuntimeReadinessState, error) {
	if heartbeatFreshness <= 0 || minimumFreeDiskBytes == 0 ||
		minimumFreeDiskBytes > uint64(^uint64(0)>>1) {
		return nil, ErrWorkerReadiness
	}
	return &RuntimeReadinessState{
		heartbeatFreshness:   heartbeatFreshness,
		minimumFreeDiskBytes: minimumFreeDiskBytes,
		expected:             make(map[string]capability.CapabilityTuple),
		probes:               make(map[string]time.Time),
	}, nil
}

// BindCurrentBoot installs the only worker identity and exact release tuple set
// accepted by this state. Rebinding is rejected so a previous boot can never
// inherit readiness observations from a replacement process.
func (state *RuntimeReadinessState) BindCurrentBoot(
	worker capability.WorkerInstance,
	tuples []capability.CapabilityTuple,
) error {
	if state == nil || capability.ValidatePassingWorker(worker) != nil ||
		len(tuples) == 0 || len(tuples) > maximumReadinessCapabilities {
		return ErrWorkerReadiness
	}
	expected := make(map[string]capability.CapabilityTuple, len(tuples))
	for _, tuple := range tuples {
		hash, err := capability.ComputeCapabilityProfileHash(tuple)
		if err != nil || hash != tuple.CapabilityProfileHash {
			return ErrWorkerReadiness
		}
		if _, duplicate := expected[hash]; duplicate {
			return ErrWorkerReadiness
		}
		expected[hash] = tuple
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.bound || state.draining {
		return ErrWorkerReadiness
	}
	worker.ClaimsEnabled = false
	state.bound = true
	state.worker = worker
	state.expected = expected
	state.probes = make(map[string]time.Time, len(expected))
	return nil
}

// RecordPassingProbe records one complete production probe. Structural and
// boot/tuple binding are revalidated before the batch is durably activated.
func (state *RuntimeReadinessState) RecordPassingProbe(
	evidence capability.CapabilityEvidence,
) error {
	if state == nil {
		return ErrWorkerReadiness
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	expected, exists := state.expected[evidence.Tuple.CapabilityProfileHash]
	if !state.bound || state.draining || !exists || expected != evidence.Tuple ||
		capability.ValidatePassingEvidence(state.worker, evidence) != nil {
		return ErrWorkerReadiness
	}
	observed := evidence.ProbedAt.UTC()
	if _, duplicate := state.probes[evidence.Tuple.CapabilityProfileHash]; duplicate {
		return ErrWorkerReadiness
	}
	state.probes[evidence.Tuple.CapabilityProfileHash] = observed
	return nil
}

// BindCapacityProbe installs the live, read-only disk/scheduler snapshotter.
// The probe is called by readiness and remains owned by the worker runtime.
func (state *RuntimeReadinessState) BindCapacityProbe(
	probe CapacitySnapshotter,
) error {
	if state == nil || nilInterface(probe) {
		return ErrWorkerReadiness
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.bound || state.draining || !nilInterface(state.capacity) {
		return ErrWorkerReadiness
	}
	state.capacity = probe
	return nil
}

// MarkInitialized opens only the in-memory initialization invariant. Durable
// boot/claims/evidence and live capacity remain independently checked.
func (state *RuntimeReadinessState) MarkInitialized() error {
	if state == nil {
		return ErrWorkerReadiness
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.bound || state.draining || nilInterface(state.capacity) ||
		!state.probesCompleteLocked() {
		return ErrWorkerReadiness
	}
	state.initialized = true
	return nil
}

// MarkClaimsEnabled mirrors the successfully committed durable activation.
// It is separate from initialization so metrics and readiness never claim the
// worker can lease before Registry.ActivateSet has returned.
func (state *RuntimeReadinessState) MarkClaimsEnabled() error {
	if state == nil {
		return ErrWorkerReadiness
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.initialized || state.draining || state.claimsEnabled {
		return ErrWorkerReadiness
	}
	state.claimsEnabled = true
	return nil
}

// BeginDrain closes initialization readiness synchronously. The production
// runtime calls it before attempting the durable claims-off write.
func (state *RuntimeReadinessState) BeginDrain() {
	if state == nil {
		return
	}
	state.mu.Lock()
	state.draining = true
	state.initialized = false
	state.claimsEnabled = false
	state.mu.Unlock()
}

func (state *RuntimeReadinessState) Snapshot() RuntimeReadinessSnapshot {
	if state == nil {
		return RuntimeReadinessSnapshot{}
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	snapshot := RuntimeReadinessSnapshot{
		Bound:                   state.bound,
		Initialized:             state.initialized,
		Draining:                state.draining,
		ClaimsEnabled:           state.claimsEnabled,
		CapacityProbeBound:      !nilInterface(state.capacity),
		ExpectedCapabilityCount: len(state.expected),
		PassingCapabilityCount:  len(state.probes),
	}
	passing := state.probesCompleteLocked()
	snapshot.SandboxProbePassing = passing
	snapshot.CgroupV2ProbePassing = passing
	return snapshot
}

func (state *RuntimeReadinessState) CapacitySnapshot(
	ctx context.Context,
) (CapacityReadinessSnapshot, error) {
	if state == nil || ctx == nil {
		return CapacityReadinessSnapshot{}, ErrWorkerReadiness
	}
	if err := ctx.Err(); err != nil {
		return CapacityReadinessSnapshot{}, err
	}
	state.mu.RLock()
	probe := state.capacity
	minimum := state.minimumFreeDiskBytes
	draining := state.draining
	totalCapacity := state.worker.TotalCapacity
	state.mu.RUnlock()
	if nilInterface(probe) || draining || totalCapacity <= 0 {
		return CapacityReadinessSnapshot{}, ErrWorkerReadiness
	}
	value, err := probe.Snapshot(ctx)
	if err != nil || value.FreeSandboxes < 0 ||
		value.FreeSandboxes > totalCapacity || value.DiskAvailableBytes < 0 {
		if ctx.Err() != nil {
			return CapacityReadinessSnapshot{}, ctx.Err()
		}
		return CapacityReadinessSnapshot{}, ErrWorkerReadiness
	}
	return CapacityReadinessSnapshot{
		FreeSandboxes:      value.FreeSandboxes,
		DiskAvailableBytes: value.DiskAvailableBytes,
		DiskHealthy:        uint64(value.DiskAvailableBytes) >= minimum,
	}, nil
}

func (state *RuntimeReadinessState) probesCompleteLocked() bool {
	if !state.bound || len(state.expected) == 0 ||
		len(state.probes) != len(state.expected) {
		return false
	}
	for hash := range state.expected {
		observed := state.probes[hash]
		if observed.IsZero() {
			return false
		}
	}
	return true
}

type runtimeReadinessFacet uint8

const (
	runtimeInitializedFacet runtimeReadinessFacet = iota + 1
	runtimeSandboxFacet
	runtimeCgroupV2Facet
	runtimeDiskFacet
)

type runtimeReadinessChecker struct {
	state *RuntimeReadinessState
	facet runtimeReadinessFacet
}

func (checker runtimeReadinessChecker) Check(ctx context.Context) error {
	if checker.state == nil || ctx == nil {
		return ErrWorkerReadiness
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if checker.facet == runtimeDiskFacet {
		snapshot, err := checker.state.CapacitySnapshot(ctx)
		if err != nil || !snapshot.DiskHealthy {
			return ErrWorkerReadiness
		}
		return nil
	}
	snapshot := checker.state.Snapshot()
	switch checker.facet {
	case runtimeInitializedFacet:
		if snapshot.Bound && snapshot.Initialized && !snapshot.Draining &&
			snapshot.CapacityProbeBound {
			return nil
		}
	case runtimeSandboxFacet:
		if snapshot.Initialized && !snapshot.Draining &&
			snapshot.SandboxProbePassing {
			return nil
		}
	case runtimeCgroupV2Facet:
		if snapshot.Initialized && !snapshot.Draining &&
			snapshot.CgroupV2ProbePassing {
			return nil
		}
	}
	return ErrWorkerReadiness
}

// RuntimeReadinessCheckers are stable adapters for WorkerHealthDependencies.
type RuntimeReadinessCheckers struct {
	Initialized commonhealth.Checker
	Sandbox     commonhealth.Checker
	CgroupV2    commonhealth.Checker
	Disk        commonhealth.Checker
}

func NewRuntimeReadinessCheckers(
	state *RuntimeReadinessState,
) (RuntimeReadinessCheckers, error) {
	if state == nil {
		return RuntimeReadinessCheckers{}, ErrWorkerReadiness
	}
	return RuntimeReadinessCheckers{
		Initialized: runtimeReadinessChecker{state: state, facet: runtimeInitializedFacet},
		Sandbox:     runtimeReadinessChecker{state: state, facet: runtimeSandboxFacet},
		CgroupV2:    runtimeReadinessChecker{state: state, facet: runtimeCgroupV2Facet},
		Disk:        runtimeReadinessChecker{state: state, facet: runtimeDiskFacet},
	}, nil
}

type readinessBinding struct {
	worker             capability.WorkerInstance
	expected           map[string]capability.CapabilityTuple
	heartbeatFreshness time.Duration
}

func (state *RuntimeReadinessState) persistenceBinding() (readinessBinding, bool) {
	if state == nil {
		return readinessBinding{}, false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	if !state.bound || state.draining || state.worker.ID == uuid.Nil ||
		state.worker.BootID == uuid.Nil || len(state.expected) == 0 {
		return readinessBinding{}, false
	}
	expected := make(map[string]capability.CapabilityTuple, len(state.expected))
	for hash, tuple := range state.expected {
		expected[hash] = tuple
	}
	return readinessBinding{
		worker:             state.worker,
		expected:           expected,
		heartbeatFreshness: state.heartbeatFreshness,
	}, true
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

var _ commonhealth.Checker = runtimeReadinessChecker{}
