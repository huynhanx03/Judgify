package db

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

var ErrInvalidLifecycleRepository = errors.New("contest lifecycle repository: invalid dependency or command")

type LifecycleRepository struct{ client *dbent.EntClient }

func NewLifecycleRepository(client *dbent.EntClient) (*LifecycleRepository, error) {
	if client == nil {
		return nil, ErrInvalidLifecycleRepository
	}
	return &LifecycleRepository{client: client}, nil
}

var _ ports.LifecycleRepository = (*LifecycleRepository)(nil)

type lockedLifecycleContest struct {
	id                         uuid.UUID
	state                      entity.ContestState
	version                    int64
	startTime                  time.Time
	endTime                    time.Time
	registrationMode           entity.RegistrationMode
	registrationOpensAt        sql.NullTime
	registrationClosesAt       sql.NullTime
	maxParticipants            sql.NullInt64
	nextSequence               int64
	scoringPolicy              entity.ScoringPolicy
	scoringPolicyVersion       int64
	standingsProjectionVersion int64
	rated                      bool
	ratingAlgorithmVersion     string
	freezeStandingsAt          sql.NullTime
}

func (repository *LifecycleRepository) Transition(
	ctx context.Context,
	command ports.TransitionCommand,
) (ports.TransitionResult, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		command.ContestID == uuid.Nil || command.IdempotencyKey == uuid.Nil ||
		(!command.System && command.ActorUserID == uuid.Nil) ||
		(command.System && (command.ActorUserID != uuid.Nil || command.ActorSessionID != nil)) ||
		command.ExpectedVersion < 1 ||
		!command.Target.Valid() || command.CommandHash == ([32]byte{}) {
		return ports.TransitionResult{}, ErrInvalidLifecycleRepository
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return ports.TransitionResult{}, errors.Join(ErrInvalidLifecycleRepository, err)
	}
	client := repository.client.DB(ctx)
	contest, err := lockLifecycleContest(ctx, client, command.ContestID)
	if err != nil {
		return ports.TransitionResult{}, err
	}
	if replay, found, err := lifecycleReplay(ctx, client, command); err != nil {
		return ports.TransitionResult{}, err
	} else if found {
		return replay, nil
	}
	if contest.version != command.ExpectedVersion {
		return ports.TransitionResult{}, entity.ErrContestVersionConflict
	}
	if err := entity.ValidateContestTransition(contest.state, command.Target); err != nil &&
		(!command.System || contest.state != entity.ContestUpcoming || command.Target != entity.ContestEnded) {
		return ports.TransitionResult{}, err
	}
	now, err := lifecycleDatabaseClock(ctx, client)
	if err != nil {
		return ports.TransitionResult{}, err
	}
	if err := validateLockedLifecycleTransition(ctx, client, contest, command.Target, now, command.System); err != nil {
		return ports.TransitionResult{}, err
	}
	transitionID, err := uuid.NewV7()
	if err != nil {
		return ports.TransitionResult{}, fmt.Errorf("create contest transition identity: %w", err)
	}
	nextVersion := contest.version + 1
	var actorUserID any
	if command.System {
		actorUserID = nil
	} else {
		actorUserID = command.ActorUserID
	}
	if _, err := client.Contest.ExecContext(ctx, `
INSERT INTO contest_transitions
    (id, contest_id, command_id, command_hash, version, from_state, to_state,
     actor_user_id, actor_session_id, reason, occurred_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)`,
		transitionID, command.ContestID, command.IdempotencyKey, command.CommandHash[:],
		nextVersion, contest.state, command.Target, actorUserID,
		command.ActorSessionID, command.Reason, now,
	); err != nil {
		return ports.TransitionResult{}, fmt.Errorf("append contest transition: %w", err)
	}
	// Every use of $3 is cast to text. Without the casts PostgreSQL deduces the
	// column type from the SET clause and text from the CASE comparisons, then
	// rejects the statement at prepare time with 42P08 inconsistent types.
	result, err := client.Contest.ExecContext(ctx, `
UPDATE contests
SET status = $3::text, version = $4, updated_at = $5,
    published_at = CASE WHEN $3::text = 'upcoming' THEN $5 ELSE published_at END,
    started_at = CASE WHEN $3::text = 'running' THEN $5 ELSE started_at END,
    ended_at = CASE WHEN $3::text = 'ended' THEN $5 ELSE ended_at END,
    cancelled_at = CASE WHEN $3::text = 'cancelled' THEN $5 ELSE cancelled_at END
WHERE id = $1 AND version = $2 AND status = $6 AND deleted_at IS NULL`,
		command.ContestID, command.ExpectedVersion, command.Target, nextVersion, now, contest.state,
	)
	if err != nil {
		return ports.TransitionResult{}, fmt.Errorf("update contest lifecycle: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return ports.TransitionResult{}, entity.ErrContestVersionConflict
	}
	return ports.TransitionResult{
		ContestID: command.ContestID, From: contest.state, To: command.Target,
		Version: nextVersion, OccurredAt: now, CommandID: command.IdempotencyKey,
	}, nil
}

func lockLifecycleContest(ctx context.Context, client *generate.Client, contestID uuid.UUID) (lockedLifecycleContest, error) {
	rows, err := client.Contest.QueryContext(ctx, `
SELECT id, status, version, start_time, end_time, registration_mode,
       registration_opens_at, registration_closes_at, max_participants,
       next_submission_sequence, scoring_policy, scoring_policy_version,
       standings_projection_version, rated, rating_algorithm_version,
       freeze_standings_at
FROM contests
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE`, contestID)
	if err != nil {
		return lockedLifecycleContest{}, fmt.Errorf("lock contest lifecycle: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return lockedLifecycleContest{}, fmt.Errorf("lock contest lifecycle: %w", err)
		}
		return lockedLifecycleContest{}, entity.ErrContestNotFound
	}
	var result lockedLifecycleContest
	var state, mode, policy string
	if err := rows.Scan(
		&result.id, &state, &result.version, &result.startTime, &result.endTime,
		&mode, &result.registrationOpensAt, &result.registrationClosesAt,
		&result.maxParticipants, &result.nextSequence, &policy,
		&result.scoringPolicyVersion,
		&result.standingsProjectionVersion,
		&result.rated,
		&result.ratingAlgorithmVersion,
		&result.freezeStandingsAt,
	); err != nil {
		return lockedLifecycleContest{}, fmt.Errorf("scan contest lifecycle: %w", err)
	}
	if rows.Next() {
		return lockedLifecycleContest{}, ErrInvalidLifecycleRepository
	}
	result.state = entity.ContestState(state)
	result.registrationMode = entity.RegistrationMode(mode)
	result.scoringPolicy = entity.ScoringPolicy(policy)
	return result, rows.Err()
}

func lifecycleReplay(
	ctx context.Context,
	client *generate.Client,
	command ports.TransitionCommand,
) (ports.TransitionResult, bool, error) {
	rows, err := client.Contest.QueryContext(ctx, `
SELECT command_hash, version, from_state, to_state, occurred_at
FROM contest_transitions
WHERE contest_id = $1 AND command_id = $2`, command.ContestID, command.IdempotencyKey)
	if err != nil {
		return ports.TransitionResult{}, false, fmt.Errorf("read contest transition receipt: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return ports.TransitionResult{}, false, rows.Err()
	}
	var storedHash []byte
	var result ports.TransitionResult
	var from, to string
	if err := rows.Scan(&storedHash, &result.Version, &from, &to, &result.OccurredAt); err != nil {
		return ports.TransitionResult{}, false, fmt.Errorf("scan contest transition receipt: %w", err)
	}
	if rows.Next() {
		return ports.TransitionResult{}, false, ErrInvalidLifecycleRepository
	}
	if len(storedHash) != len(command.CommandHash) {
		return ports.TransitionResult{}, false, ErrInvalidLifecycleRepository
	}
	if subtle.ConstantTimeCompare(storedHash, command.CommandHash[:]) != 1 {
		return ports.TransitionResult{}, false, entity.ErrContestCommandConflict
	}
	result.ContestID = command.ContestID
	result.CommandID = command.IdempotencyKey
	result.From = entity.ContestState(from)
	result.To = entity.ContestState(to)
	result.ExactReplay = true
	return result, true, rows.Err()
}

func lifecycleDatabaseClock(ctx context.Context, client *generate.Client) (time.Time, error) {
	rows, err := client.Contest.QueryContext(ctx, `SELECT clock_timestamp()`)
	if err != nil {
		return time.Time{}, fmt.Errorf("read contest database clock: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return time.Time{}, ErrInvalidLifecycleRepository
	}
	var now time.Time
	if err := rows.Scan(&now); err != nil {
		return time.Time{}, fmt.Errorf("scan contest database clock: %w", err)
	}
	return now.UTC(), rows.Err()
}

func validateLockedLifecycleTransition(
	ctx context.Context,
	client *generate.Client,
	contest lockedLifecycleContest,
	target entity.ContestState,
	now time.Time,
	system bool,
) error {
	if target == entity.ContestUpcoming || target == entity.ContestRunning {
		if !contest.startTime.Before(contest.endTime) || !contest.registrationMode.Valid() ||
			!contest.scoringPolicy.Valid() || contest.scoringPolicyVersion < 1 ||
			contest.standingsProjectionVersion < 1 ||
			contest.ratingAlgorithmVersion !=
				entity.RatingAlgorithmExpectedRankV1 ||
			contest.nextSequence < 1 || (contest.maxParticipants.Valid && contest.maxParticipants.Int64 < 1) ||
			(contest.freezeStandingsAt.Valid && (contest.freezeStandingsAt.Time.Before(contest.startTime) ||
				!contest.freezeStandingsAt.Time.Before(contest.endTime))) {
			return entity.ErrContestPolicy
		}
		if contest.registrationMode == entity.RegistrationModeRegistered {
			if !contest.registrationOpensAt.Valid || !contest.registrationClosesAt.Valid ||
				contest.registrationOpensAt.Time.After(contest.registrationClosesAt.Time) ||
				contest.registrationClosesAt.Time.After(contest.startTime) {
				return entity.ErrContestPolicy
			}
		} else if contest.registrationOpensAt.Valid || contest.registrationClosesAt.Valid {
			return entity.ErrContestPolicy
		}
	}
	switch target {
	case entity.ContestUpcoming:
		if !now.Before(contest.startTime) {
			return entity.ErrContestTransition
		}
		count, err := countContestProblemSnapshots(ctx, client, contest.id)
		if err != nil {
			return err
		}
		if count == 0 {
			count, err = freezeContestProblemSnapshots(ctx, client, contest.id, now)
			if err != nil {
				return err
			}
		}
		if count < 1 {
			return entity.ErrContestProblemSnapshot
		}
	case entity.ContestRunning:
		if now.Before(contest.startTime) || !now.Before(contest.endTime) {
			return entity.ErrContestTransition
		}
	case entity.ContestEnded:
		// A human operator may emergency-end early. A system transition may
		// also reconcile an expired upcoming contest directly to ended, but
		// database time must show that its window has elapsed.
		if system && now.Before(contest.endTime) {
			return entity.ErrContestTransition
		}
	case entity.ContestCancelled:
	default:
		return entity.ErrContestTransition
	}
	return nil
}

func countContestProblemSnapshots(ctx context.Context, client *generate.Client, contestID uuid.UUID) (int64, error) {
	rows, err := client.Contest.QueryContext(ctx, `SELECT COUNT(*) FROM contest_problems WHERE contest_id = $1`, contestID)
	if err != nil {
		return 0, fmt.Errorf("count contest problem snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, ErrInvalidLifecycleRepository
	}
	var count int64
	if err := rows.Scan(&count); err != nil {
		return 0, fmt.Errorf("scan contest problem snapshot count: %w", err)
	}
	return count, rows.Err()
}

type selectedContestProblem struct {
	displayOrder       int
	alias              string
	points             sql.NullInt64
	visibleBeforeStart bool
	problemID          uuid.UUID
	problemRevisionID  uuid.UUID
	testsetRevisionID  uuid.UUID
	checkerKey         string
	checkerVersion     string
	checkerConfig      json.RawMessage
	allowedRuntimeKeys json.RawMessage
	contentChecksum    []byte
}

func freezeContestProblemSnapshots(
	ctx context.Context,
	client *generate.Client,
	contestID uuid.UUID,
	now time.Time,
) (int64, error) {
	countRows, err := client.Contest.QueryContext(ctx, `
SELECT COUNT(*)
FROM contest_draft_problems
WHERE contest_id = $1`,
		contestID,
	)
	if err != nil {
		return 0, fmt.Errorf("count contest draft problem selection: %w", err)
	}
	if !countRows.Next() {
		return 0, errors.Join(entity.ErrContestProblemSnapshot, countRows.Close())
	}
	var selectedCount int
	if err := countRows.Scan(&selectedCount); err != nil {
		return 0, errors.Join(fmt.Errorf(
			"scan contest draft problem selection count: %w",
			err,
		), countRows.Close())
	}
	if countRows.Next() {
		return 0, errors.Join(entity.ErrContestProblemSnapshot, countRows.Close())
	}
	if err := countRows.Err(); err != nil {
		return 0, errors.Join(fmt.Errorf(
			"read contest draft problem selection count: %w",
			err,
		), countRows.Close())
	}
	if err := countRows.Close(); err != nil {
		return 0, fmt.Errorf(
			"close contest draft problem selection count: %w",
			err,
		)
	}
	if selectedCount < 1 ||
		selectedCount > maximumContestProblemProjections {
		return 0, entity.ErrContestProblemSnapshot
	}

	rows, err := client.Contest.QueryContext(ctx, `
SELECT selected.display_order, selected.alias, selected.points,
       selected.visible_before_start,
       p.id, pr.id, pr.testset_revision_id, pr.checker_key, pr.checker_version,
       pr.checker_config, pr.allowed_runtime_keys, pr.content_checksum
FROM contest_draft_problems AS selected
JOIN problems AS p ON p.id = selected.problem_id
JOIN problem_revisions AS pr ON pr.id = p.published_revision_id AND pr.problem_id = p.id
WHERE selected.contest_id = $1 AND p.lifecycle = 'published' AND p.deleted_at IS NULL
ORDER BY selected.display_order, selected.id
LIMIT $2
FOR UPDATE OF selected`,
		contestID,
		maximumContestProblemProjections+1,
	)
	if err != nil {
		return 0, fmt.Errorf("lock contest draft problem selection: %w", err)
	}
	var selected []selectedContestProblem
	for rows.Next() {
		var problem selectedContestProblem
		if err := rows.Scan(
			&problem.displayOrder, &problem.alias, &problem.points,
			&problem.visibleBeforeStart,
			&problem.problemID, &problem.problemRevisionID, &problem.testsetRevisionID,
			&problem.checkerKey, &problem.checkerVersion, &problem.checkerConfig,
			&problem.allowedRuntimeKeys, &problem.contentChecksum,
		); err != nil {
			return 0, errors.Join(fmt.Errorf("scan contest draft problem selection: %w", err), rows.Close())
		}
		selected = append(selected, problem)
		if len(selected) > maximumContestProblemProjections {
			return 0, errors.Join(entity.ErrContestProblemSnapshot, rows.Close())
		}
	}
	if err := rows.Err(); err != nil {
		return 0, errors.Join(fmt.Errorf("read contest draft problem selection: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close contest draft problem selection: %w", err)
	}
	if len(selected) != selectedCount {
		return 0, entity.ErrContestProblemSnapshot
	}

	for index, problem := range selected {
		if !validSelectedContestProblem(problem) ||
			problem.displayOrder != index+1 {
			return 0, entity.ErrContestProblemSnapshot
		}
		id, err := uuid.NewV7()
		if err != nil {
			return 0, fmt.Errorf("create contest problem snapshot identity: %w", err)
		}
		checksum := sha256.New()
		writeContestProblemChecksumPart(
			checksum,
			[]byte("judgify.contest-problem-snapshot.v3"),
		)
		writeContestProblemChecksumPart(checksum, contestID[:])
		writeContestProblemChecksumPart(checksum, problem.problemID[:])
		writeContestProblemChecksumPart(
			checksum,
			problem.problemRevisionID[:],
		)
		writeContestProblemChecksumPart(
			checksum,
			problem.testsetRevisionID[:],
		)
		writeContestProblemChecksumPart(checksum, problem.contentChecksum)
		writeContestProblemChecksumPart(
			checksum,
			[]byte(problem.checkerKey),
		)
		writeContestProblemChecksumPart(
			checksum,
			[]byte(problem.checkerVersion),
		)
		writeContestProblemChecksumPart(checksum, problem.checkerConfig)
		writeContestProblemChecksumPart(
			checksum,
			problem.allowedRuntimeKeys,
		)
		var displayOrder [8]byte
		binary.BigEndian.PutUint64(
			displayOrder[:],
			uint64(problem.displayOrder),
		)
		writeContestProblemChecksumPart(
			checksum,
			displayOrder[:],
		)
		writeContestProblemChecksumPart(
			checksum,
			[]byte(problem.alias),
		)
		var encodedPoints []byte
		var storedPoints any
		if problem.points.Valid {
			encodedPoints = make([]byte, 8)
			binary.BigEndian.PutUint64(
				encodedPoints,
				uint64(problem.points.Int64),
			)
			storedPoints = problem.points.Int64
		}
		writeContestProblemChecksumPart(checksum, encodedPoints)
		writeContestProblemChecksumPart(checksum, []byte(`{}`))
		visibleBeforeStart := byte(0)
		if problem.visibleBeforeStart {
			visibleBeforeStart = 1
		}
		writeContestProblemChecksumPart(
			checksum,
			[]byte{visibleBeforeStart},
		)
		if _, err := client.Contest.ExecContext(ctx, `
INSERT INTO contest_problems
    (id, contest_id, problem_id, problem_revision_id, testset_revision_id,
     display_order, alias, points, penalty_policy, visible_before_start,
     checker_key, checker_version, checker_config, allowed_runtime_keys,
     snapshot_checksum, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '{}'::jsonb, $9,
        $10, $11, $12, $13, $14, $15)`,
			id, contestID, problem.problemID, problem.problemRevisionID,
			problem.testsetRevisionID, problem.displayOrder, problem.alias,
			storedPoints, problem.visibleBeforeStart,
			problem.checkerKey, problem.checkerVersion, problem.checkerConfig,
			problem.allowedRuntimeKeys, checksum.Sum(nil), now,
		); err != nil {
			return 0, fmt.Errorf("freeze contest problem snapshot: %w", err)
		}
	}
	return int64(len(selected)), nil
}

func validSelectedContestProblem(problem selectedContestProblem) bool {
	if problem.displayOrder < 1 ||
		!contestProblemAliasPattern.MatchString(problem.alias) ||
		(problem.points.Valid && problem.points.Int64 < 1) ||
		problem.problemID == uuid.Nil ||
		problem.problemRevisionID == uuid.Nil ||
		problem.testsetRevisionID == uuid.Nil ||
		problem.checkerKey == "" ||
		problem.checkerVersion == "" ||
		len(problem.contentChecksum) != sha256.Size {
		return false
	}
	nonzeroChecksum := false
	for _, value := range problem.contentChecksum {
		if value != 0 {
			nonzeroChecksum = true
			break
		}
	}
	var checkerConfig map[string]any
	var runtimeKeys []string
	if !nonzeroChecksum ||
		json.Unmarshal(problem.checkerConfig, &checkerConfig) != nil ||
		checkerConfig == nil ||
		json.Unmarshal(problem.allowedRuntimeKeys, &runtimeKeys) != nil ||
		len(runtimeKeys) < 1 ||
		len(runtimeKeys) > 32 {
		return false
	}
	seen := make(map[string]struct{}, len(runtimeKeys))
	for _, runtimeKey := range runtimeKeys {
		if !contestRuntimeKeyPattern.MatchString(runtimeKey) {
			return false
		}
		if _, exists := seen[runtimeKey]; exists {
			return false
		}
		seen[runtimeKey] = struct{}{}
	}
	return true
}

func writeContestProblemChecksumPart(
	checksum hash.Hash,
	value []byte,
) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = checksum.Write(length[:])
	_, _ = checksum.Write(value)
}
