package db

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ports"
)

func TestRejudgeIdentityIsExactCommandScoped(t *testing.T) {
	t.Parallel()
	command := ports.RejudgeCommand{
		SubmissionID: uuid.New(), ExpectedVersion: 7, IdempotencyKey: uuid.New(),
		Reason: "checker normalization correction",
	}
	first := deterministicRejudgeID(command)
	replay := deterministicRejudgeID(command)
	changedReason := command
	changedReason.Reason = "different reason"
	changedVersion := command
	changedVersion.ExpectedVersion++

	require.Equal(t, first, replay)
	require.NotEqual(t, first, deterministicRejudgeID(changedReason))
	require.NotEqual(t, first, deterministicRejudgeID(changedVersion))
	require.NotEqual(t, uuid.Nil, derivedOperationID(first, "spec"))
	require.NotEqual(t, derivedOperationID(first, "spec"), derivedOperationID(first, "audit"))
}

func TestRejudgeChecksumPinsNewGenerationWithoutChangingSourceSnapshot(t *testing.T) {
	t.Parallel()
	spec := validRejudgeSpec(t)
	submissionID := uuid.New()
	first, err := spec.checksum(submissionID, 2)
	require.NoError(t, err)
	second, err := spec.checksum(submissionID, 3)
	require.NoError(t, err)
	require.NotEqual(t, first, second, "generation is part of immutable spec provenance")

	mutations := []struct {
		name   string
		mutate func(*rejudgeSpec)
	}{
		{name: "release", mutate: func(value *rejudgeSpec) {
			value.releaseID = "judge-release.other"
		}},
		{name: "approval", mutate: func(value *rejudgeSpec) {
			value.approvedReleaseManifestChecksum =
				"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		}},
		{name: "checker implementation", mutate: func(value *rejudgeSpec) {
			value.checkerImplementationChecksum =
				"sha256:9999999999999999999999999999999999999999999999999999999999999999"
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := spec
			mutation.mutate(&changed)
			got, checksumErr := changed.checksum(submissionID, 2)
			require.NoError(t, checksumErr)
			require.NotEqual(t, first, got)
		})
	}
}

func TestRejudgeRecordRejectsUnsafeReason(t *testing.T) {
	t.Parallel()
	command := ports.RejudgeCommand{
		SubmissionID: uuid.New(), ExpectedVersion: 1, IdempotencyKey: uuid.New(), Reason: "approved",
	}
	require.True(t, validRejudgeRecord(command))
	command.Reason = "hidden\x00test"
	require.False(t, validRejudgeRecord(command))
	command.Reason = strings.Repeat("a", maximumOperationReasonBytes+1)
	require.False(t, validRejudgeRecord(command))
}

func TestRejudgeRequiresThePreviousImmutableSpecChecksum(t *testing.T) {
	t.Parallel()
	spec := validRejudgeSpec(t)
	submissionID := uuid.New()
	checksum, err := spec.checksum(submissionID, 4)
	require.NoError(t, err)
	spec.specChecksum = append([]byte(nil), checksum[:]...)
	require.True(t, spec.matchesChecksum(submissionID, 4))

	tampered := spec
	tampered.releaseID = "judge-release.other"
	require.False(t, tampered.matchesChecksum(submissionID, 4))

	tampered = spec
	tampered.specChecksum = append([]byte(nil), spec.specChecksum[:31]...)
	require.False(t, tampered.matchesChecksum(submissionID, 4))
	require.False(t, spec.matchesChecksum(submissionID, 5))
}

func validRejudgeSpec(t *testing.T) rejudgeSpec {
	t.Helper()
	digest := func(character string) string { return "sha256:" + strings.Repeat(character, 64) }
	return rejudgeSpec{
		userID: uuid.New(), problemID: uuid.New(), problemRevisionID: uuid.New(), testsetRevisionID: uuid.New(),
		language: "go", runtimeKey: "go", runtimeProfileVersion: "2026.07.1", compilerVersion: "go1.25.10",
		runtimeBootstrapVersion: "workspace.v1", sandboxImageDigest: digest("a"), compileCommandID: "go.compile.v1",
		runCommandID: "go.run.v1", artifactManifestVersion: "manifest.v1", artifactManifestChecksum: digest("b"),
		artifactManifest:          json.RawMessage(`[{"path":"main","executable":true,"maximum_bytes":1,"maximum_files":1,"required":true}]`),
		supervisorProtocolVersion: "supervisor.v1", checkerKey: "exact", checkerVersion: "v1", checkerConfig: json.RawMessage(`{}`),
		releaseID:                       "judge-release.2026.07.28.1",
		approvedReleaseManifestChecksum: digest("d"),
		checkerImplementationChecksum:   digest("e"),
		capabilityProfileHash:           digest("c"), rewardSnapshot: json.RawMessage(`{"base_exp":1}`), rewardSnapshotChecksum: bytes32(1),
		cpuTimeLimitMS: 1000, wallTimeLimitMS: 2000, memoryLimitKB: 1024, outputLimitBytes: 1024, processLimit: 1,
		compileCPUTimeLimitMS: 1000, compileWallTimeLimitMS: 1000, totalAttemptWallTimeLimitMS: 2000,
		sourceSHA256: bytes32(2), judgeSpecProtocol: "judge-spec.v1",
	}
}

func bytes32(value byte) []byte {
	result := make([]byte, 32)
	result[0] = value
	return result
}
