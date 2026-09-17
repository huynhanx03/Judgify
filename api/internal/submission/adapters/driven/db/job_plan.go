package db

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ports"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	"github.com/huynhanx03/judgify/pkg/judging"
)

type persistedJudgePlan struct {
	plan             ports.JudgePlan
	artifactManifest json.RawMessage
	checkerConfig    json.RawMessage
	rewardSnapshot   json.RawMessage
	sourceDigest     []byte
	specDigest       []byte
	rewardDigest     []byte
	submissionCID    string
	jobCID           string
}

// ReadJudgePlan loads only immutable snapshots after fencing the exact worker,
// attempt, generation, owner, and opaque lease capability. Test bodies are not
// loaded here; they are read one case at a time through ReadJudgeCase.
func (repository *JobRepository) ReadJudgePlan(
	ctx context.Context,
	lease ports.JobLease,
) (ports.JudgePlan, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.JudgePlan{}, ErrInvalidJobRepository
	}
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return ports.JudgePlan{}, fmt.Errorf("begin judge plan read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return ports.JudgePlan{}, err
	}
	locked, err := repository.lockFencedLease(ctx, tx, lease, now)
	if err != nil {
		return ports.JudgePlan{}, err
	}
	if locked.submissionVersion != lease.SubmissionVersion {
		return ports.JudgePlan{}, ports.ErrLeaseLost
	}

	value, err := readPersistedJudgePlan(ctx, tx, lease.Job.ID, locked)
	if err != nil {
		return ports.JudgePlan{}, err
	}
	if err := verifyPersistedJudgePlan(&value, lease); err != nil {
		return ports.JudgePlan{}, err
	}
	testset, tests, err := readJudgeTestsetPlan(
		ctx,
		tx,
		value.plan.TestsetRevisionID,
		value.plan.ProblemID,
	)
	if err != nil {
		return ports.JudgePlan{}, err
	}
	value.plan.TestsetChecksum = testset.Checksum
	value.plan.Tests = tests
	if err := tx.Commit(); err != nil {
		return ports.JudgePlan{}, fmt.Errorf("commit judge plan read: %w", err)
	}
	return cloneJudgePlan(value.plan), nil
}

func readPersistedJudgePlan(
	ctx context.Context,
	tx *sql.Tx,
	jobID uuid.UUID,
	locked fencedLeaseState,
) (persistedJudgePlan, error) {
	var value persistedJudgePlan
	err := tx.QueryRowContext(ctx, `
SELECT
    spec.id, spec.submission_id, spec.generation, spec.user_id, spec.problem_id,
    spec.problem_revision_id, spec.testset_revision_id,
    spec.language, spec.judge_spec_protocol,
    spec.runtime_key, spec.runtime_profile_version, spec.compiler_version,
    spec.runtime_bootstrap_version, spec.sandbox_image_digest,
    spec.compile_command_id, spec.run_command_id,
    spec.artifact_manifest_version, spec.artifact_manifest_checksum,
	    spec.artifact_manifest, spec.supervisor_protocol_version,
	    spec.release_id, spec.approved_release_manifest_checksum,
	    spec.checker_implementation_checksum, spec.capability_profile_hash,
    spec.checker_key, spec.checker_version, spec.checker_config,
    spec.reward_snapshot, spec.reward_snapshot_checksum,
    spec.cpu_time_limit_ms, spec.wall_time_limit_ms, spec.memory_limit_kb,
    spec.output_limit_bytes, spec.process_limit,
    spec.compile_cpu_time_limit_ms, spec.compile_wall_time_limit_ms,
    spec.total_attempt_wall_time_limit_ms,
    submission.source_code, submission.source_sha256, spec.spec_checksum,
    submission.origin_cid, job.origin_cid
FROM judge_specs AS spec
JOIN submissions AS submission
  ON submission.id = spec.submission_id
 AND submission.user_id = spec.user_id
 AND submission.problem_id = spec.problem_id
 AND submission.latest_generation = spec.generation
JOIN judge_jobs AS job
  ON job.id = $1
 AND job.submission_id = spec.submission_id
 AND job.generation = spec.generation
 AND job.judge_spec_id = spec.id
WHERE spec.id = $2
-- judge_specs is append-only; the row cannot change, so no row lock is needed or permitted.
`, jobID, locked.judgeSpecID).Scan(
		&value.plan.JudgeSpecID,
		&value.plan.SubmissionID,
		&value.plan.Generation,
		&value.plan.UserID,
		&value.plan.ProblemID,
		&value.plan.ProblemRevisionID,
		&value.plan.TestsetRevisionID,
		&value.plan.Language,
		&value.plan.JudgeSpecProtocol,
		&value.plan.Runtime.Key,
		&value.plan.Runtime.ProfileVersion,
		&value.plan.Runtime.CompilerVersion,
		&value.plan.Runtime.BootstrapVersion,
		&value.plan.Runtime.SandboxImageDigest,
		&value.plan.Runtime.CompileCommandID,
		&value.plan.Runtime.RunCommandID,
		&value.plan.Runtime.ArtifactManifestVersion,
		&value.plan.Runtime.ArtifactManifestChecksum,
		&value.artifactManifest,
		&value.plan.Runtime.SupervisorProtocol,
		&value.plan.Release.ID,
		&value.plan.Release.ApprovedManifestChecksum,
		&value.plan.Release.CheckerImplementationChecksum,
		&value.plan.Runtime.CapabilityProfileHash,
		&value.plan.Checker.Key,
		&value.plan.Checker.Version,
		&value.checkerConfig,
		&value.rewardSnapshot,
		&value.rewardDigest,
		&value.plan.Limits.CPUTimeMilliseconds,
		&value.plan.Limits.WallTimeMilliseconds,
		&value.plan.Limits.MemoryKilobytes,
		&value.plan.Limits.OutputBytes,
		&value.plan.Limits.ProcessCount,
		&value.plan.Limits.CompileCPUTimeMilliseconds,
		&value.plan.Limits.CompileWallTimeMilliseconds,
		&value.plan.Limits.TotalAttemptWallMilliseconds,
		&value.plan.SourceCode,
		&value.sourceDigest,
		&value.specDigest,
		&value.submissionCID,
		&value.jobCID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return persistedJudgePlan{}, ports.ErrJudgePlanIntegrity
	}
	if err != nil {
		return persistedJudgePlan{}, fmt.Errorf("read immutable judge plan: %w", err)
	}
	value.plan.JobID = jobID
	value.plan.Runtime.ArtifactManifest = append(
		json.RawMessage(nil),
		value.artifactManifest...,
	)
	value.plan.Checker.ImplementationChecksum = value.plan.Release.CheckerImplementationChecksum
	value.plan.Checker.Config = append(json.RawMessage(nil), value.checkerConfig...)
	value.plan.RewardFacts = append(json.RawMessage(nil), value.rewardSnapshot...)
	value.plan.OriginCID = value.jobCID
	return value, nil
}

func verifyPersistedJudgePlan(
	value *persistedJudgePlan,
	lease ports.JobLease,
) error {
	if value == nil {
		return ports.ErrJudgePlanIntegrity
	}
	sourceDigest, ok := fixedDigest(value.sourceDigest)
	if !ok || sha256.Sum256([]byte(value.plan.SourceCode)) != sourceDigest ||
		judging.ValidateSourceCode(value.plan.SourceCode) != nil {
		return ports.ErrJudgePlanIntegrity
	}
	specDigest, ok := fixedDigest(value.specDigest)
	if !ok {
		return ports.ErrJudgePlanIntegrity
	}
	rewardDigest, ok := fixedDigest(value.rewardDigest)
	if !ok {
		return ports.ErrJudgePlanIntegrity
	}
	value.plan.SourceSHA256 = sourceDigest
	value.plan.SpecChecksum = specDigest
	value.plan.RewardChecksum = rewardDigest
	computed, err := submissioncontract.ComputeJudgeSpecChecksum(
		submissioncontract.JudgeSpecChecksumInput{
			SubmissionID:                    value.plan.SubmissionID,
			UserID:                          value.plan.UserID,
			ProblemID:                       value.plan.ProblemID,
			Generation:                      value.plan.Generation,
			ProblemRevisionID:               value.plan.ProblemRevisionID,
			TestsetRevisionID:               value.plan.TestsetRevisionID,
			Language:                        value.plan.Language,
			RuntimeKey:                      value.plan.Runtime.Key,
			RuntimeProfileVersion:           value.plan.Runtime.ProfileVersion,
			CompilerVersion:                 value.plan.Runtime.CompilerVersion,
			RuntimeBootstrapVersion:         value.plan.Runtime.BootstrapVersion,
			SandboxImageDigest:              value.plan.Runtime.SandboxImageDigest,
			CompileCommandID:                value.plan.Runtime.CompileCommandID,
			RunCommandID:                    value.plan.Runtime.RunCommandID,
			ArtifactManifestVersion:         value.plan.Runtime.ArtifactManifestVersion,
			ArtifactManifestChecksum:        value.plan.Runtime.ArtifactManifestChecksum,
			ArtifactManifest:                value.plan.Runtime.ArtifactManifest,
			SupervisorProtocol:              value.plan.Runtime.SupervisorProtocol,
			CheckerKey:                      value.plan.Checker.Key,
			CheckerVersion:                  value.plan.Checker.Version,
			CheckerConfig:                   value.plan.Checker.Config,
			ReleaseID:                       value.plan.Release.ID,
			ApprovedReleaseManifestChecksum: value.plan.Release.ApprovedManifestChecksum,
			CheckerImplementationChecksum:   value.plan.Release.CheckerImplementationChecksum,
			CapabilityProfileHash:           value.plan.Runtime.CapabilityProfileHash,
			RewardSnapshot:                  value.plan.RewardFacts,
			RewardSnapshotChecksum:          rewardDigest,
			CPUTimeMilliseconds:             value.plan.Limits.CPUTimeMilliseconds,
			WallTimeMilliseconds:            value.plan.Limits.WallTimeMilliseconds,
			MemoryKilobytes:                 value.plan.Limits.MemoryKilobytes,
			OutputBytes:                     value.plan.Limits.OutputBytes,
			ProcessCount:                    value.plan.Limits.ProcessCount,
			CompileCPUMilliseconds:          value.plan.Limits.CompileCPUTimeMilliseconds,
			CompileWallMilliseconds:         value.plan.Limits.CompileWallTimeMilliseconds,
			TotalWallMilliseconds:           value.plan.Limits.TotalAttemptWallMilliseconds,
			SourceSHA256:                    sourceDigest,
			JudgeSpecProtocol:               value.plan.JudgeSpecProtocol,
		},
	)
	if err != nil || computed != specDigest ||
		value.submissionCID != value.jobCID ||
		value.plan.OriginCID != lease.Job.OriginCID ||
		!judgePlanMatchesLease(value.plan, lease) {
		return ports.ErrJudgePlanIntegrity
	}
	return nil
}

func judgePlanMatchesLease(plan ports.JudgePlan, lease ports.JobLease) bool {
	spec := lease.JudgeSpec
	return plan.JobID == lease.Job.ID &&
		plan.SubmissionID == lease.Job.SubmissionID &&
		plan.JudgeSpecID == lease.Job.JudgeSpecID &&
		plan.Generation == lease.Job.Generation &&
		plan.JudgeSpecID == spec.ID &&
		plan.SubmissionID == spec.SubmissionID &&
		plan.Generation == spec.Generation &&
		plan.UserID == spec.UserID &&
		plan.ProblemRevisionID == spec.ProblemRevisionID &&
		plan.TestsetRevisionID == spec.TestsetRevisionID &&
		constantTimeTextEqual(plan.JudgeSpecProtocol, spec.JudgeSpecProtocol) &&
		constantTimeTextEqual(plan.Runtime.Key, spec.RuntimeKey) &&
		constantTimeTextEqual(plan.Runtime.ProfileVersion, spec.RuntimeProfileVersion) &&
		constantTimeTextEqual(plan.Runtime.CompilerVersion, spec.CompilerVersion) &&
		constantTimeTextEqual(plan.Runtime.BootstrapVersion, spec.RuntimeBootstrapVersion) &&
		constantTimeTextEqual(plan.Runtime.CompileCommandID, spec.CompileCommandID) &&
		constantTimeTextEqual(plan.Runtime.RunCommandID, spec.RunCommandID) &&
		constantTimeTextEqual(plan.Runtime.SandboxImageDigest, spec.SandboxImageDigest) &&
		constantTimeTextEqual(
			plan.Runtime.ArtifactManifestVersion,
			spec.ArtifactManifestVersion,
		) &&
		constantTimeTextEqual(
			plan.Runtime.ArtifactManifestChecksum,
			spec.ArtifactManifestChecksum,
		) &&
		constantTimeTextEqual(plan.Runtime.SupervisorProtocol, spec.SupervisorProtocol) &&
		constantTimeTextEqual(plan.Release.ID, spec.ReleaseID) &&
		constantTimeTextEqual(plan.Release.ApprovedManifestChecksum, spec.ApprovedManifestChecksum) &&
		constantTimeTextEqual(plan.Release.CheckerImplementationChecksum, spec.CheckerImplementationChecksum) &&
		constantTimeTextEqual(
			plan.Runtime.CapabilityProfileHash,
			spec.CapabilityProfileHash,
		) &&
		constantTimeTextEqual(plan.Checker.Key, spec.CheckerKey) &&
		constantTimeTextEqual(plan.Checker.Version, spec.CheckerVersion) &&
		constantTimeTextEqual(plan.Checker.ImplementationChecksum, spec.CheckerImplementationChecksum) &&
		digestBytesEqual(plan.SpecChecksum, spec.SpecChecksum) &&
		digestBytesEqual(plan.RewardChecksum, spec.RewardSnapshotChecksum)
}

func readJudgeTestsetPlan(
	ctx context.Context,
	tx *sql.Tx,
	testsetID uuid.UUID,
	problemID uuid.UUID,
) (problementity.TestsetRevision, []ports.JudgeTestSpec, error) {
	var (
		testset   problementity.TestsetRevision
		checksum  []byte
		createdBy uuid.NullUUID
	)
	testset.ID = testsetID
	err := tx.QueryRowContext(ctx, `
SELECT problem_id, revision_number, checksum, test_count,
       total_uncompressed_bytes, created_by, created_at
FROM testset_revisions
WHERE id = $1 AND problem_id = $2
-- testset_revisions is append-only; the row cannot change, so no row lock is needed or permitted.
`, testsetID, problemID).Scan(
		&testset.ProblemID,
		&testset.RevisionNumber,
		&checksum,
		&testset.TestCount,
		&testset.TotalUncompressedBytes,
		&createdBy,
		&testset.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return problementity.TestsetRevision{}, nil, ports.ErrJudgePlanIntegrity
	}
	if err != nil {
		return problementity.TestsetRevision{}, nil, fmt.Errorf(
			"read immutable judge testset: %w",
			err,
		)
	}
	if !createdBy.Valid {
		return problementity.TestsetRevision{}, nil, ports.ErrJudgePlanIntegrity
	}
	testset.CreatedBy = createdBy.UUID
	testset.Checksum, _ = fixedDigest(checksum)

	groupRows, err := tx.QueryContext(ctx, `
SELECT id, group_index, stop_on_failure, created_at
FROM testcase_groups
WHERE testset_revision_id = $1
ORDER BY group_index ASC
-- testcase_groups is append-only; rows cannot change, so no row lock is needed or permitted.
`, testsetID)
	if err != nil {
		return problementity.TestsetRevision{}, nil, fmt.Errorf(
			"read immutable judge testcase groups: %w",
			err,
		)
	}
	for groupRows.Next() {
		var group problementity.TestcaseGroup
		group.TestsetRevisionID = testsetID
		if err := groupRows.Scan(
			&group.ID,
			&group.GroupIndex,
			&group.StopOnFailure,
			&group.CreatedAt,
		); err != nil {
			_ = groupRows.Close()
			return problementity.TestsetRevision{}, nil, fmt.Errorf(
				"scan immutable judge testcase group: %w",
				err,
			)
		}
		if group.GroupIndex != len(testset.Groups) {
			_ = groupRows.Close()
			return problementity.TestsetRevision{}, nil, ports.ErrJudgePlanIntegrity
		}
		testset.Groups = append(testset.Groups, group)
	}
	if err := groupRows.Close(); err != nil {
		return problementity.TestsetRevision{}, nil, fmt.Errorf(
			"close immutable judge testcase groups: %w",
			err,
		)
	}

	tests, err := readJudgeTestReferences(ctx, tx, &testset)
	if err != nil {
		return problementity.TestsetRevision{}, nil, err
	}
	if testset.Checksum == ([32]byte{}) ||
		testset.ValidateReferenceStructure() != nil {
		return problementity.TestsetRevision{}, nil, ports.ErrJudgePlanIntegrity
	}
	return testset, tests, nil
}

func readJudgeTestReferences(
	ctx context.Context,
	tx *sql.Tx,
	testset *problementity.TestsetRevision,
) ([]ports.JudgeTestSpec, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT
    testcase.id, testcase.test_case_id, testcase.problem_id,
    testcase.testset_revision_id, testcase.group_id, testcase.kind,
    testcase.group_index, testcase.order_index, testcase.weight,
    testcase.stop_on_failure, testcase.created_at,
    input.id, input.sha256, input.encoding, input.uncompressed_size, input.created_at,
    expected.id, expected.sha256, expected.encoding,
    expected.uncompressed_size, expected.created_at
FROM testcase_revisions AS testcase
JOIN testcase_groups AS group_row
  ON group_row.id = testcase.group_id
 AND group_row.testset_revision_id = testcase.testset_revision_id
 AND group_row.group_index = testcase.group_index
 AND group_row.stop_on_failure = testcase.stop_on_failure
JOIN test_assets AS input ON input.id = testcase.input_asset_id
JOIN test_assets AS expected ON expected.id = testcase.expected_asset_id
WHERE testcase.testset_revision_id = $1
ORDER BY testcase.group_index ASC, testcase.order_index ASC
-- testcase_revisions and test_assets are append-only; rows cannot change,
-- so no row lock is needed or permitted.
`, testset.ID)
	if err != nil {
		return nil, fmt.Errorf("read immutable judge testcase references: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]ports.JudgeTestSpec, 0, testset.TestCount)
	for rows.Next() {
		var (
			test           problementity.TestcaseRevision
			groupID        uuid.UUID
			inputDigest    []byte
			expectedDigest []byte
		)
		if err := rows.Scan(
			&test.ID,
			&test.TestCaseID,
			&test.ProblemID,
			&test.TestsetRevisionID,
			&groupID,
			&test.Kind,
			&test.GroupIndex,
			&test.OrderIndex,
			&test.Weight,
			&test.StopOnFailure,
			&test.CreatedAt,
			&test.InputAsset.ID,
			&inputDigest,
			&test.InputAsset.Encoding,
			&test.InputAsset.UncompressedSize,
			&test.InputAsset.CreatedAt,
			&test.ExpectedAsset.ID,
			&expectedDigest,
			&test.ExpectedAsset.Encoding,
			&test.ExpectedAsset.UncompressedSize,
			&test.ExpectedAsset.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan immutable judge testcase reference: %w", err)
		}
		inputSHA, inputOK := fixedDigest(inputDigest)
		expectedSHA, expectedOK := fixedDigest(expectedDigest)
		if !inputOK || !expectedOK ||
			test.GroupIndex < 0 || test.GroupIndex >= len(testset.Groups) ||
			test.OrderIndex != len(testset.Groups[test.GroupIndex].Cases) ||
			groupID != testset.Groups[test.GroupIndex].ID {
			return nil, ports.ErrJudgePlanIntegrity
		}
		test.InputAsset.SHA256 = inputSHA
		test.ExpectedAsset.SHA256 = expectedSHA
		testset.Groups[test.GroupIndex].Cases = append(
			testset.Groups[test.GroupIndex].Cases,
			test,
		)
		result = append(result, ports.JudgeTestSpec{
			RevisionID:     test.ID,
			TestCaseID:     test.TestCaseID,
			GroupID:        groupID,
			GroupIndex:     test.GroupIndex,
			OrderIndex:     test.OrderIndex,
			Kind:           test.Kind,
			InputAssetID:   test.InputAsset.ID,
			OutputAssetID:  test.ExpectedAsset.ID,
			InputSHA256:    inputSHA,
			OutputSHA256:   expectedSHA,
			InputEncoding:  test.InputAsset.Encoding,
			OutputEncoding: test.ExpectedAsset.Encoding,
			InputBytes:     test.InputAsset.UncompressedSize,
			OutputBytes:    test.ExpectedAsset.UncompressedSize,
			StopOnFailure:  test.StopOnFailure,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate immutable judge testcase references: %w", err)
	}
	return result, nil
}

// ReadJudgeCase retrieves only one input/expected pair and rechecks the active
// lease in the same transaction. The caller may mount Input into a fresh
// contestant boundary; Expected remains host-side for the trusted checker.
func (repository *JobRepository) ReadJudgeCase(
	ctx context.Context,
	lease ports.JobLease,
	testcaseRevisionID uuid.UUID,
) (ports.JudgeCasePayload, error) {
	if repository == nil || repository.db == nil || ctx == nil ||
		testcaseRevisionID == uuid.Nil {
		return ports.JudgeCasePayload{}, ErrInvalidJobRepository
	}
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return ports.JudgeCasePayload{}, fmt.Errorf("begin judge case read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return ports.JudgeCasePayload{}, err
	}
	locked, err := repository.lockFencedLease(ctx, tx, lease, now)
	if err != nil {
		return ports.JudgeCasePayload{}, err
	}

	var (
		payload           ports.JudgeCasePayload
		inputDigest       []byte
		expectedDigest    []byte
		inputCreatedAt    time.Time
		expectedCreatedAt time.Time
	)
	err = tx.QueryRowContext(ctx, `
SELECT
    testcase.id,
    input.id, input.sha256, input.encoding, input.content,
    input.uncompressed_size, input.created_at,
    expected.id, expected.sha256, expected.encoding, expected.content,
    expected.uncompressed_size, expected.created_at
FROM judge_specs AS spec
JOIN testcase_revisions AS testcase
  ON testcase.id = $2
 AND testcase.testset_revision_id = spec.testset_revision_id
 AND testcase.problem_id = spec.problem_id
JOIN test_assets AS input ON input.id = testcase.input_asset_id
JOIN test_assets AS expected ON expected.id = testcase.expected_asset_id
WHERE spec.id = $1
-- judge_specs, testcase_revisions, and test_assets are all append-only; rows
-- cannot change, so no row lock is needed or permitted on any of them.
`,
		locked.judgeSpecID,
		testcaseRevisionID,
	).Scan(
		&payload.TestcaseRevisionID,
		&payload.Input.ID,
		&inputDigest,
		&payload.Input.Encoding,
		&payload.Input.Content,
		&payload.Input.UncompressedSize,
		&inputCreatedAt,
		&payload.Expected.ID,
		&expectedDigest,
		&payload.Expected.Encoding,
		&payload.Expected.Content,
		&payload.Expected.UncompressedSize,
		&expectedCreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.JudgeCasePayload{}, ports.ErrJudgeCaseIntegrity
	}
	if err != nil {
		return ports.JudgeCasePayload{}, fmt.Errorf("read immutable judge case: %w", err)
	}
	inputSHA, inputOK := fixedDigest(inputDigest)
	expectedSHA, expectedOK := fixedDigest(expectedDigest)
	payload.Input.SHA256 = inputSHA
	payload.Expected.SHA256 = expectedSHA
	if !inputOK || !expectedOK ||
		validatePersistedJudgeAsset(payload.Input, inputCreatedAt) != nil ||
		validatePersistedJudgeAsset(payload.Expected, expectedCreatedAt) != nil {
		clear(payload.Input.Content)
		clear(payload.Expected.Content)
		return ports.JudgeCasePayload{}, ports.ErrJudgeCaseIntegrity
	}
	if err := tx.Commit(); err != nil {
		clear(payload.Input.Content)
		clear(payload.Expected.Content)
		return ports.JudgeCasePayload{}, fmt.Errorf("commit judge case read: %w", err)
	}
	return payload, nil
}

func validatePersistedJudgeAsset(
	asset ports.JudgeAssetSnapshot,
	createdAt time.Time,
) error {
	return (problementity.TestAsset{
		ID:               asset.ID,
		SHA256:           asset.SHA256,
		Encoding:         asset.Encoding,
		Content:          asset.Content,
		UncompressedSize: asset.UncompressedSize,
		CreatedAt:        createdAt,
	}).ValidateMetadata()
}

func fixedDigest(value []byte) ([32]byte, bool) {
	var result [32]byte
	if len(value) != sha256.Size {
		return result, false
	}
	copy(result[:], value)
	return result, result != ([32]byte{})
}

func digestBytesEqual(expected [32]byte, actual []byte) bool {
	var normalized [32]byte
	copy(normalized[:], actual)
	return subtle.ConstantTimeEq(int32(len(actual)), sha256.Size)&
		subtle.ConstantTimeCompare(expected[:], normalized[:]) == 1
}

func cloneJudgePlan(plan ports.JudgePlan) ports.JudgePlan {
	plan.Runtime.ArtifactManifest = append(
		json.RawMessage(nil),
		plan.Runtime.ArtifactManifest...,
	)
	plan.Checker.Config = append(json.RawMessage(nil), plan.Checker.Config...)
	plan.RewardFacts = append(json.RawMessage(nil), plan.RewardFacts...)
	plan.Tests = append([]ports.JudgeTestSpec(nil), plan.Tests...)
	return plan
}

var (
	_ ports.JudgeSpecReader = (*JobRepository)(nil)
	_ ports.JudgeCaseReader = (*JobRepository)(nil)
)
