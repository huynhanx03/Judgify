package usecase

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

func TestBuildLaunchProfilesRequiresAuditedNonPlaceholderReleaseManifest(t *testing.T) {
	t.Parallel()
	manifest := validReleaseManifestForTest(t)
	runtimes, checkers, err := BuildLaunchProfiles(manifest)
	if err != nil {
		t.Fatalf("BuildLaunchProfiles() error = %v", err)
	}
	if len(runtimes) != 4 {
		t.Fatalf("launch runtime count = %d, want exactly 4", len(runtimes))
	}
	if len(checkers) != 3 {
		t.Fatalf("launch checker count = %d, want exactly 3", len(checkers))
	}

	tests := []struct {
		name   string
		mutate func(*ApprovedReleaseManifest)
		rehash bool
	}{
		{name: "unapproved", mutate: func(value *ApprovedReleaseManifest) { value.Approved = false }},
		{name: "missing audit", mutate: func(value *ApprovedReleaseManifest) { value.AuditID = uuid.Nil }},
		{name: "stale binding", mutate: func(value *ApprovedReleaseManifest) {
			value.Runtimes[0].ImageDigest = releaseDigestForTest("changed-after-approval")
		}},
		{name: "unreviewed executable identity", mutate: func(value *ApprovedReleaseManifest) {
			value.Runtimes[0].CompileCommandID = "cpp17.compile.v2"
		}, rehash: true},
		{name: "missing runtime", mutate: func(value *ApprovedReleaseManifest) { value.Runtimes = value.Runtimes[1:] }, rehash: true},
		{name: "extra runtime", mutate: func(value *ApprovedReleaseManifest) { value.Runtimes = append(value.Runtimes, value.Runtimes[0]) }, rehash: true},
		{name: "missing explicit capability", mutate: func(value *ApprovedReleaseManifest) { value.Capabilities = nil }, rehash: true},
		{name: "capability references unreviewed pairing", mutate: func(value *ApprovedReleaseManifest) {
			value.Capabilities[0].CheckerKey = "unreviewed"
		}, rehash: true},
		{name: "uniform placeholder digest", mutate: func(value *ApprovedReleaseManifest) {
			value.Runtimes[0].ImageDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}, rehash: true},
		{name: "uniform placeholder manifest checksum", mutate: func(value *ApprovedReleaseManifest) {
			value.Runtimes[0].ManifestChecksum = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		}, rehash: true},
		{name: "uniform placeholder checker checksum", mutate: func(value *ApprovedReleaseManifest) {
			value.Checkers[0].ImplementationChecksum = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
			value.Capabilities[0].CheckerImplementationChecksum = value.Checkers[0].ImplementationChecksum
		}, rehash: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := manifest
			candidate.Runtimes = append([]RuntimeRelease(nil), manifest.Runtimes...)
			candidate.Checkers = append([]CheckerRelease(nil), manifest.Checkers...)
			candidate.Capabilities = append([]CapabilityRelease(nil), manifest.Capabilities...)
			test.mutate(&candidate)
			if test.rehash {
				checksum, err := ComputeReleaseManifestChecksum(candidate)
				if err != nil {
					t.Fatalf("ComputeReleaseManifestChecksum() error = %v", err)
				}
				candidate.ApprovalChecksum = checksum
			}
			if _, _, err := BuildLaunchProfiles(candidate); err == nil {
				t.Fatal("invalid production release manifest unexpectedly built active profiles")
			}
		})
	}
}

func validReleaseManifestForTest(t *testing.T) ApprovedReleaseManifest {
	t.Helper()
	manifest := ApprovedReleaseManifest{
		ReleaseID: "judge-release.2026.07.28.1",
		Approved:  true, ApprovedBy: uuid.New(), AuditID: uuid.New(),
		ApprovedAt: time.Date(2026, time.July, 28, 10, 30, 0, 0, time.UTC),
		Runtimes: []RuntimeRelease{
			{
				RuntimeKey: "cpp17", ProfileVersion: "2026.07.1",
				CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
				CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
				ImageDigest:     releaseDigestForTest("cpp17-image"),
				ManifestVersion: "cpp17-artifact.v1", ManifestChecksum: releaseDigestForTest("cpp17-manifest"),
			},
			{
				RuntimeKey: "python3", ProfileVersion: "2026.07.1",
				CompilerVersion: "cpython-3.12.14", RuntimeBootstrapVersion: "cpython-3.12.14",
				CompileCommandID: "python3.compile.v1", RunCommandID: "python3.run.v1",
				ImageDigest:     releaseDigestForTest("python3-image"),
				ManifestVersion: "python3-artifact.v1", ManifestChecksum: releaseDigestForTest("python3-manifest"),
			},
			{
				RuntimeKey: "java17", ProfileVersion: "2026.07.1",
				CompilerVersion: "javac-17.0.20", RuntimeBootstrapVersion: "openjdk-17.0.20",
				CompileCommandID: "java17.compile.v1", RunCommandID: "java17.run.v1",
				ImageDigest:     releaseDigestForTest("java17-image"),
				ManifestVersion: "java17-artifact.v1", ManifestChecksum: releaseDigestForTest("java17-manifest"),
			},
			{
				RuntimeKey: "go", ProfileVersion: "2026.07.1",
				CompilerVersion: "go1.25.10", RuntimeBootstrapVersion: "go-module-workspace.v1",
				CompileCommandID: "go.compile.v1", RunCommandID: "go.run.v1",
				ImageDigest:     releaseDigestForTest("go-image"),
				ManifestVersion: "go-artifact.v1", ManifestChecksum: releaseDigestForTest("go-manifest"),
			},
		},
		Checkers: []CheckerRelease{
			{CheckerKey: "exact", Version: "v1", ImplementationChecksum: releaseDigestForTest("checker-exact")},
			{CheckerKey: "token", Version: "v1", ImplementationChecksum: releaseDigestForTest("checker-token")},
			{CheckerKey: "float", Version: "v1", ImplementationChecksum: releaseDigestForTest("checker-float")},
		},
		Capabilities: []CapabilityRelease{{
			JudgeSpecProtocol: "judge-spec.v1",
			RuntimeKey:        "cpp17", RuntimeProfileVersion: "2026.07.1",
			CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
			CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
			ImageDigest:     releaseDigestForTest("cpp17-image"),
			ManifestVersion: "cpp17-artifact.v1", ManifestChecksum: releaseDigestForTest("cpp17-manifest"),
			SupervisorProtocol: "supervisor.v1", CheckerKey: "exact", CheckerVersion: "v1",
			CheckerImplementationChecksum: releaseDigestForTest("checker-exact"),
			CapabilityProfileHash:         releaseDigestForTest("cpp17-exact-capability"),
		}},
	}
	checksum, err := ComputeReleaseManifestChecksum(manifest)
	if err != nil {
		t.Fatalf("ComputeReleaseManifestChecksum() error = %v", err)
	}
	manifest.ApprovalChecksum = checksum
	return manifest
}

func releaseDigestForTest(label string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(label)))
}

func launchProfilesForTest(t *testing.T) ([]entity.RuntimeProfile, []entity.CheckerProfile) {
	t.Helper()
	runtimes, checkers, err := BuildLaunchProfiles(validReleaseManifestForTest(t))
	if err != nil {
		t.Fatalf("BuildLaunchProfiles() test fixture error = %v", err)
	}
	return runtimes, checkers
}

func runtimeProfilesForTest(t *testing.T) []entity.RuntimeProfile {
	t.Helper()
	runtimes, _ := launchProfilesForTest(t)
	return runtimes
}

func checkerProfilesForTest(t *testing.T) []entity.CheckerProfile {
	t.Helper()
	_, checkers := launchProfilesForTest(t)
	return checkers
}
