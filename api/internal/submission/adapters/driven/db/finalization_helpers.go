package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"time"

	"github.com/google/uuid"

	contestentity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
	submissionservice "github.com/huynhanx03/judgify/internal/submission/usecase"
)

type expectedTestcaseResult struct {
	testcaseRevisionID uuid.UUID
	groupID            uuid.UUID
	groupIndex         int
	caseOrder          int
	kind               string
	stopOnFailure      bool
}

func validatePersistedFinalizationResults(
	ctx context.Context,
	tx *sql.Tx,
	locked lockedFinalization,
	judgement entity.Judgement,
	results []ports.JudgeTestResult,
) error {
	if tx == nil {
		return ports.ErrInvalidFinalization
	}
	if judgement.Verdict == entity.VerdictCompileError {
		if len(results) != 0 || judgement.PassedCount != 0 {
			return ports.ErrInvalidFinalization
		}
		return nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT testcase.id, testcase.group_id, testcase.group_index,
			testcase.order_index, testcase.kind, testcase.stop_on_failure
		FROM judge_specs AS spec
		JOIN testcase_revisions AS testcase
		  ON testcase.testset_revision_id = spec.testset_revision_id
		 AND testcase.problem_id = spec.problem_id
		WHERE spec.id = $1
		ORDER BY testcase.group_index ASC, testcase.order_index ASC
		-- judge_specs and testcase_revisions are append-only; rows can never be
		-- updated or deleted, so no row lock is needed or permitted.
		`, locked.judgeSpecID)
	if err != nil {
		return fmt.Errorf("read immutable testcase plan for finalization: %w", err)
	}
	defer func() { _ = rows.Close() }()
	expected := make([]expectedTestcaseResult, 0, len(results))
	for rows.Next() {
		var value expectedTestcaseResult
		if err := rows.Scan(
			&value.testcaseRevisionID,
			&value.groupID,
			&value.groupIndex,
			&value.caseOrder,
			&value.kind,
			&value.stopOnFailure,
		); err != nil {
			return fmt.Errorf("scan immutable testcase plan for finalization: %w", err)
		}
		expected = append(expected, value)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate immutable testcase plan for finalization: %w", err)
	}
	if len(expected) == 0 || len(expected) != judgement.TotalCount {
		return ports.ErrInvalidFinalization
	}
	if err := validatePersistedResultShape(locked, judgement, results); err != nil {
		return err
	}
	return validateImmutableResultPlan(expected, results)
}

// validatePersistedResultShape duplicates the safety boundary that normally
// lives in the application service. Repositories are a trust boundary too:
// background workers and future call sites must not be able to persist a
// result merely because they bypassed the service validator.
func validatePersistedResultShape(
	locked lockedFinalization,
	judgement entity.Judgement,
	results []ports.JudgeTestResult,
) error {
	if len(results) == 0 || len(results) > problemcontract.MaximumTestCases {
		return ports.ErrInvalidFinalization
	}
	accepted := 0
	hasTerminalVerdict := false
	for _, result := range results {
		if result.JudgeSpecID != locked.judgeSpecID ||
			result.TestsetRevisionID != locked.testsetRevisionID ||
			result.TestcaseRevisionID == uuid.Nil || result.GroupID == uuid.Nil ||
			result.GroupIndex < 0 || result.GroupIndex >= problemcontract.MaximumTestcaseGroups ||
			result.CaseOrder < 0 || result.CaseOrder >= problemcontract.MaximumTestCases ||
			result.ExecutionOrder < 0 || result.ExecutionOrder >= problemcontract.MaximumTestCases ||
			result.TimeMilliseconds < 0 || result.TimeMilliseconds > problemcontract.MaximumWallTimeLimitMilliseconds ||
			result.MemoryKilobytes < 0 || result.MemoryKilobytes > problemcontract.MaximumMemoryLimitKilobytes ||
			result.StdoutBytes < 0 || result.StdoutBytes > problemcontract.MaximumOutputLimitBytes ||
			result.StderrBytes < 0 || result.StderrBytes > problemcontract.MaximumOutputLimitBytes ||
			result.ExecutionStartedAt.IsZero() || result.ExecutionEndedAt.Before(result.ExecutionStartedAt) ||
			result.ExitCode != nil && result.TerminatingSignal != nil ||
			result.TerminatingSignal != nil && *result.TerminatingSignal < 1 ||
			!validPersistedTestVerdict(result.Verdict) || !validJSONObject(result.CheckerMetadata) ||
			result.SafeDiagnostic != nil && (result.Kind != problemcontract.TestKindSample || len(*result.SafeDiagnostic) > 16<<10) {
			return ports.ErrInvalidFinalization
		}
		switch result.Verdict {
		case entity.VerdictAccepted:
			accepted++
		case judgement.Verdict:
			hasTerminalVerdict = true
		}
	}
	if accepted != judgement.PassedCount {
		return ports.ErrInvalidFinalization
	}
	if judgement.Verdict == entity.VerdictAccepted {
		if len(results) != judgement.TotalCount || accepted != judgement.TotalCount {
			return ports.ErrInvalidFinalization
		}
		return nil
	}
	if !hasTerminalVerdict {
		return ports.ErrInvalidFinalization
	}
	return nil
}

func validPersistedTestVerdict(verdict entity.Verdict) bool {
	switch verdict {
	case entity.VerdictAccepted, entity.VerdictWrongAnswer,
		entity.VerdictRuntimeError, entity.VerdictTimeLimitExceeded,
		entity.VerdictMemoryLimitExceeded, entity.VerdictProcessLimitExceeded,
		entity.VerdictOutputLimitExceeded:
		return true
	default:
		return false
	}
}

func validJSONObject(value json.RawMessage) bool {
	if len(value) == 0 || len(value) > problemcontract.MaximumCheckerConfigBytes {
		return false
	}
	var decoded map[string]json.RawMessage
	return json.Unmarshal(value, &decoded) == nil && decoded != nil
}

// validateImmutableResultPlan binds every persisted result to the immutable
// testcase tuple and permits an omission only after a non-accepted result in
// that same stop-on-failure group. Execution order is attempt-global.
func validateImmutableResultPlan(
	expected []expectedTestcaseResult,
	results []ports.JudgeTestResult,
) error {
	if len(expected) == 0 || len(results) == 0 || len(results) > len(expected) {
		return ports.ErrInvalidFinalization
	}
	ordered := append([]ports.JudgeTestResult(nil), results...)
	sort.SliceStable(ordered, func(left, right int) bool {
		return ordered[left].ExecutionOrder < ordered[right].ExecutionOrder
	})
	cursor := 0
	for index, result := range ordered {
		if result.ExecutionOrder != index || cursor >= len(expected) {
			return ports.ErrInvalidFinalization
		}
		expectedCase := expected[cursor]
		if result.TestcaseRevisionID != expectedCase.testcaseRevisionID ||
			result.GroupID != expectedCase.groupID ||
			result.GroupIndex != expectedCase.groupIndex ||
			result.CaseOrder != expectedCase.caseOrder ||
			result.Kind != expectedCase.kind ||
			result.StopOnFailure != expectedCase.stopOnFailure {
			return ports.ErrInvalidFinalization
		}
		cursor++
		if result.Verdict == entity.VerdictAccepted {
			continue
		}
		if !expectedCase.stopOnFailure {
			continue
		}
		for cursor < len(expected) &&
			expected[cursor].groupIndex == expectedCase.groupIndex {
			cursor++
		}
	}
	if cursor != len(expected) {
		return ports.ErrInvalidFinalization
	}
	return nil
}

func (repository *FinalizationRepository) readIdempotentFinalization(
	ctx context.Context,
	tx *sql.Tx,
	locked lockedFinalization,
	lease ports.JobLease,
	request entity.Judgement,
	results []ports.JudgeTestResult,
	system bool,
) (ports.FinalizationResult, error) {
	if tx == nil || request.ID == uuid.Nil ||
		locked.submissionID != lease.Job.SubmissionID {
		return ports.FinalizationResult{}, ports.ErrFinalizationConflict
	}
	var (
		judgementID uuid.UUID
		verdict     string
		completedAt sql.NullTime
		passed      int
		total       int
	)
	err := tx.QueryRowContext(ctx, `
		SELECT id, verdict, completed_at, passed_count, total_count
		FROM submission_judgements
		WHERE submission_id = $1 AND generation = $2
		-- submission_judgements is append-only; the row cannot change, so no
		-- row lock is needed or permitted.
		`, locked.submissionID, locked.generation).
		Scan(&judgementID, &verdict, &completedAt, &passed, &total)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.FinalizationResult{}, ports.ErrFinalizationConflict
	}
	if err != nil {
		return ports.FinalizationResult{}, fmt.Errorf("read finalization replay: %w", err)
	}
	if judgementID != request.ID || verdict != string(request.Verdict) ||
		passed != request.PassedCount || total != request.TotalCount ||
		!completedAt.Valid || (system && verdict != string(entity.VerdictSystemError)) ||
		(!system && !request.Verdict.Contestant()) {
		return ports.FinalizationResult{}, ports.ErrFinalizationConflict
	}
	if err := validateReplayTestResults(ctx, tx, lease.Attempt.ID, results); err != nil {
		return ports.FinalizationResult{}, ports.ErrFinalizationConflict
	}
	state := locked.submissionState()
	if state.LatestPhase != entity.PhaseTerminal ||
		state.LatestOutcome != verdict {
		return ports.FinalizationResult{}, ports.ErrFinalizationConflict
	}
	return ports.FinalizationResult{
		SubmissionID:         locked.submissionID,
		JudgementID:          judgementID,
		AggregateVersion:     locked.submissionVersion,
		ActiveVerdictChanged: !system,
		FinalizedAt:          completedAt.Time.UTC(),
		IdempotentReplay:     true,
		State:                state,
	}, nil
}

// validateReplayTestResults prevents a completed job from accepting a second,
// different worker payload under the same judgement ID. PostgreSQL timestamps
// are stored at microsecond precision, so comparisons use that persisted
// precision rather than rejecting an otherwise identical retried request.
func validateReplayTestResults(
	ctx context.Context,
	tx *sql.Tx,
	attemptID uuid.UUID,
	expected []ports.JudgeTestResult,
) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT judge_spec_id, testset_revision_id, testcase_revision_id, group_id,
			group_index, case_order, execution_order, test_kind,
			group_stop_on_failure, verdict, time_ms, memory_kb, exit_code,
			terminating_signal, stdout_bytes, stderr_bytes, safe_diagnostic,
			checker_metadata, execution_started_at, execution_completed_at
		FROM judge_test_results
		WHERE attempt_id = $1
		ORDER BY execution_order ASC`, attemptID)
	if err != nil {
		return fmt.Errorf("read finalization replay results: %w", err)
	}
	defer func() { _ = rows.Close() }()
	ordered := append([]ports.JudgeTestResult(nil), expected...)
	sort.SliceStable(ordered, func(left, right int) bool {
		return ordered[left].ExecutionOrder < ordered[right].ExecutionOrder
	})
	index := 0
	for rows.Next() {
		if index >= len(ordered) {
			return ports.ErrFinalizationConflict
		}
		var (
			actual           ports.JudgeTestResult
			verdict          string
			exitCode, signal sql.NullInt64
			diagnostic       sql.NullString
		)
		if err := rows.Scan(
			&actual.JudgeSpecID, &actual.TestsetRevisionID, &actual.TestcaseRevisionID,
			&actual.GroupID, &actual.GroupIndex, &actual.CaseOrder,
			&actual.ExecutionOrder, &actual.Kind, &actual.StopOnFailure, &verdict,
			&actual.TimeMilliseconds, &actual.MemoryKilobytes, &exitCode, &signal,
			&actual.StdoutBytes, &actual.StderrBytes, &diagnostic, &actual.CheckerMetadata,
			&actual.ExecutionStartedAt, &actual.ExecutionEndedAt,
		); err != nil {
			return fmt.Errorf("scan finalization replay result: %w", err)
		}
		actual.Verdict = entity.Verdict(verdict)
		if exitCode.Valid {
			value := int(exitCode.Int64)
			actual.ExitCode = &value
		}
		if signal.Valid {
			value := int(signal.Int64)
			actual.TerminatingSignal = &value
		}
		if diagnostic.Valid {
			value := diagnostic.String
			actual.SafeDiagnostic = &value
		}
		if !equalReplayTestResult(actual, ordered[index]) {
			return ports.ErrFinalizationConflict
		}
		index++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate finalization replay results: %w", err)
	}
	if index != len(ordered) {
		return ports.ErrFinalizationConflict
	}
	return nil
}

func equalReplayTestResult(left, right ports.JudgeTestResult) bool {
	return left.JudgeSpecID == right.JudgeSpecID &&
		left.TestsetRevisionID == right.TestsetRevisionID &&
		left.TestcaseRevisionID == right.TestcaseRevisionID &&
		left.GroupID == right.GroupID && left.GroupIndex == right.GroupIndex &&
		left.CaseOrder == right.CaseOrder && left.ExecutionOrder == right.ExecutionOrder &&
		left.Kind == right.Kind && left.StopOnFailure == right.StopOnFailure &&
		left.Verdict == right.Verdict && left.TimeMilliseconds == right.TimeMilliseconds &&
		left.MemoryKilobytes == right.MemoryKilobytes && equalIntPointer(left.ExitCode, right.ExitCode) &&
		equalIntPointer(left.TerminatingSignal, right.TerminatingSignal) &&
		left.StdoutBytes == right.StdoutBytes && left.StderrBytes == right.StderrBytes &&
		equalStringPointer(left.SafeDiagnostic, right.SafeDiagnostic) &&
		jsonEqual(left.CheckerMetadata, right.CheckerMetadata) &&
		left.ExecutionStartedAt.UTC().Truncate(time.Microsecond).Equal(right.ExecutionStartedAt.UTC().Truncate(time.Microsecond)) &&
		left.ExecutionEndedAt.UTC().Truncate(time.Microsecond).Equal(right.ExecutionEndedAt.UTC().Truncate(time.Microsecond))
}

func equalIntPointer(left, right *int) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func equalStringPointer(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func jsonEqual(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	return json.Unmarshal(left, &leftValue) == nil && json.Unmarshal(right, &rightValue) == nil &&
		reflect.DeepEqual(leftValue, rightValue)
}

func updateContestantSubmission(
	ctx context.Context,
	tx *sql.Tx,
	locked lockedFinalization,
	judgement entity.Judgement,
	next entity.SubmissionState,
	now time.Time,
) error {
	now = now.UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE submissions
		SET latest_generation_phase = 'terminal',
			latest_generation_outcome = $4,
			active_judgement_id = $5,
			active_judgement_generation = $6,
			active_verdict = $7,
			latest_generation_completed_at = $8,
			status = $9, passed_count = $10, total_count = $11,
			time_ms = $12, memory_kb = $13, error_message = $14,
			judged_at = $8, verdict_version = verdict_version + 1,
			version = version + 1, updated_at = $8
		WHERE id = $1 AND latest_generation = $2 AND version = $3
			AND latest_generation_phase = 'finalizing'
			AND latest_generation_outcome IS NULL`,
		locked.submissionID, locked.generation, locked.submissionVersion,
		string(judgement.Verdict), judgement.ID, judgement.Generation,
		string(judgement.Verdict), now, submissionStatusForVerdict(judgement.Verdict),
		judgement.PassedCount, judgement.TotalCount,
		judgement.MaximumTimeMilliseconds, judgement.MaximumMemoryKilobytes,
		judgement.CompileDiagnosticSummary,
	)
	if err != nil {
		return fmt.Errorf("advance contestant submission finalization: %w", err)
	}
	affected, affectErr := result.RowsAffected()
	if affectErr != nil || affected != 1 || next.AggregateVersion != locked.submissionVersion+1 {
		return ports.ErrFinalizationConflict
	}
	return nil
}

// adjustAcceptedCounters applies only the active-verdict delta. A submission
// aggregate is counted once during durable intake; rejudging it therefore
// changes accepted projections only when the active verdict crosses the
// accepted boundary.
func adjustAcceptedCounters(
	ctx context.Context,
	tx *sql.Tx,
	locked lockedFinalization,
	nextVerdict entity.Verdict,
	now time.Time,
) error {
	if tx == nil || locked.userID == uuid.Nil || locked.problemID == uuid.Nil ||
		now.IsZero() || !nextVerdict.Contestant() {
		return ports.ErrInvalidFinalization
	}
	previousAccepted := locked.activeVerdict.Valid &&
		locked.activeVerdict.String == string(entity.VerdictAccepted)
	nextAccepted := nextVerdict == entity.VerdictAccepted
	if previousAccepted == nextAccepted {
		return nil
	}
	delta := -1
	if nextAccepted {
		delta = 1
	}
	now = now.UTC()
	userResult, err := tx.ExecContext(ctx, `
		UPDATE user_stats
		SET accepted_count = accepted_count + $2,
			updated_at = $3
		WHERE user_id = $1
			AND deleted_at IS NULL
			AND accepted_count + $2 >= 0
			AND accepted_count + $2 <= total_submissions`,
		locked.userID,
		delta,
		now,
	)
	if err != nil {
		return fmt.Errorf("adjust active user accepted count: %w", err)
	}
	affected, affectErr := userResult.RowsAffected()
	if affectErr != nil || affected != 1 {
		return ports.ErrFinalizationConflict
	}
	problemResult, err := tx.ExecContext(ctx, `
		UPDATE problems
		SET accepted_count = accepted_count + $2,
			updated_at = $3
		WHERE id = $1
			AND deleted_at IS NULL
			AND accepted_count + $2 >= 0
			AND accepted_count + $2 <= submission_count`,
		locked.problemID,
		delta,
		now,
	)
	if err != nil {
		return fmt.Errorf("adjust active problem accepted count: %w", err)
	}
	affected, affectErr = problemResult.RowsAffected()
	if affectErr != nil || affected != 1 {
		return ports.ErrFinalizationConflict
	}
	return nil
}

// rebuildContestStanding first advances the canonical normalized attempt, then
// rebuilds both official and freeze-safe projections from immutable contest
// order. It never increments score from worker delivery order, so retry and
// administrative rejudge converge on the same state.
func (repository *FinalizationRepository) rebuildContestStanding(
	ctx context.Context,
	tx *sql.Tx,
	locked lockedFinalization,
	now time.Time,
) error {
	hasContest := locked.contestID.Valid
	hasCompleteSnapshot := hasContest &&
		locked.contestProblemID.Valid &&
		locked.contestPolicyVersion.Valid &&
		locked.contestElapsedMS.Valid &&
		locked.contestSequence.Valid
	if !hasContest && !locked.contestProblemID.Valid &&
		!locked.contestPolicyVersion.Valid &&
		!locked.contestElapsedMS.Valid &&
		!locked.contestSequence.Valid {
		return nil
	}
	if repository == nil || repository.newID == nil || tx == nil ||
		!hasCompleteSnapshot || locked.contestID.UUID == uuid.Nil ||
		locked.contestProblemID.UUID == uuid.Nil ||
		locked.contestPolicyVersion.Int64 < 1 ||
		locked.contestElapsedMS.Int64 < 0 ||
		locked.contestSequence.Int64 < 1 ||
		locked.userID == uuid.Nil || locked.problemID == uuid.Nil ||
		now.IsZero() {
		return ports.ErrFinalizationConflict
	}
	now = now.UTC()
	if err := lockContestStanding(
		ctx,
		tx,
		locked.contestID.UUID,
		locked.userID,
	); err != nil {
		return err
	}
	standing, found, err := readLockedContestStanding(
		ctx,
		tx,
		locked.contestID.UUID,
		locked.userID,
	)
	if err != nil {
		return err
	}
	if !found {
		standingID := repository.newID()
		if standingID == uuid.Nil {
			return ports.ErrFinalizationUnavailable
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO contest_standings (
				id, contest_id, user_id, solved_count, penalty, problem_results
			) VALUES ($1, $2, $3, 0, 0, '{}'::jsonb)`,
			standingID,
			locked.contestID.UUID,
			locked.userID,
		); err != nil {
			return fmt.Errorf("create contest standing projection: %w", err)
		}
		standing, found, err = readLockedContestStanding(
			ctx,
			tx,
			locked.contestID.UUID,
			locked.userID,
		)
		if err != nil {
			return err
		}
		if !found {
			return ports.ErrFinalizationConflict
		}
	}
	if err := repository.syncContestScoringAttempt(
		ctx,
		tx,
		locked,
		now,
	); err != nil {
		return err
	}

	attempts, err := loadContestScoringAttempts(
		ctx,
		tx,
		locked.contestID.UUID,
		locked.userID,
	)
	if err != nil {
		return err
	}
	officialScores, err := recomputeContestProblemScores(attempts, nil)
	if err != nil {
		return err
	}
	affectedScore, ok := officialScores[locked.problemID]
	if !ok || affectedScore.contestProblemID != locked.contestProblemID.UUID {
		return ports.ErrFinalizationConflict
	}
	if err := repository.upsertContestProblemResult(
		ctx,
		tx,
		locked,
		affectedScore,
		now,
	); err != nil {
		return err
	}

	official, err := aggregateContestStanding(officialScores)
	if err != nil {
		return err
	}
	freezeAt, err := contestFreezeAt(ctx, tx, locked.contestID.UUID)
	if err != nil {
		return err
	}
	frozenScores := officialScores
	pendingByProblem := map[string]int{}
	pendingAttempts := 0
	if freezeAt != nil {
		frozenScores, err = recomputeContestProblemScores(attempts, freezeAt)
		if err != nil {
			return err
		}
		pendingByProblem, pendingAttempts, err =
			loadPostFreezeAttemptCounts(
				ctx,
				tx,
				locked.contestID.UUID,
				locked.userID,
				*freezeAt,
			)
		if err != nil {
			return err
		}
	}
	frozen, err := aggregateContestStanding(frozenScores)
	if err != nil {
		return err
	}
	officialResults, err := json.Marshal(official.problemResults)
	if err != nil {
		return fmt.Errorf("encode official contest standing projection: %w", err)
	}
	frozenResults, err := json.Marshal(frozen.problemResults)
	if err != nil {
		return fmt.Errorf("encode frozen contest standing projection: %w", err)
	}
	pendingResults, err := json.Marshal(pendingByProblem)
	if err != nil {
		return fmt.Errorf("encode frozen contest pending projection: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE contest_standings
		SET solved_count = $4,
			penalty = $5,
			problem_results = $6::jsonb,
			last_accepted_elapsed_ms = $7,
			version = version + 1,
			frozen_solved_count = $8,
			frozen_penalty = $9,
			frozen_problem_results = $10::jsonb,
			frozen_pending_problem_attempts = $11::jsonb,
			frozen_pending_attempts = $12,
			frozen_last_accepted_elapsed_ms = $13,
			frozen_version = frozen_version + 1
		WHERE id = $1 AND contest_id = $2 AND user_id = $3
			AND version = $14 AND frozen_version = $15`,
		standing.ID,
		standing.ContestID,
		standing.UserID,
		official.solvedCount,
		official.penaltySeconds,
		officialResults,
		official.lastAcceptedElapsedMS,
		frozen.solvedCount,
		frozen.penaltySeconds,
		frozenResults,
		pendingResults,
		pendingAttempts,
		frozen.lastAcceptedElapsedMS,
		standing.Version,
		standing.FrozenVersion,
	)
	if err != nil {
		return fmt.Errorf("replace contest standing projection: %w", err)
	}
	affected, affectErr := result.RowsAffected()
	if affectErr != nil || affected != 1 {
		return ports.ErrFinalizationConflict
	}
	var projectionVersion int64
	if err := tx.QueryRowContext(ctx, `
		UPDATE contests
		SET standings_projection_version = standings_projection_version + 1,
			updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING standings_projection_version`,
		locked.contestID.UUID,
		now,
	).Scan(&projectionVersion); err != nil {
		return fmt.Errorf("advance contest standings projection version: %w", err)
	}
	if projectionVersion < 2 {
		return ports.ErrFinalizationConflict
	}
	return nil
}

type contestScoringAttempt struct {
	problemID        uuid.UUID
	contestProblemID uuid.UUID
	submissionID     uuid.UUID
	submittedAt      time.Time
	elapsedMS        int64
	sequence         int64
	terminalVerdict  string
}

type contestProblemProjection struct {
	contestProblemID      uuid.UUID
	accepted              bool
	wrongAttempts         int
	elapsedMS             int64
	lastScoringSubmission *uuid.UUID
	acceptedSequence      *int64
}

type contestStandingProjection struct {
	solvedCount           int
	penaltySeconds        int
	lastAcceptedElapsedMS *int64
	problemResults        map[string]any
}

func (repository *FinalizationRepository) syncContestScoringAttempt(
	ctx context.Context,
	tx *sql.Tx,
	locked lockedFinalization,
	now time.Time,
) error {
	id := repository.newID()
	if id == uuid.Nil {
		return ports.ErrFinalizationUnavailable
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO contest_scoring_attempts (
			id, contest_id, user_id, problem_id, contest_problem_id,
			submission_id, submitted_at, submission_elapsed_ms,
			contest_submission_sequence, active_generation, terminal_verdict,
			judgement_version, source_event_id, version, created_at, updated_at
		)
		SELECT $1, contest_id, user_id, problem_id, contest_problem_id,
			id, submitted_at, contest_elapsed_ms, contest_submission_sequence,
			active_judgement_generation, active_verdict, verdict_version,
			active_judgement_id, 1, $2, $2
		FROM submissions
		WHERE id = $3
			AND contest_id = $4
			AND user_id = $5
			AND problem_id = $6
			AND contest_problem_id = $7
			AND deleted_at IS NULL
			AND active_judgement_id IS NOT NULL
			AND active_judgement_generation IS NOT NULL
			AND active_verdict IS NOT NULL
			AND verdict_version >= 1
		ON CONFLICT (submission_id) DO UPDATE
		SET active_generation = EXCLUDED.active_generation,
			terminal_verdict = EXCLUDED.terminal_verdict,
			judgement_version = EXCLUDED.judgement_version,
			source_event_id = EXCLUDED.source_event_id,
			version = contest_scoring_attempts.version + 1,
			updated_at = EXCLUDED.updated_at
		WHERE EXCLUDED.judgement_version >
			contest_scoring_attempts.judgement_version`,
		id,
		now,
		locked.submissionID,
		locked.contestID.UUID,
		locked.userID,
		locked.problemID,
		locked.contestProblemID.UUID,
	)
	if err != nil {
		return fmt.Errorf("advance canonical contest scoring attempt: %w", err)
	}
	affected, affectErr := result.RowsAffected()
	if affectErr != nil || affected != 1 {
		return ports.ErrFinalizationConflict
	}
	return nil
}

func loadContestScoringAttempts(
	ctx context.Context,
	tx *sql.Tx,
	contestID uuid.UUID,
	userID uuid.UUID,
) ([]contestScoringAttempt, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT problem_id, contest_problem_id, submission_id, submitted_at,
			submission_elapsed_ms, contest_submission_sequence, terminal_verdict
		FROM contest_scoring_attempts
		WHERE contest_id = $1 AND user_id = $2
		ORDER BY contest_submission_sequence ASC, submission_id ASC`,
		contestID,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("load canonical contest scoring attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	attempts := make([]contestScoringAttempt, 0, 16)
	var previousSequence int64
	for rows.Next() {
		var attempt contestScoringAttempt
		if err := rows.Scan(
			&attempt.problemID,
			&attempt.contestProblemID,
			&attempt.submissionID,
			&attempt.submittedAt,
			&attempt.elapsedMS,
			&attempt.sequence,
			&attempt.terminalVerdict,
		); err != nil {
			return nil, fmt.Errorf("scan canonical contest scoring attempt: %w", err)
		}
		attempt.submittedAt = attempt.submittedAt.UTC()
		if attempt.problemID == uuid.Nil || attempt.contestProblemID == uuid.Nil ||
			attempt.submissionID == uuid.Nil || attempt.submittedAt.IsZero() ||
			attempt.elapsedMS < 0 || attempt.sequence < 1 ||
			previousSequence >= attempt.sequence ||
			!contestantScoringVerdict(attempt.terminalVerdict) {
			return nil, ports.ErrFinalizationConflict
		}
		previousSequence = attempt.sequence
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate canonical contest scoring attempts: %w", err)
	}
	return attempts, nil
}

func contestantScoringVerdict(value string) bool {
	switch value {
	case "accepted", "wrong_answer", "compile_error", "runtime_error",
		"time_limit_exceeded", "memory_limit_exceeded",
		"process_limit_exceeded", "output_limit_exceeded":
		return true
	default:
		return false
	}
}

func recomputeContestProblemScores(
	attempts []contestScoringAttempt,
	before *time.Time,
) (map[uuid.UUID]contestProblemProjection, error) {
	scores := make(map[uuid.UUID]contestProblemProjection)
	for _, attempt := range attempts {
		if before != nil && !attempt.submittedAt.Before(before.UTC()) {
			continue
		}
		score, exists := scores[attempt.problemID]
		if !exists {
			score = contestProblemProjection{
				contestProblemID: attempt.contestProblemID,
			}
		}
		if score.contestProblemID != attempt.contestProblemID {
			return nil, ports.ErrFinalizationConflict
		}
		if score.accepted {
			continue
		}
		submissionID := attempt.submissionID
		score.lastScoringSubmission = &submissionID
		if attempt.terminalVerdict == "accepted" {
			sequence := attempt.sequence
			score.accepted = true
			score.elapsedMS = attempt.elapsedMS
			score.acceptedSequence = &sequence
		} else {
			if score.wrongAttempts == math.MaxInt {
				return nil, ports.ErrFinalizationConflict
			}
			score.wrongAttempts++
		}
		scores[attempt.problemID] = score
	}
	return scores, nil
}

func aggregateContestStanding(
	scores map[uuid.UUID]contestProblemProjection,
) (contestStandingProjection, error) {
	projection := contestStandingProjection{
		problemResults: make(map[string]any, len(scores)),
	}
	for problemID, score := range scores {
		if problemID == uuid.Nil || score.contestProblemID == uuid.Nil ||
			score.wrongAttempts < 0 || score.elapsedMS < 0 ||
			(!score.accepted && score.elapsedMS != 0) {
			return contestStandingProjection{}, ports.ErrFinalizationConflict
		}
		projection.problemResults[problemID.String()] = map[string]any{
			"ac":       score.accepted,
			"wa_count": score.wrongAttempts,
			"time":     int(score.elapsedMS / 1_000),
			"time_ms":  score.elapsedMS,
		}
		if !score.accepted {
			continue
		}
		if score.elapsedMS/1_000 > int64(math.MaxInt) ||
			score.wrongAttempts >
				(math.MaxInt-int(score.elapsedMS/1_000))/
					contestentity.ICPCWrongAttemptPenaltySeconds {
			return contestStandingProjection{}, ports.ErrFinalizationConflict
		}
		penalty := int(score.elapsedMS/1_000) +
			score.wrongAttempts*contestentity.ICPCWrongAttemptPenaltySeconds
		if projection.solvedCount == math.MaxInt ||
			projection.penaltySeconds > math.MaxInt-penalty {
			return contestStandingProjection{}, ports.ErrFinalizationConflict
		}
		projection.solvedCount++
		projection.penaltySeconds += penalty
		if projection.lastAcceptedElapsedMS == nil ||
			score.elapsedMS > *projection.lastAcceptedElapsedMS {
			elapsed := score.elapsedMS
			projection.lastAcceptedElapsedMS = &elapsed
		}
	}
	return projection, nil
}

func (repository *FinalizationRepository) upsertContestProblemResult(
	ctx context.Context,
	tx *sql.Tx,
	locked lockedFinalization,
	score contestProblemProjection,
	now time.Time,
) error {
	id := repository.newID()
	if id == uuid.Nil ||
		(score.accepted != (score.acceptedSequence != nil)) ||
		(score.accepted && score.lastScoringSubmission == nil) {
		return ports.ErrFinalizationUnavailable
	}
	var acceptedElapsed any
	if score.accepted {
		acceptedElapsed = score.elapsedMS
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO contest_problem_results (
			id, contest_id, user_id, problem_id, contest_problem_id,
			accepted, accepted_elapsed_ms, wrong_attempts_before_accept,
			best_score, last_scoring_submission_id, last_accepted_sequence,
			version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, 0, $9, $10, 1, $11, $11
		)
		ON CONFLICT (contest_id, user_id, contest_problem_id) DO UPDATE
		SET accepted = EXCLUDED.accepted,
			accepted_elapsed_ms = EXCLUDED.accepted_elapsed_ms,
			wrong_attempts_before_accept =
				EXCLUDED.wrong_attempts_before_accept,
			best_score = EXCLUDED.best_score,
			last_scoring_submission_id =
				EXCLUDED.last_scoring_submission_id,
			last_accepted_sequence = EXCLUDED.last_accepted_sequence,
			version = contest_problem_results.version + 1,
			updated_at = EXCLUDED.updated_at
		WHERE contest_problem_results.problem_id = EXCLUDED.problem_id`,
		id,
		locked.contestID.UUID,
		locked.userID,
		locked.problemID,
		locked.contestProblemID.UUID,
		score.accepted,
		acceptedElapsed,
		score.wrongAttempts,
		score.lastScoringSubmission,
		score.acceptedSequence,
		now,
	)
	if err != nil {
		return fmt.Errorf("replace normalized contest problem result: %w", err)
	}
	affected, affectErr := result.RowsAffected()
	if affectErr != nil || affected != 1 {
		return ports.ErrFinalizationConflict
	}
	return nil
}

func contestFreezeAt(
	ctx context.Context,
	tx *sql.Tx,
	contestID uuid.UUID,
) (*time.Time, error) {
	var value sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT freeze_standings_at
		FROM contests
		WHERE id = $1 AND deleted_at IS NULL
		FOR SHARE`,
		contestID,
	).Scan(&value); err != nil {
		return nil, fmt.Errorf("read contest freeze policy: %w", err)
	}
	if !value.Valid {
		return nil, nil
	}
	normalized := value.Time.UTC()
	return &normalized, nil
}

func loadPostFreezeAttemptCounts(
	ctx context.Context,
	tx *sql.Tx,
	contestID uuid.UUID,
	userID uuid.UUID,
	freezeAt time.Time,
) (map[string]int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT problem_id, COUNT(*)
		FROM submissions
		WHERE contest_id = $1 AND user_id = $2
			AND submitted_at >= $3
			AND contest_problem_id IS NOT NULL
			AND contest_submission_sequence IS NOT NULL
			AND deleted_at IS NULL
		GROUP BY problem_id
		ORDER BY problem_id ASC`,
		contestID,
		userID,
		freezeAt.UTC(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("load post-freeze contest attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	counts := make(map[string]int)
	total := 0
	for rows.Next() {
		var problemID uuid.UUID
		var count int64
		if err := rows.Scan(&problemID, &count); err != nil {
			return nil, 0, fmt.Errorf("scan post-freeze contest attempts: %w", err)
		}
		if problemID == uuid.Nil || count < 1 ||
			count > int64(math.MaxInt) ||
			total > math.MaxInt-int(count) {
			return nil, 0, ports.ErrFinalizationConflict
		}
		counts[problemID.String()] = int(count)
		total += int(count)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate post-freeze contest attempts: %w", err)
	}
	return counts, total, nil
}

func lockContestStanding(
	ctx context.Context,
	tx *sql.Tx,
	contestID uuid.UUID,
	userID uuid.UUID,
) error {
	var ignored any
	if err := tx.QueryRowContext(
		ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1, $2))",
		contestID.String()+":"+userID.String(),
		submissioncontract.ContestStandingAdvisorySeed,
	).Scan(&ignored); err != nil {
		return fmt.Errorf("lock contest standing projection: %w", err)
	}
	return nil
}

func readLockedContestStanding(
	ctx context.Context,
	tx *sql.Tx,
	contestID uuid.UUID,
	userID uuid.UUID,
) (contestentity.ContestStanding, bool, error) {
	var (
		standing           contestentity.ContestStanding
		raw                json.RawMessage
		frozenRaw          json.RawMessage
		pendingRaw         json.RawMessage
		lastAccepted       sql.NullInt64
		frozenLastAccepted sql.NullInt64
	)
	err := tx.QueryRowContext(ctx, `
		SELECT id, contest_id, user_id, solved_count, penalty, problem_results,
			last_accepted_elapsed_ms, version, frozen_solved_count,
			frozen_penalty, frozen_problem_results,
			frozen_pending_problem_attempts, frozen_pending_attempts,
			frozen_last_accepted_elapsed_ms, frozen_version
		FROM contest_standings
		WHERE contest_id = $1 AND user_id = $2
		FOR UPDATE`,
		contestID,
		userID,
	).Scan(
		&standing.ID,
		&standing.ContestID,
		&standing.UserID,
		&standing.SolvedCount,
		&standing.Penalty,
		&raw,
		&lastAccepted,
		&standing.Version,
		&standing.FrozenSolvedCount,
		&standing.FrozenPenalty,
		&frozenRaw,
		&pendingRaw,
		&standing.FrozenPendingAttemptCount,
		&frozenLastAccepted,
		&standing.FrozenVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return contestentity.ContestStanding{}, false, nil
	}
	if err != nil {
		return contestentity.ContestStanding{}, false,
			fmt.Errorf("lock contest standing projection row: %w", err)
	}
	if len(raw) == 0 || len(frozenRaw) == 0 || len(pendingRaw) == 0 ||
		json.Unmarshal(raw, &standing.ProblemResults) != nil ||
		json.Unmarshal(frozenRaw, &standing.FrozenProblemResults) != nil ||
		json.Unmarshal(pendingRaw, &standing.FrozenPendingByProblem) != nil ||
		standing.ProblemResults == nil ||
		standing.FrozenProblemResults == nil ||
		standing.FrozenPendingByProblem == nil ||
		standing.Version < 1 || standing.FrozenVersion < 1 {
		return contestentity.ContestStanding{}, false,
			ports.ErrFinalizationConflict
	}
	if lastAccepted.Valid {
		value := lastAccepted.Int64
		if value < 0 {
			return contestentity.ContestStanding{}, false,
				ports.ErrFinalizationConflict
		}
		standing.LastAcceptedElapsedMS = &value
	}
	if frozenLastAccepted.Valid {
		value := frozenLastAccepted.Int64
		if value < 0 {
			return contestentity.ContestStanding{}, false,
				ports.ErrFinalizationConflict
		}
		standing.FrozenLastAcceptedMS = &value
	}
	return standing, true, nil
}

func updateSystemErrorSubmission(
	ctx context.Context,
	tx *sql.Tx,
	locked lockedFinalization,
	judgement entity.Judgement,
	next entity.SubmissionState,
	now time.Time,
) error {
	now = now.UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE submissions
		SET latest_generation_phase = 'terminal',
			latest_generation_outcome = 'system_error',
			latest_generation_completed_at = $4,
			status = 'internal_error', passed_count = 0, total_count = 0,
			time_ms = NULL, memory_kb = NULL, error_message = NULL,
			judged_at = $4, verdict_version = verdict_version + 1,
			version = version + 1, updated_at = $4
		WHERE id = $1 AND latest_generation = $2 AND version = $3
			AND latest_generation_phase = 'finalizing'
			AND latest_generation_outcome IS NULL`,
		locked.submissionID, locked.generation, locked.submissionVersion, now,
	)
	if err != nil {
		return fmt.Errorf("advance system-error submission finalization: %w", err)
	}
	affected, affectErr := result.RowsAffected()
	if affectErr != nil || affected != 1 ||
		judgement.Verdict != entity.VerdictSystemError ||
		next.AggregateVersion != locked.submissionVersion+1 {
		return ports.ErrFinalizationConflict
	}
	return nil
}

func submissionStatusForVerdict(verdict entity.Verdict) string {
	switch verdict {
	case entity.VerdictAccepted, entity.VerdictWrongAnswer,
		entity.VerdictCompileError, entity.VerdictRuntimeError,
		entity.VerdictTimeLimitExceeded, entity.VerdictMemoryLimitExceeded,
		entity.VerdictProcessLimitExceeded,
		entity.VerdictOutputLimitExceeded:
		return string(verdict)
	default:
		return "internal_error"
	}
}

func completeFinalizationJob(
	ctx context.Context,
	tx *sql.Tx,
	jobID uuid.UUID,
	now time.Time,
) error {
	now = now.UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE judge_jobs
		SET state = 'done', completed_at = $2,
			lease_owner = NULL, lease_token_hash = NULL, lease_until = NULL,
			heartbeat_at = NULL, updated_at = $2
		WHERE id = $1 AND state = 'leased'`, jobID, now)
	if err != nil {
		return fmt.Errorf("complete finalization job: %w", err)
	}
	affected, affectErr := result.RowsAffected()
	if affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	return nil
}

func deadFinalizationJob(
	ctx context.Context,
	tx *sql.Tx,
	jobID uuid.UUID,
	failure ports.InfrastructureFailure,
	now time.Time,
) error {
	now = now.UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE judge_jobs
		SET state = 'dead', dead_at = $2,
			lease_owner = NULL, lease_token_hash = NULL, lease_until = NULL,
			heartbeat_at = NULL, last_error_code = $3, last_error_summary = $4,
			updated_at = $2
		WHERE id = $1 AND state = 'leased'`,
		jobID, now, failure.Code, failure.Summary,
	)
	if err != nil {
		return fmt.Errorf("dead finalization job: %w", err)
	}
	affected, affectErr := result.RowsAffected()
	if affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	return nil
}

func releaseFinalizationCapacity(
	ctx context.Context,
	tx *sql.Tx,
	lease ports.JobLease,
	now time.Time,
) error {
	return releaseWorkerCapacity(ctx, tx, expiredJob{
		jobID: lease.Job.ID, submissionID: lease.Job.SubmissionID,
		generation: lease.Job.Generation, attemptID: lease.Attempt.ID,
		workerID: lease.Attempt.WorkerID, workerBootID: lease.Attempt.WorkerBootID,
	}, now.UTC())
}

func (repository *FinalizationRepository) contestantJournalRecords(
	ctx context.Context,
	locked lockedFinalization,
	previous entity.SubmissionState,
	next entity.SubmissionState,
	judgement entity.Judgement,
	now time.Time,
	solveEvent *eventing.Event,
) ([]eventing.Event, audit.Entry, error) {
	now = now.UTC()
	eventID := repository.newID()
	payload, err := json.Marshal(submissioncontract.SubmissionJudgedV1{
		EventID: eventID, SubmissionID: locked.submissionID,
		UserID: locked.userID, ProblemID: locked.problemID,
		Generation: locked.generation, JudgementID: judgement.ID,
		ProblemRevisionID: judgement.ProblemRevisionID,
		TestsetRevisionID: judgement.TestsetRevisionID,
		AggregateVersion:  next.AggregateVersion, LatestPhase: string(next.LatestPhase),
		LatestOutcome: next.LatestOutcome, Verdict: string(judgement.Verdict),
		PassedCount: judgement.PassedCount, TotalCount: judgement.TotalCount,
		MaximumTimeMS:   judgement.MaximumTimeMilliseconds,
		MaximumMemoryKB: judgement.MaximumMemoryKilobytes,
		BecameActive:    true, PreviousActive: activeJudgementDTO(previous),
		ResultingActive: activeJudgementDTO(next), Contest: contestSnapshot(locked),
		SubmittedAt: locked.submittedAt.UTC(), OccurredAt: now,
		CorrelationID: locked.originCID,
	})
	if err != nil {
		return nil, audit.Entry{}, err
	}
	events := []eventing.Event{{
		ID: eventID, Type: submissioncontract.EventSubmissionJudgedV1,
		AggregateType: submissioncontract.AggregateSubmission,
		AggregateID:   locked.submissionID, AggregateVersion: next.AggregateVersion,
		CorrelationID: locked.originCID, Payload: payload, OccurredAt: now,
	}}
	if solveEvent != nil {
		events = append(events, *solveEvent)
	}
	return events, audit.Entry{
		Action: "submission.finalized", Resource: submissioncontract.AggregateSubmission,
		ResourceID: locked.submissionID.String(), Reason: "judge contestant evaluation finalized",
		Before:        map[string]any{"generation": previous.LatestGeneration, "phase": previous.LatestPhase, "outcome": previous.LatestOutcome},
		After:         map[string]any{"generation": next.LatestGeneration, "phase": next.LatestPhase, "outcome": next.LatestOutcome, "judgement_id": judgement.ID.String(), "verdict": judgement.Verdict},
		CorrelationID: locked.originCID, OccurredAt: now,
	}, nil
}

func (repository *FinalizationRepository) systemErrorJournalRecords(
	ctx context.Context,
	locked lockedFinalization,
	previous entity.SubmissionState,
	next entity.SubmissionState,
	judgement entity.Judgement,
	failure ports.InfrastructureFailure,
	now time.Time,
) ([]eventing.Event, audit.Entry, error) {
	now = now.UTC()
	eventID := repository.newID()
	payload, err := json.Marshal(submissioncontract.SubmissionJudgedV1{
		EventID: eventID, SubmissionID: locked.submissionID,
		UserID: locked.userID, ProblemID: locked.problemID,
		Generation: locked.generation, JudgementID: judgement.ID,
		ProblemRevisionID: judgement.ProblemRevisionID,
		TestsetRevisionID: judgement.TestsetRevisionID,
		AggregateVersion:  next.AggregateVersion, LatestPhase: string(next.LatestPhase),
		LatestOutcome: next.LatestOutcome, Verdict: string(judgement.Verdict),
		PassedCount: 0, TotalCount: 0, BecameActive: false,
		PreviousActive: activeJudgementDTO(previous), ResultingActive: activeJudgementDTO(next),
		Contest: contestSnapshot(locked), SubmittedAt: locked.submittedAt.UTC(),
		OccurredAt: now, CorrelationID: locked.originCID,
	})
	if err != nil {
		return nil, audit.Entry{}, err
	}
	return []eventing.Event{{
			ID: eventID, Type: submissioncontract.EventSubmissionJudgedV1,
			AggregateType: submissioncontract.AggregateSubmission,
			AggregateID:   locked.submissionID, AggregateVersion: next.AggregateVersion,
			CorrelationID: locked.originCID, Payload: payload, OccurredAt: now,
		}}, audit.Entry{
			Action: "submission.system_error", Resource: submissioncontract.AggregateSubmission,
			ResourceID: locked.submissionID.String(), Reason: failure.Code,
			Before:        map[string]any{"generation": previous.LatestGeneration, "phase": previous.LatestPhase, "outcome": previous.LatestOutcome},
			After:         map[string]any{"generation": next.LatestGeneration, "phase": next.LatestPhase, "outcome": next.LatestOutcome, "judgement_id": judgement.ID.String(), "failure_classification": failure.Classification, "failure_code": failure.Code},
			CorrelationID: locked.originCID, OccurredAt: now,
		}, nil
}

func activeJudgementDTO(state entity.SubmissionState) *submissioncontract.ActiveJudgementV1 {
	if state.ActiveJudgementID == nil || state.ActiveGeneration == nil ||
		state.ActiveVerdict == nil {
		return nil
	}
	return &submissioncontract.ActiveJudgementV1{
		JudgementID: *state.ActiveJudgementID, Generation: *state.ActiveGeneration,
		Verdict: string(*state.ActiveVerdict),
	}
}

func contestSnapshot(locked lockedFinalization) *submissioncontract.ContestSubmissionSnapshotV1 {
	if !locked.contestID.Valid || !locked.contestProblemID.Valid ||
		!locked.contestPolicyVersion.Valid || !locked.contestElapsedMS.Valid ||
		!locked.contestSequence.Valid {
		return nil
	}
	return &submissioncontract.ContestSubmissionSnapshotV1{
		ContestID: locked.contestID.UUID, ContestProblemID: locked.contestProblemID.UUID,
		ScoringPolicyVersion: locked.contestPolicyVersion.Int64,
		ElapsedMilliseconds:  locked.contestElapsedMS.Int64,
		SubmissionSequence:   locked.contestSequence.Int64,
	}
}

func solveLock(ctx context.Context, tx *sql.Tx, userID uuid.UUID, problemID uuid.UUID) error {
	var ignored any
	if err := tx.QueryRowContext(ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1, $2))",
		userID.String()+":"+problemID.String(),
		submissioncontract.SolveProjectionAdvisorySeed,
	).Scan(&ignored); err != nil {
		return fmt.Errorf("lock solve projection: %w", err)
	}
	return nil
}

// reconcileSolve serializes the projection by its stable user/problem key,
// then derives it from all active Accepted judgements. It intentionally does
// not trust the just-finalized submission: an older submission can remain the
// canonical first solve after a later accepted submission is finalized.
func (repository *FinalizationRepository) reconcileSolve(
	ctx context.Context,
	tx *sql.Tx,
	locked lockedFinalization,
	now time.Time,
) (*entity.SolveTransition, *eventing.Event, error) {
	if repository == nil || repository.newID == nil || tx == nil ||
		locked.userID == uuid.Nil || locked.problemID == uuid.Nil ||
		locked.originCID == "" {
		return nil, nil, ports.ErrFinalizationUnavailable
	}
	if err := solveLock(ctx, tx, locked.userID, locked.problemID); err != nil {
		return nil, nil, err
	}
	current, err := repository.lockOrCreateSolve(ctx, tx, locked, now.UTC())
	if err != nil {
		return nil, nil, err
	}
	candidate, err := selectAcceptedSolveCandidate(ctx, tx, locked.userID, locked.problemID)
	if err != nil {
		return nil, nil, err
	}
	desired, err := submissionservice.DesiredSolveSelection(candidate)
	if err != nil {
		return nil, nil, fmt.Errorf("derive canonical solve selection: %w", err)
	}
	transitionID := repository.newID()
	if transitionID == uuid.Nil {
		return nil, nil, ports.ErrFinalizationUnavailable
	}
	next, transition, changed, err := submissionservice.PlanSolvedState(
		current,
		desired,
		entity.SolveTransitionMetadata{
			ID: transitionID, Provenance: entity.SolveTransitionLive,
			OccurredAt: now.UTC(), CorrelationID: locked.originCID,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("plan canonical solve transition: %w", err)
	}
	if !changed {
		return nil, nil, nil
	}
	eventID := repository.newID()
	if eventID == uuid.Nil {
		return nil, nil, ports.ErrFinalizationUnavailable
	}
	if err := insertSolveTransition(ctx, tx, *transition, eventID); err != nil {
		return nil, nil, err
	}
	if err := updateSolveProjection(ctx, tx, current, next, now.UTC()); err != nil {
		return nil, nil, err
	}
	event, err := repository.solveTransitionEvent(*transition, eventID)
	if err != nil {
		return nil, nil, err
	}
	return transition, &event, nil
}

func (repository *FinalizationRepository) lockOrCreateSolve(
	ctx context.Context,
	tx *sql.Tx,
	locked lockedFinalization,
	now time.Time,
) (entity.UserProblemSolve, error) {
	current, found, err := readLockedSolve(ctx, tx, locked.userID, locked.problemID)
	if err != nil || found {
		return current, err
	}
	solveID := repository.newID()
	if solveID == uuid.Nil {
		return entity.UserProblemSolve{}, ports.ErrFinalizationUnavailable
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_problem_solves (
			id, created_at, updated_at, user_id, problem_id, status, version
		) VALUES ($1, $2, $2, $3, $4, 'unsolved', 0)`,
		solveID, now, locked.userID, locked.problemID,
	); err != nil {
		return entity.UserProblemSolve{}, fmt.Errorf("create solve projection: %w", err)
	}
	current, found, err = readLockedSolve(ctx, tx, locked.userID, locked.problemID)
	if err != nil || !found {
		if err != nil {
			return entity.UserProblemSolve{}, err
		}
		return entity.UserProblemSolve{}, ports.ErrFinalizationConflict
	}
	return current, nil
}

func readLockedSolve(
	ctx context.Context,
	tx *sql.Tx,
	userID uuid.UUID,
	problemID uuid.UUID,
) (entity.UserProblemSolve, bool, error) {
	var (
		value                                                              entity.UserProblemSolve
		status                                                             string
		submissionID, judgementID, revisionID, grantID, activeTransitionID uuid.NullUUID
		generation                                                         sql.NullInt64
		rewardFacts                                                        json.RawMessage
		rewardChecksum                                                     []byte
		provenance                                                         sql.NullString
		solvedAt                                                           sql.NullTime
	)
	err := tx.QueryRowContext(ctx, `
		SELECT solve.id, solve.user_id, solve.problem_id, solve.version, solve.status,
			solve.active_submission_id, solve.active_judgement_id, solve.active_generation,
			solve.active_problem_revision_id, solve.reward_facts, solve.reward_facts_checksum,
			solve.reward_provenance, solve.solved_at, solve.active_reward_transition_id,
			transition.to_reward_grant_source_id
		FROM user_problem_solves AS solve
		LEFT JOIN user_problem_solve_transitions AS transition
			ON transition.id = solve.active_reward_transition_id
		WHERE solve.user_id = $1 AND solve.problem_id = $2
		FOR UPDATE OF solve`, userID, problemID).Scan(
		&value.ID, &value.UserID, &value.ProblemID, &value.Version, &status,
		&submissionID, &judgementID, &generation, &revisionID, &rewardFacts,
		&rewardChecksum, &provenance, &solvedAt, &activeTransitionID, &grantID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.UserProblemSolve{}, false, nil
	}
	if err != nil {
		return entity.UserProblemSolve{}, false, fmt.Errorf("lock solve projection: %w", err)
	}
	value.Selection.Status = entity.SolveStatus(status)
	if value.Selection.Status == entity.SolveUnsolved {
		if submissionID.Valid || judgementID.Valid || generation.Valid || revisionID.Valid ||
			len(rewardFacts) != 0 || len(rewardChecksum) != 0 || provenance.Valid ||
			solvedAt.Valid || activeTransitionID.Valid || grantID.Valid || value.Validate() != nil {
			return entity.UserProblemSolve{}, false, ports.ErrFinalizationConflict
		}
		return value, true, nil
	}
	if !submissionID.Valid || !judgementID.Valid || !generation.Valid || !revisionID.Valid ||
		len(rewardChecksum) != 32 || !provenance.Valid || !solvedAt.Valid ||
		!activeTransitionID.Valid || !grantID.Valid {
		return entity.UserProblemSolve{}, false, ports.ErrFinalizationConflict
	}
	checksum := [32]byte{}
	copy(checksum[:], rewardChecksum)
	value.Selection = entity.SolveSelection{
		Status: entity.SolveSolved, SubmissionID: uuidPointer(submissionID.UUID),
		JudgementID: uuidPointer(judgementID.UUID), Generation: int64Pointer(generation.Int64),
		ProblemRevisionID:   uuidPointer(revisionID.UUID),
		RewardFacts:         append(json.RawMessage(nil), rewardFacts...),
		RewardFactsChecksum: &checksum,
		RewardGrantSourceID: uuidPointer(grantID.UUID),
		RewardProvenance:    rewardProvenancePointer(entity.RewardProvenance(provenance.String)),
		SolvedAt:            timePointer(solvedAt.Time.UTC()),
	}
	value.ActiveRewardTransitionID = uuidPointer(activeTransitionID.UUID)
	if value.Validate() != nil {
		return entity.UserProblemSolve{}, false, ports.ErrFinalizationConflict
	}
	return value, true, nil
}

func selectAcceptedSolveCandidate(
	ctx context.Context,
	tx *sql.Tx,
	userID uuid.UUID,
	problemID uuid.UUID,
) (*submissionservice.AcceptedSolveCandidate, error) {
	var candidate submissionservice.AcceptedSolveCandidate
	var checksum []byte
	err := tx.QueryRowContext(ctx, `
		SELECT submission.id, judgement.id, judgement.generation,
			judgement.problem_revision_id, judgement.reward_snapshot,
			judgement.reward_snapshot_checksum, judgement.completed_at
		FROM submissions AS submission
		JOIN submission_judgements AS judgement
			ON judgement.id = submission.active_judgement_id
			AND judgement.submission_id = submission.id
			AND judgement.generation = submission.active_judgement_generation
			AND judgement.verdict = submission.active_verdict
		WHERE submission.user_id = $1 AND submission.problem_id = $2
			AND submission.active_verdict = 'accepted'
		ORDER BY submission.submitted_at ASC, submission.id ASC
		LIMIT 1
		FOR SHARE OF submission`, userID, problemID).Scan(
		// submission_judgements is append-only (trigger + ACL revocation) so
		// locking "judgement" is unprivileged and vacuous. FOR SHARE OF
		// submission (mutable) prevents a concurrent finalizer from swapping
		// active_judgement_id or active_verdict between this read and the
		// downstream insert.
		&candidate.SubmissionID, &candidate.JudgementID, &candidate.Generation,
		&candidate.ProblemRevisionID, &candidate.RewardFacts, &checksum,
		&candidate.SolvedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select canonical accepted solve: %w", err)
	}
	if len(checksum) != len(candidate.RewardFactsChecksum) {
		return nil, ports.ErrFinalizationConflict
	}
	copy(candidate.RewardFactsChecksum[:], checksum)
	candidate.SolvedAt = candidate.SolvedAt.UTC()
	return &candidate, nil
}

func insertSolveTransition(ctx context.Context, tx *sql.Tx, value entity.SolveTransition, eventID uuid.UUID) error {
	if value.ID == uuid.Nil || eventID == uuid.Nil || value.SolveID == uuid.Nil ||
		value.UserID == uuid.Nil || value.ProblemID == uuid.Nil ||
		value.ToVersion != value.FromVersion+1 || value.OccurredAt.IsZero() ||
		value.CorrelationID == "" {
		return ports.ErrFinalizationConflict
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO user_problem_solve_transitions (
			id, event_id, created_at, solve_id, user_id, problem_id, from_version, to_version,
			from_status, to_status, from_submission_id, to_submission_id,
			from_judgement_id, to_judgement_id, from_generation, to_generation,
			from_problem_revision_id, to_problem_revision_id, from_reward_facts,
			to_reward_facts, from_reward_facts_checksum, to_reward_facts_checksum,
			from_reward_grant_source_id, to_reward_grant_source_id,
			from_reward_provenance, to_reward_provenance, from_solved_at, to_solved_at,
			provenance, occurred_at, correlation_id
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,
			$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31
		)`,
		value.ID, eventID, value.OccurredAt.UTC(), value.SolveID, value.UserID, value.ProblemID,
		value.FromVersion, value.ToVersion, string(value.From.Status), string(value.To.Status),
		sqlUUID(value.From.SubmissionID), sqlUUID(value.To.SubmissionID),
		sqlUUID(value.From.JudgementID), sqlUUID(value.To.JudgementID),
		sqlInt64(value.From.Generation), sqlInt64(value.To.Generation),
		sqlUUID(value.From.ProblemRevisionID), sqlUUID(value.To.ProblemRevisionID),
		sqlJSON(value.From.RewardFacts), sqlJSON(value.To.RewardFacts),
		sqlChecksum(value.From.RewardFactsChecksum), sqlChecksum(value.To.RewardFactsChecksum),
		sqlUUID(value.From.RewardGrantSourceID), sqlUUID(value.To.RewardGrantSourceID),
		sqlRewardProvenance(value.From.RewardProvenance), sqlRewardProvenance(value.To.RewardProvenance),
		sqlTime(value.From.SolvedAt), sqlTime(value.To.SolvedAt), string(value.Provenance),
		value.OccurredAt.UTC(), value.CorrelationID,
	)
	if err != nil {
		return fmt.Errorf("insert solve transition: %w", err)
	}
	return nil
}

func updateSolveProjection(
	ctx context.Context,
	tx *sql.Tx,
	current entity.UserProblemSolve,
	next entity.UserProblemSolve,
	now time.Time,
) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE user_problem_solves
		SET status = $3, active_submission_id = $4, active_judgement_id = $5,
			active_generation = $6, active_problem_revision_id = $7,
			reward_facts = $8, reward_facts_checksum = $9,
			active_reward_transition_id = $10, reward_provenance = $11,
			solved_at = $12, version = $13, updated_at = $14
		WHERE id = $1 AND version = $2`,
		current.ID, current.Version, string(next.Selection.Status),
		sqlUUID(next.Selection.SubmissionID), sqlUUID(next.Selection.JudgementID),
		sqlInt64(next.Selection.Generation), sqlUUID(next.Selection.ProblemRevisionID),
		sqlJSON(next.Selection.RewardFacts), sqlChecksum(next.Selection.RewardFactsChecksum),
		sqlUUID(next.ActiveRewardTransitionID), sqlRewardProvenance(next.Selection.RewardProvenance),
		sqlTime(next.Selection.SolvedAt), next.Version, now.UTC(),
	)
	if err != nil {
		return fmt.Errorf("update solve projection: %w", err)
	}
	affected, affectErr := result.RowsAffected()
	if affectErr != nil || affected != 1 {
		return ports.ErrFinalizationConflict
	}
	return nil
}

func (repository *FinalizationRepository) solveTransitionEvent(
	value entity.SolveTransition,
	eventID uuid.UUID,
) (eventing.Event, error) {
	if eventID == uuid.Nil {
		return eventing.Event{}, ports.ErrFinalizationUnavailable
	}
	payload, err := json.Marshal(submissioncontract.ProblemSolveStatusChangedV1{
		EventID: eventID, TransitionID: value.ID, SolveID: value.SolveID,
		UserID: value.UserID, ProblemID: value.ProblemID,
		From:       solveStateDTO(value.From, value.FromVersion),
		To:         solveStateDTO(value.To, value.ToVersion),
		Provenance: string(value.Provenance), OccurredAt: value.OccurredAt.UTC(),
		CorrelationID: value.CorrelationID,
	})
	if err != nil {
		return eventing.Event{}, fmt.Errorf("marshal solve transition event: %w", err)
	}
	return eventing.Event{
		ID: eventID, Type: submissioncontract.EventProblemSolveStatusChangedV1,
		AggregateType: submissioncontract.AggregateUserProblemSolve,
		AggregateID:   value.SolveID, AggregateVersion: value.ToVersion,
		CorrelationID: value.CorrelationID, Payload: payload, OccurredAt: value.OccurredAt.UTC(),
	}, nil
}

func solveStateDTO(value entity.SolveSelection, version int64) submissioncontract.SolveStateV1 {
	state := submissioncontract.SolveStateV1{
		Version: version, Status: string(value.Status),
		SubmissionID: value.SubmissionID, JudgementID: value.JudgementID,
		Generation: value.Generation, ProblemRevisionID: value.ProblemRevisionID,
		RewardGrantSourceID: value.RewardGrantSourceID, RewardFacts: append(json.RawMessage(nil), value.RewardFacts...),
		SolvedAt: value.SolvedAt,
	}
	if value.RewardProvenance != nil {
		provenance := string(*value.RewardProvenance)
		state.RewardProvenance = &provenance
	}
	if value.RewardFactsChecksum != nil {
		checksum := fmt.Sprintf("%x", value.RewardFactsChecksum[:])
		state.RewardFactsChecksum = &checksum
	}
	return state
}

func uuidPointer(value uuid.UUID) *uuid.UUID                                         { return &value }
func int64Pointer(value int64) *int64                                                { return &value }
func timePointer(value time.Time) *time.Time                                         { return &value }
func rewardProvenancePointer(value entity.RewardProvenance) *entity.RewardProvenance { return &value }
func sqlUUID(value *uuid.UUID) any {
	if value == nil {
		return nil
	}
	return *value
}
func sqlInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
func sqlTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
func sqlJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
func sqlChecksum(value *[32]byte) any {
	if value == nil {
		return nil
	}
	return value[:]
}
func sqlRewardProvenance(value *entity.RewardProvenance) any {
	if value == nil {
		return nil
	}
	return string(*value)
}
