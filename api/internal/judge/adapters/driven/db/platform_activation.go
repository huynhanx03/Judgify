package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedactivation "github.com/huynhanx03/judgify/internal/ent/generate/activatedjudgecapability"
	generatedevidence "github.com/huynhanx03/judgify/internal/ent/generate/judgeworkercapability"
	generatedworker "github.com/huynhanx03/judgify/internal/ent/generate/judgeworkerinstance"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/judge/capability"
)

var ErrInvalidPlatformActivationStore = errors.New("platform activation store: denied")

type PlatformActivationStore struct {
	client *internalent.EntClient
}

func NewPlatformActivationStore(client *internalent.EntClient) (*PlatformActivationStore, error) {
	if client == nil || client.Client() == nil {
		return nil, ErrInvalidPlatformActivationStore
	}
	return &PlatformActivationStore{client: client}, nil
}

func (store *PlatformActivationStore) CreateIfFreshWorkerProves(
	ctx context.Context,
	activation capability.ActivatedCapability,
	notBefore time.Time,
) (capability.ActivatedCapability, error) {
	if store == nil || store.client == nil || ctx == nil || ctx.Err() != nil ||
		notBefore.IsZero() || !notBefore.Before(activation.ActivatedAt) ||
		!validUnprovedActivation(activation) {
		return capability.ActivatedCapability{}, ErrInvalidPlatformActivationStore
	}
	var created capability.ActivatedCapability
	err := store.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := store.client.DB(txCtx)
		evidencePredicates := exactEvidencePredicates(activation)
		workerQuery := client.JudgeWorkerInstance.Query().
			Where(
				generatedworker.ClaimsEnabledEQ(true),
				generatedworker.HeartbeatAtGTE(notBefore.UTC()),
				generatedworker.HeartbeatAtLTE(activation.ActivatedAt.UTC()),
				currentActivationWorkerBoot(),
				generatedworker.HasCapabilitiesWith(evidencePredicates...),
			).
			Order(
				generatedworker.ByHeartbeatAt(entsql.OrderDesc()),
				generatedworker.ByID(),
			)
		if store.isPostgres() {
			// DisableClaims updates this same row and therefore cannot invalidate
			// the liveness proof before the activation commits.
			workerQuery.ForUpdate()
		}
		worker, err := workerQuery.First(txCtx)
		if err != nil {
			return fmt.Errorf("select fresh claims-enabled worker proof: %w", err)
		}

		evidenceQuery := client.JudgeWorkerCapability.Query().
			Where(append(evidencePredicates,
				generatedevidence.WorkerInstanceIDEQ(worker.ID),
				generatedevidence.BootIDEQ(worker.BootID),
				generatedevidence.ProbedAtGTE(worker.BootStartedAt),
			)...).
			Order(
				generatedevidence.ByProbedAt(entsql.OrderDesc()),
				generatedevidence.ByID(),
			)
		if store.isPostgres() {
			evidenceQuery.ForShare()
		}
		evidence, err := evidenceQuery.First(txCtx)
		if err != nil {
			return fmt.Errorf("select exact worker evidence proof: %w", err)
		}
		proofWorker := mapProofWorker(worker)
		proofEvidence := mapCapabilityEvidence(evidence)
		if evidence.ID == uuid.Nil ||
			evidence.WorkerInstanceID != worker.ID ||
			evidence.BootID != worker.BootID ||
			capability.ValidatePassingEvidence(proofWorker, proofEvidence) != nil ||
			!proofWorker.ClaimsEnabled ||
			!proofWorker.HeartbeatAt.After(proofWorker.StartedAt) ||
			proofWorker.HeartbeatAt.Before(notBefore) ||
			proofWorker.HeartbeatAt.After(activation.ActivatedAt) ||
			proofEvidence.ProbedAt.After(activation.ActivatedAt) ||
			proofEvidence.Tuple != activation.Tuple ||
			proofEvidence.CheckerImplementationChecksum !=
				activation.Tuple.CheckerImplementationChecksum {
			return ErrInvalidPlatformActivationStore
		}
		activation.CheckerImplementationChecksum =
			proofEvidence.CheckerImplementationChecksum
		activation.SmokeChecksum = proofEvidence.SmokeChecksum
		activation.GoldenChecksum = proofEvidence.GoldenChecksum

		tuple := activation.Tuple
		record, err := client.ActivatedJudgeCapability.Create().
			SetID(activation.ID).
			SetReleaseID(activation.ReleaseID).
			SetApprovedReleaseManifestChecksum(activation.ApprovedReleaseManifestChecksum).
			SetActorID(activation.ActorID).
			SetAuditID(activation.AuditID).
			SetReason(activation.Reason).
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
			SetCheckerImplementationChecksum(proofEvidence.CheckerImplementationChecksum).
			SetSmokeChecksum(proofEvidence.SmokeChecksum).
			SetGoldenChecksum(proofEvidence.GoldenChecksum).
			SetProofWorkerInstanceID(worker.ID).
			SetProofBootID(worker.BootID).
			SetProofEvidenceID(evidence.ID).
			SetState(generatedactivation.StateActive).
			SetActivatedAt(activation.ActivatedAt.UTC()).
			Save(txCtx)
		if err != nil {
			return fmt.Errorf("create durable platform activation: %w", err)
		}
		created = mapActivatedCapability(record)
		return nil
	})
	if err != nil {
		return capability.ActivatedCapability{}, errors.Join(ErrInvalidPlatformActivationStore, err)
	}
	return created, nil
}

func (store *PlatformActivationStore) Retire(
	ctx context.Context,
	request capability.RetirementRequest,
	retiredAt time.Time,
) error {
	if store == nil || store.client == nil || ctx == nil || ctx.Err() != nil ||
		retiredAt.IsZero() || capability.ValidateRetirementRequest(request) != nil {
		return ErrInvalidPlatformActivationStore
	}
	err := store.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := store.client.DB(txCtx)
		// Updating the activation first takes a row lock. Submission intake
		// holds a FOR SHARE lock on this same row while it resolves a
		// capability, so no intake can pass the active check while retirement
		// inspects the durable job graph.
		updated, updateErr := client.ActivatedJudgeCapability.
			UpdateOneID(request.ActivationID).
			Where(
				generatedactivation.StateEQ(generatedactivation.StateActive),
				generatedactivation.ActivatedAtLTE(retiredAt.UTC()),
			).
			SetState(generatedactivation.StateRetired).
			SetRetiredAt(retiredAt.UTC()).
			SetRetiredBy(request.ActorID).
			SetRetirementAuditID(request.AuditID).
			SetRetirementReason(request.Reason).
			Save(txCtx)
		if updateErr != nil {
			return updateErr
		}
		if updated.State != generatedactivation.StateRetired {
			return ErrInvalidPlatformActivationStore
		}

		// Lock one matching job before deciding. A concurrent worker claim
		// therefore either waits for this transaction or skips the row, and
		// intake cannot create a new matching job until the activation lock is
		// released. The exact tuple comparison is deliberate: a release may
		// contain several profiles, and retiring one must not block unrelated
		// work.
		jobInspectionQuery := `
SELECT job.id
FROM judge_jobs AS job
JOIN judge_specs AS spec
  ON spec.id = job.judge_spec_id
 AND spec.submission_id = job.submission_id
 AND spec.generation = job.generation
JOIN activated_judge_capabilities AS activation
  ON activation.id = $1
WHERE job.state IN ('ready', 'leased', 'retry_wait')
  AND spec.release_id = activation.release_id
  AND spec.approved_release_manifest_checksum =
      activation.approved_release_manifest_checksum
  AND spec.judge_spec_protocol = activation.judge_spec_protocol
  AND spec.runtime_key = activation.runtime_key
  AND spec.runtime_profile_version = activation.runtime_profile_version
  AND spec.compiler_version = activation.compiler_version
  AND spec.runtime_bootstrap_version = activation.runtime_bootstrap_version
  AND spec.compile_command_id = activation.compile_command_id
  AND spec.run_command_id = activation.run_command_id
  AND spec.sandbox_image_digest = activation.image_digest
  AND spec.artifact_manifest_version = activation.manifest_version
  AND spec.artifact_manifest_checksum = activation.manifest_checksum
  AND spec.supervisor_protocol_version = activation.supervisor_protocol
  AND spec.checker_key = activation.checker_key
  AND spec.checker_version = activation.checker_version
  AND spec.checker_implementation_checksum =
      activation.checker_implementation_checksum
  AND spec.capability_profile_hash = activation.capability_profile_hash`
		if store.isPostgres() {
			jobInspectionQuery += "\nFOR UPDATE OF job"
		}
		jobInspectionQuery += "\nLIMIT 1"
		rows, queryErr := client.ActivatedJudgeCapability.QueryContext(
			txCtx,
			jobInspectionQuery,
			request.ActivationID,
		)
		if queryErr != nil {
			return fmt.Errorf("inspect activation jobs: %w", queryErr)
		}
		defer func() { _ = rows.Close() }()
		if rows.Next() {
			var jobID uuid.UUID
			if scanErr := rows.Scan(&jobID); scanErr != nil {
				return fmt.Errorf("scan activation job: %w", scanErr)
			}
			if jobID == uuid.Nil {
				return ErrInvalidPlatformActivationStore
			}
			return capability.ErrActivationHasNonTerminalJobs
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			return fmt.Errorf("read activation jobs: %w", rowsErr)
		}
		return nil
	})
	if err != nil {
		return errors.Join(ErrInvalidPlatformActivationStore, err)
	}
	return nil
}

func (store *PlatformActivationStore) Active(
	ctx context.Context,
	releaseID string,
	tuple capability.CapabilityTuple,
) (capability.ActivatedCapability, bool, error) {
	if store == nil || store.client == nil || ctx == nil || ctx.Err() != nil || releaseID == "" {
		return capability.ActivatedCapability{}, false, ErrInvalidPlatformActivationStore
	}
	expectedHash, err := capability.ComputeCapabilityProfileHash(tuple)
	if err != nil || expectedHash != tuple.CapabilityProfileHash {
		return capability.ActivatedCapability{}, false, ErrInvalidPlatformActivationStore
	}
	records, err := store.client.DB(ctx).ActivatedJudgeCapability.Query().
		Where(
			generatedactivation.ReleaseIDEQ(releaseID),
			generatedactivation.StateEQ(generatedactivation.StateActive),
			generatedactivation.JudgeSpecProtocolEQ(tuple.JudgeSpecProtocol),
			generatedactivation.RuntimeKeyEQ(tuple.RuntimeKey),
			generatedactivation.RuntimeProfileVersionEQ(tuple.RuntimeProfileVersion),
			generatedactivation.CompilerVersionEQ(tuple.CompilerVersion),
			generatedactivation.RuntimeBootstrapVersionEQ(tuple.RuntimeBootstrapVersion),
			generatedactivation.CompileCommandIDEQ(tuple.CompileCommandID),
			generatedactivation.RunCommandIDEQ(tuple.RunCommandID),
			generatedactivation.ImageDigestEQ(tuple.ImageDigest),
			generatedactivation.ManifestVersionEQ(tuple.ManifestVersion),
			generatedactivation.ManifestChecksumEQ(tuple.ManifestChecksum),
			generatedactivation.SupervisorProtocolEQ(tuple.SupervisorProtocol),
			generatedactivation.CheckerKeyEQ(tuple.CheckerKey),
			generatedactivation.CheckerVersionEQ(tuple.CheckerVersion),
			generatedactivation.CheckerImplementationChecksumEQ(
				tuple.CheckerImplementationChecksum,
			),
			generatedactivation.CapabilityProfileHashEQ(tuple.CapabilityProfileHash),
		).
		Order(
			generatedactivation.ByActivatedAt(entsql.OrderDesc()),
			generatedactivation.ByID(),
		).
		Limit(2).
		All(ctx)
	if err != nil {
		return capability.ActivatedCapability{}, false, errors.Join(ErrInvalidPlatformActivationStore, err)
	}
	if len(records) == 0 {
		return capability.ActivatedCapability{}, false, nil
	}
	if len(records) != 1 {
		return capability.ActivatedCapability{}, false, ErrInvalidPlatformActivationStore
	}
	return mapActivatedCapability(records[0]), true, nil
}

func exactEvidencePredicates(activation capability.ActivatedCapability) []predicate.JudgeWorkerCapability {
	tuple := activation.Tuple
	return []predicate.JudgeWorkerCapability{
		generatedevidence.PassingEQ(true),
		generatedevidence.ProbedAtLTE(activation.ActivatedAt.UTC()),
		generatedevidence.JudgeSpecProtocolEQ(tuple.JudgeSpecProtocol),
		generatedevidence.RuntimeKeyEQ(tuple.RuntimeKey),
		generatedevidence.RuntimeProfileVersionEQ(tuple.RuntimeProfileVersion),
		generatedevidence.CompilerVersionEQ(tuple.CompilerVersion),
		generatedevidence.RuntimeBootstrapVersionEQ(tuple.RuntimeBootstrapVersion),
		generatedevidence.CompileCommandIDEQ(tuple.CompileCommandID),
		generatedevidence.RunCommandIDEQ(tuple.RunCommandID),
		generatedevidence.ImageDigestEQ(tuple.ImageDigest),
		generatedevidence.ManifestVersionEQ(tuple.ManifestVersion),
		generatedevidence.ManifestChecksumEQ(tuple.ManifestChecksum),
		generatedevidence.SupervisorProtocolEQ(tuple.SupervisorProtocol),
		generatedevidence.CheckerKeyEQ(tuple.CheckerKey),
		generatedevidence.CheckerVersionEQ(tuple.CheckerVersion),
		generatedevidence.CheckerImplementationChecksumEQ(
			tuple.CheckerImplementationChecksum,
		),
		generatedevidence.CapabilityProfileHashEQ(tuple.CapabilityProfileHash),
	}
}

func currentActivationWorkerBoot() predicate.JudgeWorkerInstance {
	return func(worker *entsql.Selector) {
		newer := entsql.Table(generatedworker.Table).As("newer_worker")
		worker.Where(entsql.And(
			entsql.ColumnsGT(
				worker.C(generatedworker.FieldHeartbeatAt),
				worker.C(generatedworker.FieldBootStartedAt),
			),
			entsql.NotExists(
				entsql.Select(newer.C(generatedworker.FieldID)).
					From(newer).
					Where(entsql.And(
						entsql.ColumnsEQ(
							newer.C(generatedworker.FieldWorkerKey),
							worker.C(generatedworker.FieldWorkerKey),
						),
						entsql.Or(
							entsql.ColumnsGT(
								newer.C(generatedworker.FieldBootStartedAt),
								worker.C(generatedworker.FieldBootStartedAt),
							),
							entsql.And(
								entsql.ColumnsEQ(
									newer.C(generatedworker.FieldBootStartedAt),
									worker.C(generatedworker.FieldBootStartedAt),
								),
								entsql.ColumnsGT(
									newer.C(generatedworker.FieldID),
									worker.C(generatedworker.FieldID),
								),
							),
						),
					)),
			),
		))
	}
}

func validUnprovedActivation(activation capability.ActivatedCapability) bool {
	expectedHash, err := capability.ComputeCapabilityProfileHash(activation.Tuple)
	request := capability.ActivationRequest{
		ReleaseID:                       activation.ReleaseID,
		ApprovedReleaseManifestChecksum: activation.ApprovedReleaseManifestChecksum,
		ActorID:                         activation.ActorID, AuditID: activation.AuditID,
		Reason: activation.Reason, Tuple: activation.Tuple,
	}
	return err == nil && expectedHash == activation.Tuple.CapabilityProfileHash &&
		capability.ValidateActivationRequest(request) == nil &&
		activation.ID != uuid.Nil && activation.ActorID != uuid.Nil && activation.AuditID != uuid.Nil &&
		activation.CheckerImplementationChecksum == "" &&
		activation.SmokeChecksum == "" &&
		activation.GoldenChecksum == "" && activation.State == capability.ActivationStateActive &&
		!activation.ActivatedAt.IsZero() && activation.RetiredAt == nil &&
		activation.ProofWorkerInstanceID == uuid.Nil && activation.ProofBootID == uuid.Nil &&
		activation.ProofEvidenceID == uuid.Nil
}

func mapProofWorker(record *generate.JudgeWorkerInstance) capability.WorkerInstance {
	if record == nil {
		return capability.WorkerInstance{}
	}
	return capability.WorkerInstance{
		ID: record.ID, WorkerKey: record.WorkerKey, BootID: record.BootID,
		StartedAt: record.BootStartedAt.UTC(), HeartbeatAt: record.HeartbeatAt.UTC(),
		ClaimsEnabled: record.ClaimsEnabled, TotalCapacity: record.TotalCapacity,
		FreeCapacity: record.FreeCapacity,
	}
}

func mapCapabilityEvidence(record *generate.JudgeWorkerCapability) capability.CapabilityEvidence {
	if record == nil {
		return capability.CapabilityEvidence{}
	}
	return capability.CapabilityEvidence{
		Tuple: capability.CapabilityTuple{
			JudgeSpecProtocol: record.JudgeSpecProtocol, RuntimeKey: record.RuntimeKey,
			RuntimeProfileVersion:   record.RuntimeProfileVersion,
			CompilerVersion:         record.CompilerVersion,
			RuntimeBootstrapVersion: record.RuntimeBootstrapVersion,
			CompileCommandID:        record.CompileCommandID, RunCommandID: record.RunCommandID,
			ImageDigest: record.ImageDigest, ManifestVersion: record.ManifestVersion,
			ManifestChecksum:   record.ManifestChecksum,
			SupervisorProtocol: record.SupervisorProtocol, CheckerKey: record.CheckerKey,
			CheckerVersion:                record.CheckerVersion,
			CheckerImplementationChecksum: record.CheckerImplementationChecksum,
			CapabilityProfileHash:         record.CapabilityProfileHash,
		},
		BootID:                        record.BootID,
		CheckerImplementationChecksum: record.CheckerImplementationChecksum,
		SmokeChecksum:                 record.SmokeChecksum, GoldenChecksum: record.GoldenChecksum,
		ProbedAt: record.ProbedAt.UTC(), Passing: record.Passing,
	}
}

func (store *PlatformActivationStore) isPostgres() bool {
	driver, ok := store.client.DialectDriver()
	return ok && driver.Dialect() == dialect.Postgres
}

func mapActivatedCapability(record *generate.ActivatedJudgeCapability) capability.ActivatedCapability {
	if record == nil {
		return capability.ActivatedCapability{}
	}
	return capability.ActivatedCapability{
		ID: record.ID, ReleaseID: record.ReleaseID, ActorID: record.ActorID,
		ApprovedReleaseManifestChecksum: record.ApprovedReleaseManifestChecksum,
		AuditID:                         record.AuditID, Reason: record.Reason,
		Tuple: capability.CapabilityTuple{
			JudgeSpecProtocol: record.JudgeSpecProtocol, RuntimeKey: record.RuntimeKey,
			RuntimeProfileVersion:   record.RuntimeProfileVersion,
			CompilerVersion:         record.CompilerVersion,
			RuntimeBootstrapVersion: record.RuntimeBootstrapVersion,
			CompileCommandID:        record.CompileCommandID, RunCommandID: record.RunCommandID,
			ImageDigest:     record.ImageDigest,
			ManifestVersion: record.ManifestVersion, ManifestChecksum: record.ManifestChecksum,
			SupervisorProtocol: record.SupervisorProtocol, CheckerKey: record.CheckerKey,
			CheckerVersion:                record.CheckerVersion,
			CheckerImplementationChecksum: record.CheckerImplementationChecksum,
			CapabilityProfileHash:         record.CapabilityProfileHash,
		},
		CheckerImplementationChecksum: record.CheckerImplementationChecksum,
		SmokeChecksum:                 record.SmokeChecksum, GoldenChecksum: record.GoldenChecksum,
		ProofWorkerInstanceID: record.ProofWorkerInstanceID, ProofBootID: record.ProofBootID,
		ProofEvidenceID: record.ProofEvidenceID, State: capability.ActivationState(record.State),
		ActivatedAt: record.ActivatedAt.UTC(), RetiredAt: record.RetiredAt,
	}
}

var (
	_ capability.ActivationStore  = (*PlatformActivationStore)(nil)
	_ capability.ActivationReader = (*PlatformActivationStore)(nil)
)
