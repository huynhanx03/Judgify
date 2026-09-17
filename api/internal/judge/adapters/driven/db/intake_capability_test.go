package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	judgecapability "github.com/huynhanx03/judgify/internal/judge/capability"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

func TestSubmissionCapabilityResolverLocksExactTupleBeforeBoundedAdmission(t *testing.T) {
	client, mock := newPublicRuntimeCatalogMockClient(t)
	resolver := newCapabilityResolverForTest(t, client)
	resolver.disk = capabilityDiskProbe{bytes: 10 << 30}
	now := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectExactSubmissionCapability(t, mock)
	expectCapabilityBacklogLock(mock)
	mock.ExpectQuery(`SELECT COUNT\(\*\), MIN\(created_at\), clock_timestamp\(\)`).
		WillReturnRows(sqlmock.NewRows([]string{
			"count", "oldest", "now",
		}).AddRow(4, now.Add(-time.Minute), now))
	mock.ExpectCommit()

	var snapshot intake.CapabilitySnapshot
	err := platformtx.NewEntUnitOfWork(client.Client()).Within(
		context.Background(),
		func(txCtx context.Context) error {
			var resolveErr error
			snapshot, resolveErr = resolver.Resolve(
				txCtx,
				intake.CapabilityRequest{
					RuntimeKey:     "go",
					CheckerKey:     "exact",
					CheckerVersion: "v1",
				},
			)
			return resolveErr
		},
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if snapshot.ReleaseID != "judge-release.2026.07.28.1" ||
		snapshot.ApprovedReleaseManifestChecksum != capabilityDigestForTest("67") ||
		snapshot.CheckerImplementationChecksum != capabilityDigestForTest("89") ||
		snapshot.CapabilityProfileHash != capabilityProfileHashForTest(t) ||
		snapshot.Language != "go" ||
		snapshot.RuntimeProfileVersion != "2026.07.1" ||
		snapshot.TotalAttemptWallMilliseconds != 0 {
		t.Fatalf("capability snapshot = %+v", snapshot)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}

func TestSubmissionCapabilityResolverRejectsBacklogAgeBeforeDiskProbe(t *testing.T) {
	client, mock := newPublicRuntimeCatalogMockClient(t)
	resolver := newCapabilityResolverForTest(t, client)
	disk := &countingCapabilityDiskProbe{bytes: 10 << 30}
	resolver.disk = disk
	now := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectExactSubmissionCapability(t, mock)
	expectCapabilityBacklogLock(mock)
	mock.ExpectQuery(`SELECT COUNT\(\*\), MIN\(created_at\), clock_timestamp\(\)`).
		WillReturnRows(sqlmock.NewRows([]string{
			"count", "oldest", "now",
		}).AddRow(1, now.Add(-30*time.Minute), now))
	mock.ExpectRollback()

	err := platformtx.NewEntUnitOfWork(client.Client()).Within(
		context.Background(),
		func(txCtx context.Context) error {
			_, resolveErr := resolver.Resolve(
				txCtx,
				intake.CapabilityRequest{
					RuntimeKey:     "go",
					CheckerKey:     "exact",
					CheckerVersion: "v1",
				},
			)
			return resolveErr
		},
	)
	if !errors.Is(err, intake.ErrCapabilityUnavailable) {
		t.Fatalf("Resolve() error = %v, want capability unavailable", err)
	}
	if disk.calls != 0 {
		t.Fatalf("disk probe calls = %d, want 0", disk.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}

func TestSubmissionCapabilityResolverRejectsInsufficientLocalDisk(t *testing.T) {
	client, mock := newPublicRuntimeCatalogMockClient(t)
	resolver := newCapabilityResolverForTest(t, client)
	resolver.disk = capabilityDiskProbe{bytes: 1}
	now := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectExactSubmissionCapability(t, mock)
	expectCapabilityBacklogLock(mock)
	mock.ExpectQuery(`SELECT COUNT\(\*\), MIN\(created_at\), clock_timestamp\(\)`).
		WillReturnRows(sqlmock.NewRows([]string{
			"count", "oldest", "now",
		}).AddRow(0, nil, now))
	mock.ExpectRollback()

	err := platformtx.NewEntUnitOfWork(client.Client()).Within(
		context.Background(),
		func(txCtx context.Context) error {
			_, resolveErr := resolver.Resolve(
				txCtx,
				intake.CapabilityRequest{
					RuntimeKey:     "go",
					CheckerKey:     "exact",
					CheckerVersion: "v1",
				},
			)
			return resolveErr
		},
	)
	if !errors.Is(err, intake.ErrCapabilityUnavailable) {
		t.Fatalf("Resolve() error = %v, want capability unavailable", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}

func newCapabilityResolverForTest(
	t *testing.T,
	client *internalent.EntClient,
) *SubmissionCapabilityResolver {
	t.Helper()
	resolver, err := NewSubmissionCapabilityResolver(
		client,
		SubmissionCapabilityConfig{
			ReleaseID:                       "judge-release.2026.07.28.1",
			ApprovedReleaseManifestChecksum: capabilityDigestForTest("67"),
			MaximumReadyBacklog:             100,
			MaximumOldestReadyAge:           30 * time.Minute,
			MinimumFreeDiskBytes:            5 << 30,
			AdmissionDiskPath:               ".",
		},
	)
	if err != nil {
		t.Fatalf("NewSubmissionCapabilityResolver() error = %v", err)
	}
	return resolver
}

func expectExactSubmissionCapability(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	mock.ExpectQuery(`SELECT activation\.release_id,\s+activation\.approved_release_manifest_checksum`).
		WithArgs(
			"judge-release.2026.07.28.1",
			capabilityDigestForTest("67"),
			"go",
			"exact",
			"v1",
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"release_id",
			"approved_release_manifest_checksum",
			"language",
			"judge_spec_protocol",
			"runtime_profile_version",
			"compiler_version",
			"runtime_bootstrap_version",
			"sandbox_image_digest",
			"compile_command_id",
			"run_command_id",
			"manifest_version",
			"manifest_checksum",
			"artifact_manifest",
			"supervisor_protocol",
			"capability_profile_hash",
			"checker_implementation_checksum",
			"compile_cpu_time_ms",
			"compile_wall_time_ms",
		}).AddRow(
			"judge-release.2026.07.28.1",
			capabilityDigestForTest("67"),
			"go",
			"judge-spec.v1",
			"2026.07.1",
			"go1.25.10",
			"go-module-workspace.v1",
			capabilityDigestForTest("01"),
			"go.compile.v1",
			"go.run.v1",
			"go-artifact.v1",
			capabilityDigestForTest("23"),
			[]byte(`[{"path":"main","executable":true}]`),
			"supervisor.v1",
			capabilityProfileHashForTest(t),
			capabilityDigestForTest("89"),
			10_000,
			10_000,
		))
}

func TestCapabilitySnapshotBindingRejectsProjectionDrift(t *testing.T) {
	t.Parallel()
	config := SubmissionCapabilityConfig{
		ReleaseID:                       "judge-release.2026.07.28.1",
		ApprovedReleaseManifestChecksum: capabilityDigestForTest("67"),
	}
	request := intake.CapabilityRequest{
		RuntimeKey: "go", CheckerKey: "exact", CheckerVersion: "v1",
	}
	snapshot := capabilitySnapshotForTest(t)
	if !capabilitySnapshotMatchesRequest(config, request, snapshot) {
		t.Fatal("exact capability snapshot did not match its resolver binding")
	}

	tests := []struct {
		name   string
		mutate func(*intake.CapabilitySnapshot)
	}{
		{name: "release", mutate: func(value *intake.CapabilitySnapshot) {
			value.ReleaseID = "judge-release.other"
		}},
		{name: "approval", mutate: func(value *intake.CapabilitySnapshot) {
			value.ApprovedReleaseManifestChecksum = capabilityDigestForTest("68")
		}},
		{name: "runtime", mutate: func(value *intake.CapabilitySnapshot) {
			value.RuntimeProfileVersion = "2026.07.2"
		}},
		{name: "checker implementation", mutate: func(value *intake.CapabilitySnapshot) {
			value.CheckerImplementationChecksum = capabilityDigestForTest("90")
		}},
		{name: "profile hash", mutate: func(value *intake.CapabilitySnapshot) {
			value.CapabilityProfileHash = capabilityDigestForTest("46")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := snapshot
			test.mutate(&changed)
			if capabilitySnapshotMatchesRequest(config, request, changed) {
				t.Fatal("drifted capability snapshot matched resolver binding")
			}
		})
	}
}

func capabilityProfileHashForTest(t *testing.T) string {
	t.Helper()
	return capabilitySnapshotForTest(t).CapabilityProfileHash
}

func capabilitySnapshotForTest(t *testing.T) intake.CapabilitySnapshot {
	t.Helper()
	snapshot := intake.CapabilitySnapshot{
		ReleaseID:                       "judge-release.2026.07.28.1",
		ApprovedReleaseManifestChecksum: capabilityDigestForTest("67"),
		Language:                        "go",
		JudgeSpecProtocol:               "judge-spec.v1",
		RuntimeProfileVersion:           "2026.07.1",
		CompilerVersion:                 "go1.25.10",
		RuntimeBootstrapVersion:         "go-module-workspace.v1",
		SandboxImageDigest:              capabilityDigestForTest("01"),
		CompileCommandID:                "go.compile.v1",
		RunCommandID:                    "go.run.v1",
		ArtifactManifestVersion:         "go-artifact.v1",
		ArtifactManifestChecksum:        capabilityDigestForTest("23"),
		ArtifactManifest:                []byte(`[{"path":"main","executable":true}]`),
		SupervisorProtocolVersion:       "supervisor.v1",
		CheckerImplementationChecksum:   capabilityDigestForTest("89"),
		CompileCPUTimeMilliseconds:      10_000,
		CompileWallTimeMilliseconds:     10_000,
	}
	tuple := judgecapability.CapabilityTuple{
		JudgeSpecProtocol:             snapshot.JudgeSpecProtocol,
		RuntimeKey:                    "go",
		RuntimeProfileVersion:         snapshot.RuntimeProfileVersion,
		CompilerVersion:               snapshot.CompilerVersion,
		RuntimeBootstrapVersion:       snapshot.RuntimeBootstrapVersion,
		CompileCommandID:              snapshot.CompileCommandID,
		RunCommandID:                  snapshot.RunCommandID,
		ImageDigest:                   snapshot.SandboxImageDigest,
		ManifestVersion:               snapshot.ArtifactManifestVersion,
		ManifestChecksum:              snapshot.ArtifactManifestChecksum,
		SupervisorProtocol:            snapshot.SupervisorProtocolVersion,
		CheckerKey:                    "exact",
		CheckerVersion:                "v1",
		CheckerImplementationChecksum: snapshot.CheckerImplementationChecksum,
	}
	hash, err := judgecapability.ComputeCapabilityProfileHash(tuple)
	if err != nil {
		t.Fatalf("ComputeCapabilityProfileHash() error = %v", err)
	}
	snapshot.CapabilityProfileHash = hash
	return snapshot
}

func expectCapabilityBacklogLock(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT pg_advisory_xact_lock`).
		WithArgs(
			submissioncontract.JudgeBacklogAdmissionAdvisoryLock,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"pg_advisory_xact_lock",
		}).AddRow(nil))
}

func capabilityDigestForTest(pair string) string {
	return "sha256:" +
		pair + pair + pair + pair + pair + pair + pair + pair +
		pair + pair + pair + pair + pair + pair + pair + pair +
		pair + pair + pair + pair + pair + pair + pair + pair +
		pair + pair + pair + pair + pair + pair + pair + pair
}

type capabilityDiskProbe struct {
	bytes uint64
	err   error
}

func (probe capabilityDiskProbe) FreeBytes(
	context.Context,
	string,
) (uint64, error) {
	return probe.bytes, probe.err
}

type countingCapabilityDiskProbe struct {
	bytes uint64
	err   error
	calls int
}

func (probe *countingCapabilityDiskProbe) FreeBytes(
	context.Context,
	string,
) (uint64, error) {
	probe.calls++
	return probe.bytes, probe.err
}
