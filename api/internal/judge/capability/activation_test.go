package capability

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestActivationServiceCreatesAuditedPlatformActivationOnlyThroughSeparateStore(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)
	freshness := 2 * time.Minute
	store := &recordingActivationStore{}
	service, err := NewActivationService(store, freshness)
	if err != nil {
		t.Fatalf("NewActivationService() error = %v", err)
	}
	tuple := activationTestTuple(t)
	request := ActivationRequest{
		ReleaseID:                       "judge-release.2026.07.28.1",
		ApprovedReleaseManifestChecksum: "sha256:b45e743f0c94809cc04fc395c3345cce7749a997393870ba2c37a9de783b41dc",
		ActorID:                         uuid.New(), AuditID: uuid.New(),
		Reason: "approve exact launch capability after reviewed golden smoke evidence",
		Tuple:  tuple,
	}

	activated, err := service.Activate(context.Background(), request, now)
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if store.calls != 1 {
		t.Fatalf("activation store calls = %d, want 1", store.calls)
	}
	if store.notBefore != now.Add(-freshness) {
		t.Fatalf("fresh worker cutoff = %v, want %v", store.notBefore, now.Add(-freshness))
	}
	if activated.ID == uuid.Nil || activated.State != ActivationStateActive ||
		activated.ActivatedAt != now || activated.ReleaseID != request.ReleaseID ||
		activated.ApprovedReleaseManifestChecksum != request.ApprovedReleaseManifestChecksum ||
		activated.ActorID != request.ActorID || activated.AuditID != request.AuditID ||
		activated.Reason != request.Reason || activated.Tuple != tuple ||
		activated.CheckerImplementationChecksum != tuple.CheckerImplementationChecksum ||
		activated.SmokeChecksum != activationTestSmokeChecksum ||
		activated.GoldenChecksum != activationTestGoldenChecksum {
		t.Fatalf("activated capability = %#v, want exact audited request", activated)
	}
}

type recordingActivationStore struct {
	calls     int
	notBefore time.Time
}

func (store *recordingActivationStore) CreateIfFreshWorkerProves(
	_ context.Context,
	activation ActivatedCapability,
	notBefore time.Time,
) (ActivatedCapability, error) {
	store.calls++
	store.notBefore = notBefore
	activation.ProofWorkerInstanceID = uuid.New()
	activation.ProofBootID = uuid.New()
	activation.ProofEvidenceID = uuid.New()
	activation.CheckerImplementationChecksum = activation.Tuple.CheckerImplementationChecksum
	activation.SmokeChecksum = activationTestSmokeChecksum
	activation.GoldenChecksum = activationTestGoldenChecksum
	return activation, nil
}

func (store *recordingActivationStore) Retire(
	_ context.Context,
	_ RetirementRequest,
	_ time.Time,
) error {
	return nil
}

func activationTestTuple(t *testing.T) CapabilityTuple {
	t.Helper()
	tuple := CapabilityTuple{
		JudgeSpecProtocol: "judge-spec.v1", RuntimeKey: "cpp17", RuntimeProfileVersion: "2026.07.1",
		CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
		CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
		ImageDigest:        "sha256:aaed1b901a2090c1b63ee5cacd1bc2536f78f741f9a750a85a7f9f9fe1dd6f34",
		ManifestVersion:    "cpp17-artifact.v1",
		ManifestChecksum:   "sha256:b45e743f0c94809cc04fc395c3345cce7749a997393870ba2c37a9de783b41dc",
		SupervisorProtocol: "supervisor.v1", CheckerKey: "exact", CheckerVersion: "v1",
		CheckerImplementationChecksum: "sha256:0ca1bb8dc836e6d75ca2d96e723cd8f0f4de0c1345873842eb3cda599347091a",
	}
	hash, err := ComputeCapabilityProfileHash(tuple)
	if err != nil {
		t.Fatalf("ComputeCapabilityProfileHash() error = %v", err)
	}
	tuple.CapabilityProfileHash = hash
	return tuple
}

const (
	activationTestSmokeChecksum  = "sha256:68f62d9d9db290b381b55027d14b9f5827e8fe00b102ac0c24c894b4f2aa50a9"
	activationTestGoldenChecksum = "sha256:f3c7dd28b1ecf7e40c4c695129fd1b67cc65a4149421133446f062886c24dd9d"
)
