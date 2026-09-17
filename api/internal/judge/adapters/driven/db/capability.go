package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	generatedworker "github.com/huynhanx03/judgify/internal/ent/generate/judgeworkerinstance"
	"github.com/huynhanx03/judgify/internal/judge/capability"
)

var ErrInvalidCapabilityEvidenceStore = errors.New("capability evidence store: invalid operation")

const maximumCapabilityEvidenceBatch = 64

// CapabilityEvidenceStore persists boot-scoped evidence. Its activation write
// changes only the worker's own claim eligibility; platform API activation is
// a separate audited command and is deliberately not part of this adapter.
type CapabilityEvidenceStore struct {
	client *internalent.EntClient
}

func NewCapabilityEvidenceStore(client *internalent.EntClient) (*CapabilityEvidenceStore, error) {
	if client == nil || client.Client() == nil {
		return nil, ErrInvalidCapabilityEvidenceStore
	}
	return &CapabilityEvidenceStore{client: client}, nil
}

func (store *CapabilityEvidenceStore) RegisterBoot(
	ctx context.Context,
	worker capability.WorkerInstance,
) error {
	if store == nil || store.client == nil || ctx == nil ||
		capability.ValidatePassingWorker(worker) != nil {
		return ErrInvalidCapabilityEvidenceStore
	}
	_, err := store.client.DB(ctx).JudgeWorkerInstance.Create().
		SetID(worker.ID).
		SetWorkerKey(worker.WorkerKey).
		SetBootID(worker.BootID).
		SetBootStartedAt(worker.StartedAt.UTC()).
		SetHeartbeatAt(worker.HeartbeatAt.UTC()).
		SetClaimsEnabled(false).
		SetTotalCapacity(worker.TotalCapacity).
		SetFreeCapacity(worker.FreeCapacity).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("%w: register worker boot: %v", ErrInvalidCapabilityEvidenceStore, err)
	}
	return nil
}

func (store *CapabilityEvidenceStore) DisableClaims(
	ctx context.Context,
	workerID uuid.UUID,
	bootID uuid.UUID,
) error {
	if store == nil || store.client == nil || ctx == nil || workerID == uuid.Nil || bootID == uuid.Nil {
		return ErrInvalidCapabilityEvidenceStore
	}
	updated, err := store.client.DB(ctx).JudgeWorkerInstance.Update().
		Where(generatedworker.IDEQ(workerID), generatedworker.BootIDEQ(bootID)).
		SetClaimsEnabled(false).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("%w: disable worker claims: %v", ErrInvalidCapabilityEvidenceStore, err)
	}
	if updated != 1 {
		return fmt.Errorf("%w: worker boot not found", ErrInvalidCapabilityEvidenceStore)
	}
	return nil
}

func (store *CapabilityEvidenceStore) RecordPassingEvidenceSetAndEnableClaims(
	ctx context.Context,
	worker capability.WorkerInstance,
	evidenceSet []capability.CapabilityEvidence,
) error {
	if store == nil || store.client == nil || ctx == nil ||
		worker.ClaimsEnabled ||
		len(evidenceSet) == 0 ||
		len(evidenceSet) > maximumCapabilityEvidenceBatch {
		return ErrInvalidCapabilityEvidenceStore
	}
	validated := append([]capability.CapabilityEvidence(nil), evidenceSet...)
	seen := make(map[string]struct{}, len(validated))
	for index := range validated {
		evidence := validated[index]
		hash := evidence.Tuple.CapabilityProfileHash
		if capability.ValidatePassingEvidence(worker, evidence) != nil {
			return ErrInvalidCapabilityEvidenceStore
		}
		if _, duplicate := seen[hash]; duplicate {
			return ErrInvalidCapabilityEvidenceStore
		}
		seen[hash] = struct{}{}
		validated[index].ProbedAt = evidence.ProbedAt.UTC()
	}
	return store.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := store.client.DB(txCtx)
		// This is the transaction's first write. On PostgreSQL it locks only the
		// exact, still-disabled boot; a stale boot or racing retry loses closed.
		updated, err := client.JudgeWorkerInstance.Update().
			Where(
				generatedworker.IDEQ(worker.ID),
				generatedworker.BootIDEQ(worker.BootID),
				generatedworker.ClaimsEnabledEQ(false),
			).
			SetClaimsEnabled(false).
			Save(txCtx)
		if err != nil {
			return fmt.Errorf("lock disabled worker boot: %w", err)
		}
		if updated != 1 {
			return ErrInvalidCapabilityEvidenceStore
		}

		for _, evidence := range validated {
			tuple := evidence.Tuple
			if _, err := client.JudgeWorkerCapability.Create().
				SetWorkerInstanceID(worker.ID).
				SetBootID(worker.BootID).
				SetJudgeSpecProtocol(tuple.JudgeSpecProtocol).
				SetRuntimeKey(tuple.RuntimeKey).
				SetRuntimeProfileVersion(tuple.RuntimeProfileVersion).
				SetCompilerVersion(tuple.CompilerVersion).
				SetRuntimeBootstrapVersion(tuple.RuntimeBootstrapVersion).
				SetCompileCommandID(tuple.CompileCommandID).
				SetRunCommandID(tuple.RunCommandID).
				SetImageDigest(tuple.ImageDigest).
				SetManifestVersion(tuple.ManifestVersion).
				SetManifestChecksum(tuple.ManifestChecksum).
				SetSupervisorProtocol(tuple.SupervisorProtocol).
				SetCheckerKey(tuple.CheckerKey).
				SetCheckerVersion(tuple.CheckerVersion).
				SetCapabilityProfileHash(tuple.CapabilityProfileHash).
				SetCheckerImplementationChecksum(evidence.CheckerImplementationChecksum).
				SetSmokeChecksum(evidence.SmokeChecksum).
				SetGoldenChecksum(evidence.GoldenChecksum).
				SetProbedAt(evidence.ProbedAt).
				SetPassing(true).
				Save(txCtx); err != nil {
				return fmt.Errorf("persist passing worker evidence set: %w", err)
			}
		}

		updated, err = client.JudgeWorkerInstance.Update().
			Where(
				generatedworker.IDEQ(worker.ID),
				generatedworker.BootIDEQ(worker.BootID),
				generatedworker.ClaimsEnabledEQ(false),
			).
			SetClaimsEnabled(true).
			Save(txCtx)
		if err != nil {
			return fmt.Errorf("enable exact worker boot claims: %w", err)
		}
		if updated != 1 {
			return ErrInvalidCapabilityEvidenceStore
		}
		return nil
	})
}

var _ capability.EvidenceStore = (*CapabilityEvidenceStore)(nil)
