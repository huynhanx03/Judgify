package capability

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// ProfileCatalog is the read-only Problem-owned registry boundary used to
// verify that every capability component is active and exact.
type ProfileCatalog interface {
	RuntimeProfile(key string, version string) (entity.RuntimeProfile, bool)
	CheckerProfile(key string, version string) (entity.CheckerProfile, bool)
}

const maximumActivationEvidence = 64

// EvidenceStore is the durable boundary. Implementations must persist the
// complete passing evidence set and enable claims in one transaction,
// conditional on the same still-disabled worker boot. A crash or rejected row
// leaves claims disabled and persists none of the set.
type EvidenceStore interface {
	RegisterBoot(ctx context.Context, worker WorkerInstance) error
	DisableClaims(ctx context.Context, workerID uuid.UUID, bootID uuid.UUID) error
	RecordPassingEvidenceSetAndEnableClaims(
		ctx context.Context,
		worker WorkerInstance,
		evidence []CapabilityEvidence,
	) error
}

type Registry struct {
	catalog   ProfileCatalog
	store     EvidenceStore
	freshness time.Duration
	approved  map[string]CapabilityTuple

	mu          sync.RWMutex
	workers     map[uuid.UUID]WorkerInstance
	activations map[activationKey]CapabilityEvidence
}

type activationKey struct {
	WorkerID    uuid.UUID
	BootID      uuid.UUID
	ProfileHash string
}

func NewRegistry(
	catalog ProfileCatalog,
	store EvidenceStore,
	freshness time.Duration,
	approvedTuples []CapabilityTuple,
) (*Registry, error) {
	if catalog == nil || store == nil || freshness <= 0 ||
		len(approvedTuples) == 0 ||
		len(approvedTuples) > maximumActivationEvidence {
		return nil, ErrInvalidCapability
	}
	registry := &Registry{
		catalog: catalog, store: store, freshness: freshness,
		approved:    make(map[string]CapabilityTuple, len(approvedTuples)),
		workers:     make(map[uuid.UUID]WorkerInstance),
		activations: make(map[activationKey]CapabilityEvidence),
	}
	for _, tuple := range approvedTuples {
		if validateCapabilityTuple(tuple) != nil {
			return nil, ErrInvalidCapability
		}
		if _, duplicate := registry.approved[tuple.CapabilityProfileHash]; duplicate {
			return nil, ErrInvalidCapability
		}
		evidence := CapabilityEvidence{
			Tuple:                         tuple,
			CheckerImplementationChecksum: tuple.CheckerImplementationChecksum,
		}
		if registry.validateCatalogComponents(evidence) != nil {
			return nil, ErrInvalidCapability
		}
		registry.approved[tuple.CapabilityProfileHash] = tuple
	}
	return registry, nil
}

// RegisterBoot persists an instance with claims forced off, even if a caller
// supplied a stale claims-enabled value.
func (registry *Registry) RegisterBoot(ctx context.Context, worker WorkerInstance) error {
	if registry == nil || ctx == nil || ctx.Err() != nil || !validWorker(worker) {
		return ErrCapabilityDenied
	}
	worker.ClaimsEnabled = false
	worker.StartedAt = worker.StartedAt.UTC()
	worker.HeartbeatAt = worker.HeartbeatAt.UTC()
	if err := registry.store.RegisterBoot(ctx, worker); err != nil {
		return errors.Join(ErrCapabilityDenied, err)
	}
	registry.mu.Lock()
	registry.workers[worker.ID] = worker
	registry.mu.Unlock()
	return nil
}

// DisableClaims fences this exact boot before a worker drain. It is idempotent
// only at the caller's lifecycle level; a missing or replaced boot fails
// closed so an old process cannot silently continue claiming work.
func (registry *Registry) DisableClaims(
	ctx context.Context,
	workerID uuid.UUID,
	bootID uuid.UUID,
) error {
	if registry == nil || ctx == nil || ctx.Err() != nil || workerID == uuid.Nil || bootID == uuid.Nil {
		return ErrCapabilityDenied
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	worker, exists := registry.workers[workerID]
	if !exists || worker.BootID != bootID {
		return ErrCapabilityDenied
	}
	worker.ClaimsEnabled = false
	registry.workers[workerID] = worker
	if err := registry.store.DisableClaims(ctx, workerID, bootID); err != nil {
		return errors.Join(ErrCapabilityDenied, err)
	}
	return nil
}

// Activate retains the single-advertisement boundary for one-tuple catalogs.
// A partial call against a multi-tuple release fails closed through ActivateSet.
func (registry *Registry) Activate(
	ctx context.Context,
	worker WorkerInstance,
	advertisement CapabilityAdvertisement,
	now time.Time,
) error {
	return registry.ActivateSet(
		ctx,
		worker,
		[]CapabilityAdvertisement{advertisement},
		now,
	)
}

// ActivateSet fails closed and admits only the complete approved tuple set.
// The registry keeps its lock across durable fencing and commit so CanClaim
// cannot observe an in-memory partial transition.
func (registry *Registry) ActivateSet(
	ctx context.Context,
	worker WorkerInstance,
	advertisements []CapabilityAdvertisement,
	now time.Time,
) error {
	if registry == nil || ctx == nil || ctx.Err() != nil || now.IsZero() {
		return ErrCapabilityDenied
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registered, exists := registry.workers[worker.ID]
	if !exists || registered.BootID != worker.BootID {
		return ErrCapabilityDenied
	}
	registered.ClaimsEnabled = false
	registry.workers[worker.ID] = registered
	if err := registry.store.DisableClaims(ctx, worker.ID, worker.BootID); err != nil {
		return errors.Join(ErrCapabilityDenied, err)
	}
	if registered.WorkerKey != worker.WorkerKey ||
		!registered.StartedAt.Equal(worker.StartedAt) ||
		!registered.HeartbeatAt.Equal(worker.HeartbeatAt) ||
		registered.TotalCapacity != worker.TotalCapacity ||
		registered.FreeCapacity != worker.FreeCapacity {
		return ErrCapabilityDenied
	}
	worker = registered
	worker.ClaimsEnabled = false
	now = now.UTC()
	if worker.StartedAt.IsZero() || worker.StartedAt.After(worker.HeartbeatAt) ||
		worker.HeartbeatAt.After(now) ||
		len(advertisements) != len(registry.approved) {
		return ErrCapabilityDenied
	}
	evidenceSet := make([]CapabilityEvidence, 0, len(advertisements))
	seen := make(map[string]struct{}, len(advertisements))
	for _, advertisement := range advertisements {
		if advertisesListsOrRanges(advertisement) {
			return ErrCapabilityDenied
		}
		evidence := advertisement.Evidence
		expected, approved := registry.approved[evidence.Tuple.CapabilityProfileHash]
		_, duplicate := seen[evidence.Tuple.CapabilityProfileHash]
		if !approved || duplicate || expected != evidence.Tuple ||
			ValidatePassingEvidence(worker, evidence) != nil ||
			evidence.CheckerImplementationChecksum != evidence.Tuple.CheckerImplementationChecksum ||
			evidence.ProbedAt.After(now) ||
			registry.validateCatalogComponents(evidence) != nil {
			return ErrCapabilityDenied
		}
		seen[evidence.Tuple.CapabilityProfileHash] = struct{}{}
		evidence.ProbedAt = evidence.ProbedAt.UTC()
		evidenceSet = append(evidenceSet, evidence)
	}
	if len(seen) != len(registry.approved) {
		return ErrCapabilityDenied
	}
	if err := registry.store.RecordPassingEvidenceSetAndEnableClaims(
		ctx,
		worker,
		evidenceSet,
	); err != nil {
		return errors.Join(ErrCapabilityDenied, err)
	}
	worker.ClaimsEnabled = true
	worker.HeartbeatAt = worker.HeartbeatAt.UTC()
	registry.workers[worker.ID] = worker
	for _, evidence := range evidenceSet {
		registry.activations[activationKey{
			WorkerID: worker.ID, BootID: worker.BootID,
			ProfileHash: evidence.Tuple.CapabilityProfileHash,
		}] = evidence
	}
	return nil
}

// CanClaim matches one full persisted tuple row. It never joins independent
// runtime/checker/version lists, so synthetic Cartesian combinations miss.
func (registry *Registry) CanClaim(
	workerID uuid.UUID,
	bootID uuid.UUID,
	tuple CapabilityTuple,
	now time.Time,
) bool {
	if registry == nil || now.IsZero() || validateCapabilityTuple(tuple) != nil {
		return false
	}
	registry.mu.RLock()
	worker, exists := registry.workers[workerID]
	evidence, activated := registry.activations[activationKey{
		WorkerID: workerID, BootID: bootID, ProfileHash: tuple.CapabilityProfileHash,
	}]
	registry.mu.RUnlock()
	return exists && worker.BootID == bootID && worker.ClaimsEnabled &&
		freshAt(now, worker.HeartbeatAt, registry.freshness) && activated &&
		evidence.Passing && evidence.BootID == bootID && evidence.Tuple == tuple &&
		evidence.CheckerImplementationChecksum == tuple.CheckerImplementationChecksum &&
		!evidence.ProbedAt.Before(worker.StartedAt) &&
		!evidence.ProbedAt.After(now)
}

func (registry *Registry) validateCatalogComponents(evidence CapabilityEvidence) error {
	tuple := evidence.Tuple
	if err := validateCapabilityTuple(tuple); err != nil {
		return err
	}
	runtimeProfile, exists := registry.catalog.RuntimeProfile(tuple.RuntimeKey, tuple.RuntimeProfileVersion)
	if !exists || !runtimeProfile.Active || runtimeProfile.ImageDigest != tuple.ImageDigest ||
		runtimeProfile.CompilerVersion != tuple.CompilerVersion ||
		runtimeProfile.RuntimeBootstrapVersion != tuple.RuntimeBootstrapVersion ||
		runtimeProfile.CompileCommandID != tuple.CompileCommandID ||
		runtimeProfile.RunCommandID != tuple.RunCommandID ||
		runtimeProfile.ManifestVersion != tuple.ManifestVersion ||
		runtimeProfile.ManifestChecksum != tuple.ManifestChecksum {
		return ErrCapabilityDenied
	}
	checkerProfile, exists := registry.catalog.CheckerProfile(tuple.CheckerKey, tuple.CheckerVersion)
	if !exists || !checkerProfile.Active ||
		checkerProfile.ImplementationChecksum != evidence.CheckerImplementationChecksum {
		return ErrCapabilityDenied
	}
	return nil
}

func validWorker(worker WorkerInstance) bool {
	return worker.ID != uuid.Nil && worker.BootID != uuid.Nil &&
		capabilityIdentifierPattern.MatchString(worker.WorkerKey) &&
		!worker.StartedAt.IsZero() && !worker.HeartbeatAt.IsZero() &&
		!worker.HeartbeatAt.Before(worker.StartedAt) && worker.TotalCapacity > 0 &&
		worker.FreeCapacity >= 0 && worker.FreeCapacity <= worker.TotalCapacity
}

func freshAt(now time.Time, observed time.Time, maximumAge time.Duration) bool {
	if observed.IsZero() || observed.After(now) {
		return false
	}
	return now.Sub(observed) <= maximumAge
}

func advertisesListsOrRanges(advertisement CapabilityAdvertisement) bool {
	return len(advertisement.RuntimeKeys) != 0 ||
		len(advertisement.RuntimeProfileVersions) != 0 ||
		len(advertisement.CheckerKeys) != 0 ||
		len(advertisement.CheckerVersions) != 0 ||
		advertisement.RuntimeVersionRange != "" ||
		advertisement.CheckerVersionRange != ""
}
