package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

var ErrInvalidSubmissionReservationRepository = errors.New("contest submission reservation: invalid dependency or request")

// SubmissionReservationRepository is a narrow Contest-owned transaction
// participant. It never opens a transaction; submission intake supplies the
// active UnitOfWork after taking Identity and Submission locks.
type SubmissionReservationRepository struct {
	client *dbent.EntClient
}

func NewSubmissionReservationRepository(client *dbent.EntClient) (*SubmissionReservationRepository, error) {
	if client == nil {
		return nil, ErrInvalidSubmissionReservationRepository
	}
	return &SubmissionReservationRepository{client: client}, nil
}

var _ ports.ContestSubmissionReserver = (*SubmissionReservationRepository)(nil)

type lockedContestAdmission struct {
	id                   uuid.UUID
	state                entity.ContestState
	startTime            time.Time
	endTime              time.Time
	registrationMode     entity.RegistrationMode
	maxParticipants      sql.NullInt64
	nextSequence         int64
	scoringPolicy        entity.ScoringPolicy
	scoringPolicyVersion int64
}

type lockedRegistration struct {
	id                      uuid.UUID
	status                  entity.RegistrationStatus
	source                  entity.RegistrationSource
	version                 int64
	officialSubmissionCount int64
	exists                  bool
}

func (repository *SubmissionReservationRepository) ReserveContestSubmission(
	ctx context.Context,
	request ports.ReserveContestSubmissionRequest,
) (ports.ContestSubmissionReservation, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		request.ContestID == uuid.Nil || request.ContestProblemID == uuid.Nil ||
		request.ProblemID == uuid.Nil || request.UserID == uuid.Nil || request.RuntimeKey == "" {
		return ports.ContestSubmissionReservation{}, ErrInvalidSubmissionReservationRepository
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return ports.ContestSubmissionReservation{}, errors.Join(ErrInvalidSubmissionReservationRepository, err)
	}
	client := repository.client.DB(ctx)

	// Global Contest lock order: aggregate, participant, problem snapshot. No
	// database clock read is allowed before these potentially blocking locks.
	contest, err := lockContestAdmission(ctx, client, request.ContestID)
	if err != nil {
		return ports.ContestSubmissionReservation{}, err
	}
	registration, err := lockContestRegistration(ctx, client, request.ContestID, request.UserID)
	if err != nil {
		return ports.ContestSubmissionReservation{}, err
	}
	problem, err := lockContestProblem(ctx, client, request)
	if err != nil {
		return ports.ContestSubmissionReservation{}, err
	}
	now, err := registrationDatabaseClock(ctx, client)
	if err != nil {
		return ports.ContestSubmissionReservation{}, err
	}

	aggregate := entity.Contest{
		ID: contest.id, State: contest.state, StartTime: contest.startTime, EndTime: contest.endTime,
	}
	if !aggregate.AcceptsOfficialSubmissionAt(now) {
		return ports.ContestSubmissionReservation{}, entity.ErrContestNotRunning
	}
	if !runtimeAllowed(problem.AllowedRuntimeKeys, request.RuntimeKey) {
		return ports.ContestSubmissionReservation{}, entity.ErrContestProblemUnavailable
	}
	implicitRegistration, err := repository.ensureParticipant(ctx, client, contest, registration, request.UserID, now)
	if err != nil {
		return ports.ContestSubmissionReservation{}, err
	}
	if err := incrementOfficialSubmissionCount(ctx, client, contest.id, request.UserID, now); err != nil {
		return ports.ContestSubmissionReservation{}, err
	}
	sequence, err := reserveContestSequence(ctx, client, contest.id, now)
	if err != nil {
		return ports.ContestSubmissionReservation{}, err
	}
	if sequence < 1 || sequence != contest.nextSequence {
		return ports.ContestSubmissionReservation{}, entity.ErrContestSequence
	}

	elapsed := now.Sub(contest.startTime).Milliseconds()
	if elapsed < 0 {
		elapsed = 0
	}
	return ports.ContestSubmissionReservation{
		ContestID: contest.id, ContestProblemID: problem.ID, ProblemID: problem.ProblemID,
		ProblemRevisionID: problem.ProblemRevisionID, TestsetRevisionID: problem.TestsetRevisionID,
		ScoringPolicy: contest.scoringPolicy, ScoringPolicyVersion: contest.scoringPolicyVersion,
		Lane: "contest", SubmittedAt: now, SubmissionElapsedMillis: elapsed,
		ContestSubmissionSequence: sequence, CheckerKey: problem.CheckerKey,
		CheckerVersion:                 problem.CheckerVersion,
		CheckerConfig:                  append(json.RawMessage(nil), problem.CheckerConfig...),
		AllowedRuntimeKeys:             append([]string(nil), problem.AllowedRuntimeKeys...),
		ProblemSnapshotChecksum:        problem.SnapshotChecksum,
		ImplicitRegistrationTransition: implicitRegistration,
	}, nil
}

func lockContestAdmission(ctx context.Context, client *generate.Client, contestID uuid.UUID) (lockedContestAdmission, error) {
	rows, err := client.Contest.QueryContext(ctx, `
SELECT id, status, start_time, end_time, registration_mode, max_participants,
       next_submission_sequence, scoring_policy, scoring_policy_version
FROM contests
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE`, contestID)
	if err != nil {
		return lockedContestAdmission{}, fmt.Errorf("lock contest admission: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return lockedContestAdmission{}, fmt.Errorf("lock contest admission: %w", err)
		}
		return lockedContestAdmission{}, entity.ErrContestNotFound
	}
	var result lockedContestAdmission
	if err := rows.Scan(
		&result.id, &result.state, &result.startTime, &result.endTime,
		&result.registrationMode, &result.maxParticipants, &result.nextSequence,
		&result.scoringPolicy, &result.scoringPolicyVersion,
	); err != nil {
		return lockedContestAdmission{}, fmt.Errorf("scan contest admission: %w", err)
	}
	if rows.Next() {
		return lockedContestAdmission{}, ErrInvalidSubmissionReservationRepository
	}
	return result, rows.Err()
}

func lockContestRegistration(
	ctx context.Context,
	client *generate.Client,
	contestID uuid.UUID,
	userID uuid.UUID,
) (lockedRegistration, error) {
	rows, err := client.Contest.QueryContext(ctx, `
SELECT id, status, source, version, official_submission_count
FROM contest_registrations
WHERE contest_id = $1 AND user_id = $2
FOR UPDATE`, contestID, userID)
	if err != nil {
		return lockedRegistration{}, fmt.Errorf("lock contest registration: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return lockedRegistration{}, fmt.Errorf("lock contest registration: %w", err)
		}
		return lockedRegistration{}, nil
	}
	var result lockedRegistration
	if err := rows.Scan(&result.id, &result.status, &result.source, &result.version, &result.officialSubmissionCount); err != nil {
		return lockedRegistration{}, fmt.Errorf("scan contest registration: %w", err)
	}
	result.exists = true
	if rows.Next() {
		return lockedRegistration{}, ErrInvalidSubmissionReservationRepository
	}
	return result, rows.Err()
}

func lockContestProblem(
	ctx context.Context,
	client *generate.Client,
	request ports.ReserveContestSubmissionRequest,
) (entity.ContestProblem, error) {
	// contest_problems is append-only: the trigger and ACL revocation prevent any
	// UPDATE or DELETE, so FOR KEY SHARE would guard against mutations that can
	// never occur. The plain SELECT is sufficient — the snapshot is immutable by
	// construction once the contest is published.
	rows, err := client.Contest.QueryContext(ctx, `
SELECT id, contest_id, problem_id, problem_revision_id, testset_revision_id,
       checker_key, checker_version, checker_config, allowed_runtime_keys, snapshot_checksum
FROM contest_problems
WHERE id = $1 AND contest_id = $2 AND problem_id = $3`, request.ContestProblemID, request.ContestID, request.ProblemID)
	if err != nil {
		return entity.ContestProblem{}, fmt.Errorf("lock contest problem snapshot: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return entity.ContestProblem{}, fmt.Errorf("lock contest problem snapshot: %w", err)
		}
		return entity.ContestProblem{}, entity.ErrContestProblemUnavailable
	}
	var result entity.ContestProblem
	var runtimes, checksum []byte
	if err := rows.Scan(
		&result.ID, &result.ContestID, &result.ProblemID, &result.ProblemRevisionID,
		&result.TestsetRevisionID, &result.CheckerKey, &result.CheckerVersion,
		&result.CheckerConfig, &runtimes, &checksum,
	); err != nil {
		return entity.ContestProblem{}, fmt.Errorf("scan contest problem snapshot: %w", err)
	}
	if rows.Next() || len(checksum) != len(result.SnapshotChecksum) || json.Unmarshal(runtimes, &result.AllowedRuntimeKeys) != nil {
		return entity.ContestProblem{}, ErrInvalidSubmissionReservationRepository
	}
	copy(result.SnapshotChecksum[:], checksum)
	return result, rows.Err()
}

func (repository *SubmissionReservationRepository) ensureParticipant(
	ctx context.Context,
	client *generate.Client,
	contest lockedContestAdmission,
	registration lockedRegistration,
	userID uuid.UUID,
	now time.Time,
) (*entity.ContestRegistrationTransition, error) {
	if registration.exists && registration.status == entity.RegistrationBanned {
		return nil, entity.ErrContestRegistrationBanned
	}
	if contest.registrationMode == entity.RegistrationModeRegistered {
		if !registration.exists || registration.status != entity.RegistrationRegistered {
			return nil, entity.ErrContestRegistrationRequired
		}
		return nil, nil
	}
	if contest.registrationMode != entity.RegistrationModeOpen {
		return nil, entity.ErrContestPolicy
	}
	if registration.exists && registration.status == entity.RegistrationRegistered {
		return nil, nil
	}
	if contest.maxParticipants.Valid {
		count, err := countActiveContestRegistrations(ctx, client, contest.id)
		if err != nil {
			return nil, err
		}
		if count >= contest.maxParticipants.Int64 {
			return nil, entity.ErrContestRegistrationFull
		}
	}
	registrationID := registration.id
	if registrationID == uuid.Nil {
		var err error
		registrationID, err = uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("create open registration identity: %w", err)
		}
	}
	transitionID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("create open registration transition identity: %w", err)
	}
	commandID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("create open registration command identity: %w", err)
	}
	commandHash := sha256.Sum256([]byte("judgify.implicit-open-registration.v1\x00" + contest.id.String() + "\x00" + userID.String() + "\x00" + now.UTC().Format(time.RFC3339Nano)))
	nextVersion := registration.version + 1
	next := entity.ContestRegistration{
		ID: registrationID, ContestID: contest.id, UserID: userID,
		Status: entity.RegistrationRegistered, Source: entity.RegistrationSourceImplicitOpen,
		Version: nextVersion, OfficialSubmissionCount: registration.officialSubmissionCount,
		RegisteredAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	command := ports.RegistrationCommand{
		ContestID: contest.id, UserID: userID, IdempotencyKey: commandID,
		ActorUserID: userID, CommandHash: commandHash,
	}
	fromStatus := "absent"
	if registration.exists {
		fromStatus = string(registration.status)
		if err := appendRegistrationTransition(ctx, client, transitionID, command, ports.RegistrationRegister, fromStatus, next); err != nil {
			return nil, err
		}
		result, err := client.Contest.ExecContext(ctx, `
UPDATE contest_registrations
SET status = 'registered', source = 'implicit_open', version = version + 1,
    registered_at = $3, withdrawn_at = NULL, banned_at = NULL, reason = NULL,
    updated_at = $3
WHERE contest_id = $1 AND user_id = $2 AND status = 'withdrawn'`, contest.id, userID, now)
		if err != nil {
			return nil, fmt.Errorf("reactivate open contest registration: %w", err)
		}
		updated, err := result.RowsAffected()
		if err != nil || updated != 1 {
			return nil, entity.ErrContestRegistrationTransition
		}
	} else {
		if _, err := client.Contest.ExecContext(ctx, `
INSERT INTO contest_registrations
    (id, contest_id, user_id, status, source, version, official_submission_count,
     registered_at, created_at, updated_at)
VALUES ($1, $2, $3, 'registered', 'implicit_open', 1, 0, $4, $4, $4)`,
			registrationID, contest.id, userID, now,
		); err != nil {
			return nil, fmt.Errorf("reserve open contest registration: %w", err)
		}
		if err := appendRegistrationTransition(ctx, client, transitionID, command, ports.RegistrationRegister, fromStatus, next); err != nil {
			return nil, err
		}
	}
	return &entity.ContestRegistrationTransition{
		ID: transitionID, ContestID: contest.id, RegistrationID: registrationID,
		UserID: userID, CommandID: commandID, CommandHash: commandHash,
		RegistrationVersion: nextVersion, Mutation: string(ports.RegistrationRegister),
		FromStatus: fromStatus, ToStatus: entity.RegistrationRegistered,
		Source: entity.RegistrationSourceImplicitOpen, ActorUserID: &userID, OccurredAt: now,
	}, nil
}

func countActiveContestRegistrations(ctx context.Context, client *generate.Client, contestID uuid.UUID) (int64, error) {
	rows, err := client.Contest.QueryContext(ctx, `
SELECT COUNT(*)
FROM contest_registrations
WHERE contest_id = $1 AND status = 'registered'`, contestID)
	if err != nil {
		return 0, fmt.Errorf("count active contest registrations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, ErrInvalidSubmissionReservationRepository
	}
	var count int64
	if err := rows.Scan(&count); err != nil {
		return 0, fmt.Errorf("scan active contest registrations: %w", err)
	}
	return count, rows.Err()
}

func reserveContestSequence(ctx context.Context, client *generate.Client, contestID uuid.UUID, now time.Time) (int64, error) {
	rows, err := client.Contest.QueryContext(ctx, `
UPDATE contests
SET next_submission_sequence = next_submission_sequence + 1,
    updated_at = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING next_submission_sequence - 1 AS reserved_sequence`, contestID, now)
	if err != nil {
		return 0, fmt.Errorf("reserve contest submission sequence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, entity.ErrContestNotFound
	}
	var sequence int64
	if err := rows.Scan(&sequence); err != nil {
		return 0, fmt.Errorf("scan contest submission sequence: %w", err)
	}
	return sequence, rows.Err()
}

func incrementOfficialSubmissionCount(
	ctx context.Context,
	client *generate.Client,
	contestID uuid.UUID,
	userID uuid.UUID,
	now time.Time,
) error {
	result, err := client.Contest.ExecContext(ctx, `
UPDATE contest_registrations
SET official_submission_count = official_submission_count + 1,
    updated_at = $3
WHERE contest_id = $1 AND user_id = $2 AND status = 'registered'`, contestID, userID, now)
	if err != nil {
		return fmt.Errorf("record official contest submission: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return entity.ErrContestRegistrationRequired
	}
	return nil
}

func runtimeAllowed(values []string, requested string) bool {
	for _, value := range values {
		if value == requested {
			return true
		}
	}
	return false
}
