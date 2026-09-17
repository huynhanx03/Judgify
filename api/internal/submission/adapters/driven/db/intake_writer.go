package db

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/pkg/judging"
)

const (
	maximumInitialJudgeAttempts = 100

	contestRegistrationAggregateType = contestcontract.AggregateRegistration
	contestRegistrationChangedV1     = contestcontract.EventRegistrationChangedV1
	implicitRegistrationAuditReason  = "implicit open-contest registration during submission admission"
)

type DurableIntakeWriter struct {
	client      *dbent.EntClient
	outbox      eventing.OutboxAppender
	audit       audit.Appender
	maxAttempts uint32
	newID       func() (uuid.UUID, error)
}

func NewDurableIntakeWriter(
	client *dbent.EntClient,
	outbox eventing.OutboxAppender,
	auditAppender audit.Appender,
	maxAttempts uint32,
) (*DurableIntakeWriter, error) {
	if client == nil || outbox == nil || auditAppender == nil ||
		maxAttempts < 1 || maxAttempts > maximumInitialJudgeAttempts {
		return nil, intake.ErrPersistence
	}
	return &DurableIntakeWriter{
		client:      client,
		outbox:      outbox,
		audit:       auditAppender,
		maxAttempts: maxAttempts,
		newID:       uuid.NewV7,
	}, nil
}

func (writer *DurableIntakeWriter) Persist(
	ctx context.Context,
	accepted intake.AcceptedSubmission,
) (intake.Result, error) {
	if writer == nil || writer.client == nil || writer.outbox == nil ||
		writer.audit == nil || writer.newID == nil || ctx == nil ||
		!validAcceptedSubmission(accepted) {
		return intake.Result{}, intake.ErrPersistence
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return intake.Result{}, err
	}

	submissionID, err := writer.newID()
	if err != nil {
		return intake.Result{}, fmt.Errorf(
			"create durable submission identity: %w",
			err,
		)
	}
	judgeSpecID, err := writer.newID()
	if err != nil {
		return intake.Result{}, fmt.Errorf(
			"create judge specification identity: %w",
			err,
		)
	}
	jobID, err := writer.newID()
	if err != nil {
		return intake.Result{}, fmt.Errorf("create judge job identity: %w", err)
	}
	queuedEventID, err := writer.newID()
	if err != nil {
		return intake.Result{}, fmt.Errorf(
			"create submission event identity: %w",
			err,
		)
	}
	if submissionID == uuid.Nil || judgeSpecID == uuid.Nil ||
		jobID == uuid.Nil || queuedEventID == uuid.Nil {
		return intake.Result{}, intake.ErrPersistence
	}

	specChecksum, err := submissioncontract.ComputeJudgeSpecChecksum(
		judgeSpecChecksumInput(submissionID, accepted),
	)
	if err != nil {
		return intake.Result{}, errors.Join(intake.ErrPersistence, err)
	}
	client := writer.client.DB(ctx)
	if err := insertDurableSubmission(
		ctx,
		client,
		submissionID,
		accepted,
	); err != nil {
		return intake.Result{}, err
	}
	if err := incrementDurableSubmissionCounters(
		ctx,
		client,
		accepted.Command.UserID,
		accepted.Command.ProblemID,
		accepted.SubmittedAt,
	); err != nil {
		return intake.Result{}, err
	}
	if err := insertImmutableJudgeSpec(
		ctx,
		client,
		judgeSpecID,
		submissionID,
		specChecksum,
		accepted,
	); err != nil {
		return intake.Result{}, err
	}
	lane := "regular"
	if accepted.Contest != nil {
		lane = accepted.Contest.Lane
	}
	if err := insertReadyJudgeJob(
		ctx,
		client,
		jobID,
		judgeSpecID,
		submissionID,
		lane,
		writer.maxAttempts,
		accepted,
	); err != nil {
		return intake.Result{}, err
	}

	queuedPayload, err := json.Marshal(submissioncontract.SubmissionQueuedV1{
		SubmissionID: submissionID,
		Generation:   1,
		JudgeSpecID:  judgeSpecID,
		JobID:        jobID,
		Lane:         lane,
		QueuedAt:     accepted.SubmittedAt.UTC().Format(time.RFC3339Nano),
		CID:          accepted.Command.CorrelationID,
	})
	if err != nil {
		return intake.Result{}, fmt.Errorf(
			"encode submission queued event: %w",
			err,
		)
	}
	events := []eventing.Event{{
		ID:               queuedEventID,
		Type:             submissioncontract.EventSubmissionQueuedV1,
		AggregateType:    submissioncontract.AggregateSubmission,
		AggregateID:      submissionID,
		AggregateVersion: 1,
		CorrelationID:    accepted.Command.CorrelationID,
		Payload:          queuedPayload,
		OccurredAt:       accepted.SubmittedAt,
	}}
	registrationEvent, exists, err := writer.registrationEffect(
		ctx,
		accepted,
	)
	if err != nil {
		return intake.Result{}, err
	}
	if exists {
		events = append(events, registrationEvent)
	}
	if err := writer.outbox.Append(ctx, events...); err != nil {
		return intake.Result{}, fmt.Errorf(
			"append durable submission events: %w",
			err,
		)
	}

	result := intake.Result{
		SubmissionID:     submissionID,
		Generation:       1,
		Phase:            entity.PhaseQueued,
		AggregateVersion: 1,
		SubmittedAt:      accepted.SubmittedAt,
		CorrelationID:    accepted.Command.CorrelationID,
	}
	if accepted.Contest != nil {
		sequence := accepted.Contest.ContestSubmissionSequence
		result.ContestSequence = &sequence
	}
	return result, nil
}

func (writer *DurableIntakeWriter) registrationEffect(
	ctx context.Context,
	accepted intake.AcceptedSubmission,
) (eventing.Event, bool, error) {
	if accepted.Contest == nil ||
		accepted.Contest.ImplicitRegistrationTransition == nil {
		return eventing.Event{}, false, nil
	}
	transition := accepted.Contest.ImplicitRegistrationTransition
	if transition.ID == uuid.Nil ||
		transition.ContestID != accepted.Contest.ContestID ||
		transition.RegistrationID == uuid.Nil ||
		transition.UserID != accepted.Command.UserID ||
		transition.CommandID == uuid.Nil ||
		transition.CommandHash == ([32]byte{}) ||
		transition.RegistrationVersion < 1 ||
		transition.Mutation != "register" ||
		transition.ToStatus != "registered" ||
		transition.Source != "implicit_open" ||
		transition.ActorUserID == nil ||
		*transition.ActorUserID != accepted.Command.UserID ||
		transition.OccurredAt.IsZero() {
		return eventing.Event{}, false, intake.ErrPersistence
	}
	eventID, err := writer.newID()
	if err != nil {
		return eventing.Event{}, false, fmt.Errorf(
			"create implicit registration event identity: %w",
			err,
		)
	}
	if eventID == uuid.Nil {
		return eventing.Event{}, false, intake.ErrPersistence
	}
	payload, err := json.Marshal(struct {
		ContestID      uuid.UUID `json:"contest_id"`
		RegistrationID uuid.UUID `json:"registration_id"`
		UserID         uuid.UUID `json:"user_id"`
		Status         string    `json:"status"`
		Source         string    `json:"source"`
		Version        int64     `json:"version"`
		OccurredAt     string    `json:"occurred_at"`
	}{
		ContestID:      transition.ContestID,
		RegistrationID: transition.RegistrationID,
		UserID:         transition.UserID,
		Status:         string(transition.ToStatus),
		Source:         string(transition.Source),
		Version:        transition.RegistrationVersion,
		OccurredAt:     transition.OccurredAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return eventing.Event{}, false, fmt.Errorf(
			"encode implicit registration event: %w",
			err,
		)
	}
	actorID := *transition.ActorUserID
	if _, err := writer.audit.Append(ctx, audit.Entry{
		ID:             eventID,
		ActorUserID:    &actorID,
		ActorSessionID: transition.ActorSessionID,
		Action:         contestRegistrationChangedV1,
		Resource:       contestRegistrationAggregateType,
		ResourceID:     transition.RegistrationID.String(),
		Reason:         implicitRegistrationAuditReason,
		Before: map[string]any{
			"status":  transition.FromStatus,
			"version": transition.RegistrationVersion - 1,
		},
		After: map[string]any{
			"status":  transition.ToStatus,
			"source":  transition.Source,
			"version": transition.RegistrationVersion,
		},
		CorrelationID: accepted.Command.CorrelationID,
		OccurredAt:    transition.OccurredAt,
	}); err != nil {
		return eventing.Event{}, false, fmt.Errorf(
			"append implicit registration audit: %w",
			err,
		)
	}
	return eventing.Event{
		ID:               eventID,
		Type:             contestRegistrationChangedV1,
		AggregateType:    contestRegistrationAggregateType,
		AggregateID:      transition.RegistrationID,
		AggregateVersion: transition.RegistrationVersion,
		CorrelationID:    accepted.Command.CorrelationID,
		Payload:          payload,
		OccurredAt:       transition.OccurredAt,
	}, true, nil
}

func validAcceptedSubmission(accepted intake.AcceptedSubmission) bool {
	expectedRequestHash, requestErr := intake.HashCommand(accepted.Command)
	language, supportedLanguage := judging.ParseLanguage(
		accepted.Capability.Language,
	)
	expectedAttemptWallMilliseconds := int64(
		accepted.Capability.CompileWallTimeMilliseconds,
	) + int64(accepted.Problem.WallTimeMilliseconds)*int64(
		accepted.Problem.TestCount,
	)
	if accepted.Command.UserID == uuid.Nil ||
		accepted.Command.ProblemID == uuid.Nil ||
		accepted.RequestHash == ([32]byte{}) ||
		requestErr != nil ||
		expectedRequestHash != accepted.RequestHash ||
		accepted.SourceSHA256 == ([32]byte{}) ||
		sha256.Sum256(accepted.Command.Source) != accepted.SourceSHA256 ||
		accepted.SubmittedAt.IsZero() ||
		intake.ValidateIdempotencyKey(accepted.Command.IdempotencyKey) != nil ||
		correlation.Validate(accepted.Command.CorrelationID) != nil ||
		judging.ValidateSourceCode(string(accepted.Command.Source)) != nil ||
		accepted.Problem.ProblemID != accepted.Command.ProblemID ||
		accepted.Problem.ProblemRevisionID == uuid.Nil ||
		accepted.Problem.TestsetRevisionID == uuid.Nil ||
		accepted.Problem.RewardSnapshotChecksum == ([32]byte{}) ||
		!supportedLanguage ||
		string(language) != accepted.Capability.Language ||
		accepted.Capability.ReleaseID == "" ||
		accepted.Capability.ApprovedReleaseManifestChecksum == "" ||
		accepted.Capability.CheckerImplementationChecksum == "" ||
		accepted.Problem.TestCount < 1 ||
		accepted.Problem.WallTimeMilliseconds < 1 ||
		accepted.Capability.CompileWallTimeMilliseconds < 1 ||
		expectedAttemptWallMilliseconds < int64(accepted.Capability.CompileWallTimeMilliseconds) ||
		expectedAttemptWallMilliseconds > int64(24*time.Hour/time.Millisecond) ||
		int64(accepted.Capability.TotalAttemptWallMilliseconds) != expectedAttemptWallMilliseconds {
		return false
	}
	if accepted.Contest == nil {
		return accepted.Command.ContestID == nil &&
			accepted.Command.ContestProblemID == nil
	}
	return accepted.Command.ContestID != nil &&
		accepted.Command.ContestProblemID != nil &&
		accepted.Contest.ContestID == *accepted.Command.ContestID &&
		accepted.Contest.ContestProblemID == *accepted.Command.ContestProblemID &&
		accepted.Contest.ProblemID == accepted.Command.ProblemID &&
		accepted.Contest.ProblemRevisionID == accepted.Problem.ProblemRevisionID &&
		accepted.Contest.TestsetRevisionID == accepted.Problem.TestsetRevisionID &&
		accepted.Contest.ScoringPolicy.Valid() &&
		accepted.Contest.ScoringPolicyVersion >= 1 &&
		accepted.Contest.Lane == "contest" &&
		accepted.Contest.SubmittedAt.Equal(accepted.SubmittedAt) &&
		accepted.Contest.SubmissionElapsedMillis >= 0 &&
		accepted.Contest.ContestSubmissionSequence >= 1 &&
		accepted.Contest.ProblemSnapshotChecksum != ([32]byte{})
}

func judgeSpecChecksumInput(
	submissionID uuid.UUID,
	accepted intake.AcceptedSubmission,
) submissioncontract.JudgeSpecChecksumInput {
	return submissioncontract.JudgeSpecChecksumInput{
		SubmissionID:                    submissionID,
		UserID:                          accepted.Command.UserID,
		ProblemID:                       accepted.Command.ProblemID,
		Generation:                      1,
		ProblemRevisionID:               accepted.Problem.ProblemRevisionID,
		TestsetRevisionID:               accepted.Problem.TestsetRevisionID,
		Language:                        accepted.Capability.Language,
		RuntimeKey:                      accepted.Command.RuntimeKey,
		RuntimeProfileVersion:           accepted.Capability.RuntimeProfileVersion,
		CompilerVersion:                 accepted.Capability.CompilerVersion,
		RuntimeBootstrapVersion:         accepted.Capability.RuntimeBootstrapVersion,
		SandboxImageDigest:              accepted.Capability.SandboxImageDigest,
		CompileCommandID:                accepted.Capability.CompileCommandID,
		RunCommandID:                    accepted.Capability.RunCommandID,
		ArtifactManifestVersion:         accepted.Capability.ArtifactManifestVersion,
		ArtifactManifestChecksum:        accepted.Capability.ArtifactManifestChecksum,
		ArtifactManifest:                accepted.Capability.ArtifactManifest,
		SupervisorProtocol:              accepted.Capability.SupervisorProtocolVersion,
		CheckerKey:                      accepted.Problem.CheckerKey,
		CheckerVersion:                  accepted.Problem.CheckerVersion,
		CheckerConfig:                   accepted.Problem.CheckerConfig,
		ReleaseID:                       accepted.Capability.ReleaseID,
		ApprovedReleaseManifestChecksum: accepted.Capability.ApprovedReleaseManifestChecksum,
		CheckerImplementationChecksum:   accepted.Capability.CheckerImplementationChecksum,
		CapabilityProfileHash:           accepted.Capability.CapabilityProfileHash,
		RewardSnapshot:                  accepted.Problem.RewardSnapshot,
		RewardSnapshotChecksum:          accepted.Problem.RewardSnapshotChecksum,
		CPUTimeMilliseconds:             accepted.Problem.CPUTimeMilliseconds,
		WallTimeMilliseconds:            accepted.Problem.WallTimeMilliseconds,
		MemoryKilobytes:                 accepted.Problem.MemoryKilobytes,
		OutputBytes:                     accepted.Problem.OutputBytes,
		ProcessCount:                    accepted.Problem.ProcessCount,
		CompileCPUMilliseconds:          accepted.Capability.CompileCPUTimeMilliseconds,
		CompileWallMilliseconds:         accepted.Capability.CompileWallTimeMilliseconds,
		TotalWallMilliseconds:           accepted.Capability.TotalAttemptWallMilliseconds,
		SourceSHA256:                    accepted.SourceSHA256,
		JudgeSpecProtocol:               accepted.Capability.JudgeSpecProtocol,
	}
}

func insertDurableSubmission(
	ctx context.Context,
	client *generate.Client,
	submissionID uuid.UUID,
	accepted intake.AcceptedSubmission,
) error {
	var contestID *uuid.UUID
	var contestProblemID *uuid.UUID
	var scoringPolicyVersion *int64
	var contestElapsed *int64
	var contestSequence *int64
	if accepted.Contest != nil {
		contestID = &accepted.Contest.ContestID
		contestProblemID = &accepted.Contest.ContestProblemID
		scoringPolicyVersion = &accepted.Contest.ScoringPolicyVersion
		contestElapsed = &accepted.Contest.SubmissionElapsedMillis
		contestSequence = &accepted.Contest.ContestSubmissionSequence
	}
	result, err := client.Submission.ExecContext(ctx, `
INSERT INTO submissions
    (id, problem_id, user_id, contest_id, contest_problem_id,
     contest_scoring_policy_version, submitted_at, contest_elapsed_ms,
     contest_submission_sequence, language, runtime_key, source_code,
     source_sha256, version, latest_generation, latest_generation_phase,
     idempotency_key, request_hash, origin_cid, latest_generation_queued_at,
     status, created_at, updated_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
     1, 1, 'queued', $14, $15, $16, $17, 'pending', $18, $18)`,
		submissionID,
		accepted.Command.ProblemID,
		accepted.Command.UserID,
		contestID,
		contestProblemID,
		scoringPolicyVersion,
		accepted.SubmittedAt,
		contestElapsed,
		contestSequence,
		accepted.Capability.Language,
		accepted.Command.RuntimeKey,
		string(accepted.Command.Source),
		accepted.SourceSHA256[:],
		accepted.Command.IdempotencyKey,
		accepted.RequestHash[:],
		accepted.Command.CorrelationID,
		accepted.SubmittedAt,
		accepted.SubmittedAt,
	)
	if err != nil {
		return fmt.Errorf("insert durable submission: %w", err)
	}
	if err := requireOneRow(result); err != nil {
		return fmt.Errorf("insert durable submission: %w", err)
	}
	return nil
}

// incrementDurableSubmissionCounters counts immutable submission aggregates,
// not judge generations. It runs in the same intake transaction as the
// idempotency-protected insert, so retries and administrative rejudges cannot
// inflate either projection.
func incrementDurableSubmissionCounters(
	ctx context.Context,
	client *generate.Client,
	userID uuid.UUID,
	problemID uuid.UUID,
	submittedAt time.Time,
) error {
	if client == nil || userID == uuid.Nil || problemID == uuid.Nil ||
		submittedAt.IsZero() {
		return intake.ErrPersistence
	}
	userResult, err := client.Submission.ExecContext(ctx, `
UPDATE user_stats
SET total_submissions = total_submissions + 1,
    updated_at = $2
WHERE user_id = $1
  AND deleted_at IS NULL`,
		userID,
		submittedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("increment durable user submission count: %w", err)
	}
	if err := requireOneRow(userResult); err != nil {
		return fmt.Errorf(
			"increment durable user submission count: %w",
			err,
		)
	}
	problemResult, err := client.Submission.ExecContext(ctx, `
UPDATE problems
SET submission_count = submission_count + 1,
    updated_at = $2
WHERE id = $1
  AND deleted_at IS NULL`,
		problemID,
		submittedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf(
			"increment durable problem submission count: %w",
			err,
		)
	}
	if err := requireOneRow(problemResult); err != nil {
		return fmt.Errorf(
			"increment durable problem submission count: %w",
			err,
		)
	}
	return nil
}

func insertImmutableJudgeSpec(
	ctx context.Context,
	client *generate.Client,
	judgeSpecID uuid.UUID,
	submissionID uuid.UUID,
	specChecksum [32]byte,
	accepted intake.AcceptedSubmission,
) error {
	result, err := client.Submission.ExecContext(ctx, `
INSERT INTO judge_specs
    (id, submission_id, user_id, problem_id, generation,
     problem_revision_id, testset_revision_id, language, runtime_key,
     runtime_profile_version, compiler_version, runtime_bootstrap_version,
     sandbox_image_digest, compile_command_id, run_command_id,
     artifact_manifest_version, artifact_manifest_checksum, artifact_manifest,
     supervisor_protocol_version, checker_key, checker_version, checker_config,
     release_id, approved_release_manifest_checksum, checker_implementation_checksum,
     capability_profile_hash, reward_snapshot, reward_snapshot_checksum,
     cpu_time_limit_ms, wall_time_limit_ms, memory_limit_kb,
     output_limit_bytes, process_limit, compile_cpu_time_limit_ms,
     compile_wall_time_limit_ms, total_attempt_wall_time_limit_ms,
     source_sha256, judge_spec_protocol, spec_checksum, created_at)
VALUES
    ($1, $2, $3, $4, 1, $5, $6, $7, $8, $9, $10, $11, $12,
     $13, $14, $15, $16, $17::jsonb, $18, $19, $20, $21::jsonb,
     $22, $23, $24, $25, $26::jsonb, $27, $28, $29, $30, $31, $32,
     $33, $34, $35, $36, $37, $38, $39)`,
		judgeSpecID,
		submissionID,
		accepted.Command.UserID,
		accepted.Command.ProblemID,
		accepted.Problem.ProblemRevisionID,
		accepted.Problem.TestsetRevisionID,
		accepted.Capability.Language,
		accepted.Command.RuntimeKey,
		accepted.Capability.RuntimeProfileVersion,
		accepted.Capability.CompilerVersion,
		accepted.Capability.RuntimeBootstrapVersion,
		accepted.Capability.SandboxImageDigest,
		accepted.Capability.CompileCommandID,
		accepted.Capability.RunCommandID,
		accepted.Capability.ArtifactManifestVersion,
		accepted.Capability.ArtifactManifestChecksum,
		accepted.Capability.ArtifactManifest,
		accepted.Capability.SupervisorProtocolVersion,
		accepted.Problem.CheckerKey,
		accepted.Problem.CheckerVersion,
		accepted.Problem.CheckerConfig,
		accepted.Capability.ReleaseID,
		accepted.Capability.ApprovedReleaseManifestChecksum,
		accepted.Capability.CheckerImplementationChecksum,
		accepted.Capability.CapabilityProfileHash,
		accepted.Problem.RewardSnapshot,
		accepted.Problem.RewardSnapshotChecksum[:],
		accepted.Problem.CPUTimeMilliseconds,
		accepted.Problem.WallTimeMilliseconds,
		accepted.Problem.MemoryKilobytes,
		accepted.Problem.OutputBytes,
		accepted.Problem.ProcessCount,
		accepted.Capability.CompileCPUTimeMilliseconds,
		accepted.Capability.CompileWallTimeMilliseconds,
		accepted.Capability.TotalAttemptWallMilliseconds,
		accepted.SourceSHA256[:],
		accepted.Capability.JudgeSpecProtocol,
		specChecksum[:],
		accepted.SubmittedAt,
	)
	if err != nil {
		return fmt.Errorf("insert immutable judge specification: %w", err)
	}
	if err := requireOneRow(result); err != nil {
		return fmt.Errorf("insert immutable judge specification: %w", err)
	}
	return nil
}

func insertReadyJudgeJob(
	ctx context.Context,
	client *generate.Client,
	jobID uuid.UUID,
	judgeSpecID uuid.UUID,
	submissionID uuid.UUID,
	lane string,
	maxAttempts uint32,
	accepted intake.AcceptedSubmission,
) error {
	if lane != "regular" && lane != "contest" && lane != "maintenance" {
		return intake.ErrPersistence
	}
	result, err := client.Submission.ExecContext(ctx, `
INSERT INTO judge_jobs
    (id, submission_id, generation, judge_spec_id, lane, base_priority,
     state, available_at, attempts, max_attempts, origin_cid,
     created_at, updated_at)
VALUES
    ($1, $2, 1, $3, $4, 0, 'ready', $5, 0, $6, $7, $5, $5)`,
		jobID,
		submissionID,
		judgeSpecID,
		lane,
		accepted.SubmittedAt,
		int64(maxAttempts),
		accepted.Command.CorrelationID,
	)
	if err != nil {
		return fmt.Errorf("insert ready judge job: %w", err)
	}
	if err := requireOneRow(result); err != nil {
		return fmt.Errorf("insert ready judge job: %w", err)
	}
	return nil
}

func requireOneRow(result interface {
	RowsAffected() (int64, error)
}) error {
	if result == nil {
		return intake.ErrPersistence
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return intake.ErrPersistence
	}
	return nil
}

var _ intake.Writer = (*DurableIntakeWriter)(nil)
