package runtimecatalog

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/judge/capability"
	problemports "github.com/huynhanx03/judgify/internal/ports"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
	problemservice "github.com/huynhanx03/judgify/internal/problem/usecase"
)

func TestActivationBinderAcceptsOnlyExactActivatedReleaseTuple(t *testing.T) {
	t.Parallel()
	tuple := testTuple(t)
	catalog := testCatalog(t, "judge-release.2026.07.29.1", tuple)
	reader := &activationReaderFake{found: true, activation: capability.ActivatedCapability{
		ID: uuid.New(), ReleaseID: catalog.ReleaseID(), State: capability.ActivationStateActive,
		ActorID: uuid.New(), AuditID: uuid.New(),
		Tuple: tuple, ApprovedReleaseManifestChecksum: catalog.ApprovalChecksum(),
		CheckerImplementationChecksum: testCheckerChecksum,
		SmokeChecksum:                 testSmokeChecksum, GoldenChecksum: testGoldenChecksum,
		ProofWorkerInstanceID: uuid.New(), ProofBootID: uuid.New(), ProofEvidenceID: uuid.New(),
		ActivatedAt: time.Date(2026, time.July, 29, 8, 0, 0, 0, time.UTC),
	}}
	binder, err := NewActivationBinder(catalog, reader)
	if err != nil {
		t.Fatalf("NewActivationBinder() error = %v", err)
	}
	bound, err := binder.Bind(context.Background(), tuple)
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if reader.releaseID != catalog.ReleaseID() || reader.tuple != tuple ||
		bound.Activation.ID != reader.activation.ID || bound.Runtime.Key != tuple.RuntimeKey ||
		bound.Checker.Key != tuple.CheckerKey {
		t.Fatalf("Bind() did not preserve exact tuple activation: %#v", bound)
	}
}

func TestActivationBinderRejectsMismatchedOrAmbiguousActivation(t *testing.T) {
	t.Parallel()
	tuple := testTuple(t)
	catalog := testCatalog(t, "judge-release.2026.07.29.1", tuple)
	valid := capability.ActivatedCapability{
		ID: uuid.New(), ReleaseID: catalog.ReleaseID(), State: capability.ActivationStateActive,
		ActorID: uuid.New(), AuditID: uuid.New(),
		Tuple: tuple, ApprovedReleaseManifestChecksum: catalog.ApprovalChecksum(),
		CheckerImplementationChecksum: testCheckerChecksum,
		SmokeChecksum:                 testSmokeChecksum, GoldenChecksum: testGoldenChecksum,
		ProofWorkerInstanceID: uuid.New(), ProofBootID: uuid.New(), ProofEvidenceID: uuid.New(),
		ActivatedAt: time.Date(2026, time.July, 29, 8, 0, 0, 0, time.UTC),
	}
	for name, mutate := range map[string]func(*activationReaderFake){
		"missing":         func(reader *activationReaderFake) { reader.found = false },
		"different tuple": func(reader *activationReaderFake) { reader.activation.Tuple.CheckerVersion = "v2" },
		"retired":         func(reader *activationReaderFake) { reader.activation.State = capability.ActivationStateRetired },
		"wrong checksum": func(reader *activationReaderFake) {
			reader.activation.CheckerImplementationChecksum = testOtherChecksum
		},
		"missing proof": func(reader *activationReaderFake) { reader.activation.ProofBootID = uuid.Nil },
	} {
		t.Run(name, func(t *testing.T) {
			reader := &activationReaderFake{activation: valid, found: true}
			mutate(reader)
			binder, err := NewActivationBinder(catalog, reader)
			if err != nil {
				t.Fatalf("NewActivationBinder() error = %v", err)
			}
			if _, err := binder.Bind(context.Background(), tuple); !errors.Is(err, ErrCapabilityBindingDenied) {
				t.Fatalf("Bind() error = %v, want %v", err, ErrCapabilityBindingDenied)
			}
		})
	}
}

func TestCatalogBootstrapUsesExplicitTupleAllowList(t *testing.T) {
	t.Parallel()
	tuple := testTuple(t)
	catalog := testCatalog(t, "judge-release.2026.07.29.1", tuple)

	approved := catalog.ApprovedTuples()
	if len(approved) != 1 || approved[0] != tuple {
		t.Fatalf("ApprovedTuples() = %#v, want exactly %#v", approved, tuple)
	}
	approved[0].RuntimeKey = "mutated"
	if again := catalog.ApprovedTuples(); len(again) != 1 || again[0] != tuple {
		t.Fatalf("ApprovedTuples() leaked mutable catalog state: %#v", again)
	}
}

func TestCatalogFencesAuthoringMutationsToApprovedManifest(t *testing.T) {
	t.Parallel()
	manifest, _ := approvedManifestForCatalogTest(t)
	catalog, err := LoadApprovedRelease(manifest)
	if err != nil {
		t.Fatalf("LoadApprovedRelease() error = %v", err)
	}
	if len(catalog.CatalogVersion()) != 67 ||
		catalog.CatalogVersion()[:3] != "v1." {
		t.Fatalf("CatalogVersion() = %q, want opaque v1 digest", catalog.CatalogVersion())
	}
	if err := catalog.ValidateAuthoringCatalogVersion(
		context.Background(),
		catalog.CatalogVersion(),
	); err != nil {
		t.Fatalf("ValidateAuthoringCatalogVersion(current) error = %v", err)
	}
	if err := catalog.ValidateAuthoringCatalogVersion(
		context.Background(),
		"v1.bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	); !errors.Is(err, problemports.ErrAuthoringCatalogChanged) {
		t.Fatalf("ValidateAuthoringCatalogVersion(stale) error = %v", err)
	}

	rotatedManifest := manifest
	rotatedManifest.ApprovedAt = rotatedManifest.ApprovedAt.Add(time.Second)
	rotatedChecksum, err := problemservice.ComputeReleaseManifestChecksum(rotatedManifest)
	if err != nil {
		t.Fatalf("ComputeReleaseManifestChecksum(rotated) error = %v", err)
	}
	rotatedManifest.ApprovalChecksum = rotatedChecksum
	rotatedCatalog, err := LoadApprovedRelease(rotatedManifest)
	if err != nil {
		t.Fatalf("LoadApprovedRelease(rotated) error = %v", err)
	}
	if rotatedCatalog.CatalogVersion() == catalog.CatalogVersion() {
		t.Fatal("CatalogVersion() did not rotate with a newly approved manifest")
	}
	if err := rotatedCatalog.ValidateAuthoringCatalogVersion(
		context.Background(),
		catalog.CatalogVersion(),
	); !errors.Is(err, problemports.ErrAuthoringCatalogChanged) {
		t.Fatalf("rotated catalog accepted the previous version: %v", err)
	}

	changedManifest := manifest
	changedManifest.ApprovalChecksum = testOtherChecksum
	changed, err := LoadApprovedRelease(changedManifest)
	if err == nil || changed != nil {
		t.Fatal("LoadApprovedRelease() accepted a forged approval checksum")
	}
}

func TestCatalogDelegatesProblemPublicationValidationToReviewedProfiles(t *testing.T) {
	t.Parallel()
	tuple := testTuple(t)
	catalog := testCatalog(t, "judge-release.2026.07.29.1", tuple)
	if err := catalog.ValidateRuntimeKeys(context.Background(), []string{"cpp17"}); err != nil {
		t.Fatalf("ValidateRuntimeKeys() error = %v", err)
	}
	if err := catalog.ValidateChecker(context.Background(), "exact", "v1", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("ValidateChecker() error = %v", err)
	}
	if err := catalog.ValidateExecutionPairings(
		context.Background(),
		[]string{"cpp17"},
		"exact",
		"v1",
	); err != nil {
		t.Fatalf("ValidateExecutionPairings() error = %v", err)
	}
	if err := catalog.ValidateChecker(
		context.Background(),
		"token",
		"v1",
		json.RawMessage(`{}`),
	); err != nil {
		t.Fatalf("ValidateChecker(token) error = %v", err)
	}
	if err := catalog.ValidateExecutionPairings(
		context.Background(),
		[]string{"cpp17"},
		"token",
		"v1",
	); !errors.Is(err, problemports.ErrExecutionCapabilityUnavailable) {
		t.Fatalf("unapproved pairing error = %v", err)
	}
	if err := catalog.ValidateRuntimeKeys(context.Background(), []string{"unreviewed"}); err == nil {
		t.Fatal("ValidateRuntimeKeys() accepted an unreviewed key")
	}
}

func TestCatalogRejectsPairingForAStaleRuntimeProfileVersion(t *testing.T) {
	t.Parallel()
	staleTuple := testTuple(t)
	staleTuple.RuntimeProfileVersion = "2025.01.1"
	hash, err := capability.ComputeCapabilityProfileHash(staleTuple)
	if err != nil {
		t.Fatalf("ComputeCapabilityProfileHash() error = %v", err)
	}
	staleTuple.CapabilityProfileHash = hash
	catalog := testCatalog(t, "judge-release.2026.07.29.1", staleTuple)

	err = catalog.ValidateExecutionPairings(
		context.Background(),
		[]string{"cpp17"},
		"exact",
		"v1",
	)
	if !errors.Is(err, problemports.ErrExecutionCapabilityUnavailable) {
		t.Fatalf("ValidateExecutionPairings() error = %v", err)
	}
}

func TestActivationBinderRejectsActivatedTupleOutsideReleaseAllowList(t *testing.T) {
	t.Parallel()
	approved := testTuple(t)
	other := approved
	other.SupervisorProtocol = "supervisor.v2"
	hash, err := capability.ComputeCapabilityProfileHash(other)
	if err != nil {
		t.Fatalf("ComputeCapabilityProfileHash() error = %v", err)
	}
	other.CapabilityProfileHash = hash
	catalog := testCatalog(t, "judge-release.2026.07.29.1", approved)
	reader := &activationReaderFake{found: true, activation: capability.ActivatedCapability{
		ID: uuid.New(), ReleaseID: catalog.ReleaseID(), State: capability.ActivationStateActive,
		ActorID: uuid.New(), AuditID: uuid.New(), Tuple: other,
		ApprovedReleaseManifestChecksum: catalog.ApprovalChecksum(),
		CheckerImplementationChecksum:   testCheckerChecksum,
		SmokeChecksum:                   testSmokeChecksum, GoldenChecksum: testGoldenChecksum,
		ProofWorkerInstanceID: uuid.New(), ProofBootID: uuid.New(), ProofEvidenceID: uuid.New(),
		ActivatedAt: time.Date(2026, time.July, 29, 8, 0, 0, 0, time.UTC),
	}}
	binder, err := NewActivationBinder(catalog, reader)
	if err != nil {
		t.Fatalf("NewActivationBinder() error = %v", err)
	}
	if _, err := binder.Bind(context.Background(), other); !errors.Is(err, ErrCapabilityBindingDenied) {
		t.Fatalf("Bind() error = %v, want %v", err, ErrCapabilityBindingDenied)
	}
	if reader.releaseID != "" {
		t.Fatal("Bind() consulted durable activation for a tuple outside the release allow-list")
	}
}

func TestLoadApprovedReleaseRejectsUnapprovedManifest(t *testing.T) {
	t.Parallel()
	_, err := LoadApprovedRelease(problemservice.ApprovedReleaseManifest{ReleaseID: "judge-release.2026.07.29.1"})
	if !errors.Is(err, ErrRuntimeCatalogDenied) {
		t.Fatalf("LoadApprovedRelease() error = %v, want %v", err, ErrRuntimeCatalogDenied)
	}
}

func TestLoadApprovedReleaseUsesOnlyExplicitManifestTuples(t *testing.T) {
	t.Parallel()
	manifest, tuple := approvedManifestForCatalogTest(t)
	catalog, err := LoadApprovedRelease(manifest)
	if err != nil {
		t.Fatalf("LoadApprovedRelease() error = %v", err)
	}
	if got := catalog.ApprovedTuples(); len(got) != 1 || got[0] != tuple {
		t.Fatalf("ApprovedTuples() = %#v, want only %#v", got, tuple)
	}
	activeRuntimes := 0
	for _, profile := range catalog.RuntimeProfiles() {
		if profile.Active {
			activeRuntimes++
		}
	}
	activeCheckers := 0
	for _, profile := range catalog.CheckerProfiles() {
		if profile.Active {
			activeCheckers++
		}
	}
	if activeRuntimes != 1 || activeCheckers != 1 {
		t.Fatalf("active profile counts = %d/%d, want 1/1", activeRuntimes, activeCheckers)
	}
	if err := catalog.ValidateRuntimeKeys(context.Background(), []string{"python3"}); err == nil {
		t.Fatal("catalog exposed a runtime absent from every approved tuple")
	}
	if err := catalog.ValidateChecker(
		context.Background(),
		"token",
		"v1",
		json.RawMessage(`{}`),
	); err == nil {
		t.Fatal("catalog exposed a checker absent from every approved tuple")
	}

	manifest.Capabilities[0].CapabilityProfileHash = testOtherChecksum
	checksum, err := problemservice.ComputeReleaseManifestChecksum(manifest)
	if err != nil {
		t.Fatalf("ComputeReleaseManifestChecksum() error = %v", err)
	}
	manifest.ApprovalChecksum = checksum
	if _, err := LoadApprovedRelease(manifest); !errors.Is(err, ErrRuntimeCatalogDenied) {
		t.Fatalf("LoadApprovedRelease() invalid tuple hash error = %v, want %v", err, ErrRuntimeCatalogDenied)
	}
}

const (
	testImageDigest      = "sha256:aaed1b901a2090c1b63ee5cacd1bc2536f78f741f9a750a85a7f9f9fe1dd6f34"
	testManifestChecksum = "sha256:b45e743f0c94809cc04fc395c3345cce7749a997393870ba2c37a9de783b41dc"
	testCheckerChecksum  = "sha256:0ca1bb8dc836e6d75ca2d96e723cd8f0f4de0c1345873842eb3cda599347091a"
	testSmokeChecksum    = "sha256:68f62d9d9db290b381b55027d14b9f5827e8fe00b102ac0c24c894b4f2aa50a9"
	testGoldenChecksum   = "sha256:f3c7dd28b1ecf7e40c4c695129fd1b67cc65a4149421133446f062886c24dd9d"
	testOtherChecksum    = "sha256:1ca1bb8dc836e6d75ca2d96e723cd8f0f4de0c1345873842eb3cda599347091a"
)

func approvedManifestForCatalogTest(
	t *testing.T,
) (problemservice.ApprovedReleaseManifest, capability.CapabilityTuple) {
	t.Helper()
	cppImage := testDigest("catalog-cpp-image")
	cppManifest := testDigest("catalog-cpp-manifest")
	manifest := problemservice.ApprovedReleaseManifest{
		ReleaseID: "judge-release.2026.07.29.1", Approved: true,
		ApprovedBy: uuid.MustParse("c49b094c-0c1d-4f59-8dd1-56f7d9dd705e"),
		AuditID:    uuid.MustParse("f2bfdd31-5b55-4c94-9cfb-9f69e78a6847"),
		ApprovedAt: time.Date(2026, time.July, 29, 9, 0, 0, 0, time.UTC),
		Runtimes: []problemservice.RuntimeRelease{
			{RuntimeKey: "cpp17", ProfileVersion: "2026.07.1", CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl", CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1", ImageDigest: cppImage, ManifestVersion: "cpp17-artifact.v1", ManifestChecksum: cppManifest},
			{RuntimeKey: "python3", ProfileVersion: "2026.07.1", CompilerVersion: "cpython-3.12.14", RuntimeBootstrapVersion: "cpython-3.12.14", CompileCommandID: "python3.compile.v1", RunCommandID: "python3.run.v1", ImageDigest: testDigest("catalog-python-image"), ManifestVersion: "python3-artifact.v1", ManifestChecksum: testDigest("catalog-python-manifest")},
			{RuntimeKey: "java17", ProfileVersion: "2026.07.1", CompilerVersion: "javac-17.0.20", RuntimeBootstrapVersion: "openjdk-17.0.20", CompileCommandID: "java17.compile.v1", RunCommandID: "java17.run.v1", ImageDigest: testDigest("catalog-java-image"), ManifestVersion: "java17-artifact.v1", ManifestChecksum: testDigest("catalog-java-manifest")},
			{RuntimeKey: "go", ProfileVersion: "2026.07.1", CompilerVersion: "go1.25.10", RuntimeBootstrapVersion: "go-module-workspace.v1", CompileCommandID: "go.compile.v1", RunCommandID: "go.run.v1", ImageDigest: testDigest("catalog-go-image"), ManifestVersion: "go-artifact.v1", ManifestChecksum: testDigest("catalog-go-manifest")},
		},
		Checkers: []problemservice.CheckerRelease{
			{CheckerKey: "exact", Version: "v1", ImplementationChecksum: testDigest("catalog-checker-exact")},
			{CheckerKey: "token", Version: "v1", ImplementationChecksum: testDigest("catalog-checker-token")},
			{CheckerKey: "float", Version: "v1", ImplementationChecksum: testDigest("catalog-checker-float")},
		},
	}
	tuple := capability.CapabilityTuple{
		JudgeSpecProtocol: "judge-spec.v1", RuntimeKey: "cpp17", RuntimeProfileVersion: "2026.07.1",
		CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
		CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
		ImageDigest: cppImage, ManifestVersion: "cpp17-artifact.v1", ManifestChecksum: cppManifest,
		SupervisorProtocol: "supervisor.v1", CheckerKey: "exact", CheckerVersion: "v1",
		CheckerImplementationChecksum: manifest.Checkers[0].ImplementationChecksum,
	}
	hash, err := capability.ComputeCapabilityProfileHash(tuple)
	if err != nil {
		t.Fatalf("ComputeCapabilityProfileHash() error = %v", err)
	}
	tuple.CapabilityProfileHash = hash
	manifest.Capabilities = []problemservice.CapabilityRelease{{
		JudgeSpecProtocol: tuple.JudgeSpecProtocol, RuntimeKey: tuple.RuntimeKey,
		RuntimeProfileVersion: tuple.RuntimeProfileVersion, CompilerVersion: tuple.CompilerVersion,
		RuntimeBootstrapVersion: tuple.RuntimeBootstrapVersion, CompileCommandID: tuple.CompileCommandID,
		RunCommandID: tuple.RunCommandID, ImageDigest: tuple.ImageDigest,
		ManifestVersion: tuple.ManifestVersion, ManifestChecksum: tuple.ManifestChecksum,
		SupervisorProtocol: tuple.SupervisorProtocol, CheckerKey: tuple.CheckerKey,
		CheckerVersion:                tuple.CheckerVersion,
		CheckerImplementationChecksum: tuple.CheckerImplementationChecksum,
		CapabilityProfileHash:         tuple.CapabilityProfileHash,
	}}
	checksum, err := problemservice.ComputeReleaseManifestChecksum(manifest)
	if err != nil {
		t.Fatalf("ComputeReleaseManifestChecksum() error = %v", err)
	}
	manifest.ApprovalChecksum = checksum
	return manifest, tuple
}

func testDigest(label string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(label)))
}

func testCatalog(t *testing.T, releaseID string, approved ...capability.CapabilityTuple) *Catalog {
	t.Helper()
	registry, err := problemservice.NewRuntimeRegistry([]problementity.RuntimeProfile{{
		ID: uuid.New(), Key: "cpp17", Language: "cpp", DisplayName: "C++17",
		ProfileVersion: "2026.07.1", CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
		ImageDigest: testImageDigest, CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
		Compile: problementity.CommandTemplate{Executable: "/usr/bin/g++", Arguments: []problementity.CommandArgument{
			{Kind: problementity.ArgumentWorkerPath, Value: "source"},
			{Kind: problementity.ArgumentLiteral, Value: "-o"},
			{Kind: problementity.ArgumentWorkerPath, Value: "artifact"},
		}},
		Run:                   problementity.CommandTemplate{Executable: "/workspace/artifact/main"},
		DefaultCompileTimeout: 15 * time.Second, ManifestVersion: "cpp17-artifact.v1", ManifestChecksum: testManifestChecksum,
		Artifacts: []problementity.RuntimeArtifact{{Path: "main", Executable: true, MaximumBytes: 32 << 20, MaximumFiles: 1, Required: true}},
		Workspace: problementity.RuntimeWorkspace{SourcePath: "source/main.cpp", Values: []problementity.RuntimeWorkspaceValue{
			{Name: "source", Path: "/workspace/source/main.cpp"}, {Name: "artifact", Path: "/workspace/artifact/main"},
		}}, Active: true,
	}}, []problementity.CheckerProfile{
		{
			ID: uuid.New(), Key: "exact", Version: "v1", ImplementationChecksum: testCheckerChecksum,
			ConfigKind: problementity.CheckerConfigNone, Active: true,
		},
		{
			ID: uuid.New(), Key: "token", Version: "v1", ImplementationChecksum: testOtherChecksum,
			ConfigKind: problementity.CheckerConfigNone, Active: true,
		},
	})
	if err != nil {
		t.Fatalf("NewRuntimeRegistry() error = %v", err)
	}
	tuples := append([]capability.CapabilityTuple(nil), approved...)
	byHash := make(map[string]capability.CapabilityTuple, len(tuples))
	for _, tuple := range tuples {
		byHash[tuple.CapabilityProfileHash] = tuple
	}
	return &Catalog{
		releaseID: releaseID, approvalChecksum: testManifestChecksum,
		catalogVersion: authoringCatalogVersionForTest(releaseID, testManifestChecksum),
		profiles:       registry,
		approvedTuples: tuples, approvedByHash: byHash,
	}
}

func authoringCatalogVersionForTest(releaseID, checksum string) string {
	version, err := problemports.NewAuthoringCatalogVersion(releaseID, checksum)
	if err != nil {
		panic(err)
	}
	return version
}

func testTuple(t *testing.T) capability.CapabilityTuple {
	t.Helper()
	tuple := capability.CapabilityTuple{
		JudgeSpecProtocol: "judge-spec.v1", RuntimeKey: "cpp17", RuntimeProfileVersion: "2026.07.1",
		CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
		CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
		ImageDigest: testImageDigest, ManifestVersion: "cpp17-artifact.v1", ManifestChecksum: testManifestChecksum,
		SupervisorProtocol: "supervisor.v1", CheckerKey: "exact", CheckerVersion: "v1",
		CheckerImplementationChecksum: testCheckerChecksum,
	}
	hash, err := capability.ComputeCapabilityProfileHash(tuple)
	if err != nil {
		t.Fatalf("ComputeCapabilityProfileHash() error = %v", err)
	}
	tuple.CapabilityProfileHash = hash
	return tuple
}

type activationReaderFake struct {
	activation capability.ActivatedCapability
	found      bool
	releaseID  string
	tuple      capability.CapabilityTuple
}

func (reader *activationReaderFake) Active(
	_ context.Context,
	releaseID string,
	tuple capability.CapabilityTuple,
) (capability.ActivatedCapability, bool, error) {
	reader.releaseID, reader.tuple = releaseID, tuple
	return reader.activation, reader.found, nil
}
