package db

import (
	"context"
	"crypto/subtle"
	"database/sql"
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

var ErrInvalidRegistrationCommandRepository = errors.New("contest registration command repository: invalid dependency or command")

type RegistrationCommandRepository struct{ client *dbent.EntClient }

func NewRegistrationCommandRepository(client *dbent.EntClient) (*RegistrationCommandRepository, error) {
	if client == nil {
		return nil, ErrInvalidRegistrationCommandRepository
	}
	return &RegistrationCommandRepository{client: client}, nil
}

var _ ports.RegistrationCommandRepository = (*RegistrationCommandRepository)(nil)

type lockedRegistrationContest struct {
	state           entity.ContestState
	version         int64
	mode            entity.RegistrationMode
	opensAt         sql.NullTime
	closesAt        sql.NullTime
	maxParticipants sql.NullInt64
}

func (repository *RegistrationCommandRepository) Apply(
	ctx context.Context,
	command ports.RegistrationCommand,
	mutation ports.RegistrationMutation,
) (ports.RegistrationResult, error) {
	if repository == nil || repository.client == nil || ctx == nil || command.ContestID == uuid.Nil ||
		command.UserID == uuid.Nil || command.IdempotencyKey == uuid.Nil || command.CommandHash == ([32]byte{}) {
		return ports.RegistrationResult{}, ErrInvalidRegistrationCommandRepository
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return ports.RegistrationResult{}, errors.Join(ErrInvalidRegistrationCommandRepository, err)
	}
	client := repository.client.DB(ctx)
	contest, err := lockRegistrationCommandContest(ctx, client, command.ContestID)
	if err != nil {
		return ports.RegistrationResult{}, err
	}
	if replay, found, err := registrationReplay(ctx, client, command); err != nil {
		return ports.RegistrationResult{}, err
	} else if found {
		return replay, nil
	}
	if contest.version != command.ExpectedContestVersion {
		return ports.RegistrationResult{}, entity.ErrContestVersionConflict
	}
	registration, err := lockContestRegistration(ctx, client, command.ContestID, command.UserID)
	if err != nil {
		return ports.RegistrationResult{}, err
	}
	currentVersion := int64(0)
	if registration.exists {
		currentVersion = registration.version
	}
	if command.ExpectedRegistrationVersion != nil && *command.ExpectedRegistrationVersion != currentVersion {
		return ports.RegistrationResult{}, entity.ErrContestVersionConflict
	}
	if registration.exists && registration.status == entity.RegistrationBanned {
		return ports.RegistrationResult{}, entity.ErrContestRegistrationBanned
	}
	now, err := registrationDatabaseClock(ctx, client)
	if err != nil {
		return ports.RegistrationResult{}, err
	}
	if err := validateRegistrationMutation(ctx, client, contest, registration, command, mutation, now); err != nil {
		return ports.RegistrationResult{}, err
	}

	next := buildRegistrationMutation(registration, command, mutation, now)
	if next.ID == uuid.Nil {
		next.ID, err = uuid.NewV7()
		if err != nil {
			return ports.RegistrationResult{}, fmt.Errorf("create registration identity: %w", err)
		}
	}
	transitionID, err := uuid.NewV7()
	if err != nil {
		return ports.RegistrationResult{}, fmt.Errorf("create registration transition identity: %w", err)
	}
	fromStatus := "absent"
	if registration.exists {
		fromStatus = string(registration.status)
		if err := appendRegistrationTransition(ctx, client, transitionID, command, mutation, fromStatus, next); err != nil {
			return ports.RegistrationResult{}, err
		}
		result, err := client.Contest.ExecContext(ctx, `
UPDATE contest_registrations
SET status = $4, source = $5, version = $6, official_submission_count = $7,
    registered_at = COALESCE($8, registered_at), withdrawn_at = $9, banned_at = $10, reason = $11, updated_at = $12
WHERE id = $1 AND contest_id = $2 AND user_id = $3 AND version = $13`,
			next.ID, next.ContestID, next.UserID, next.Status, next.Source, next.Version,
			next.OfficialSubmissionCount, next.RegisteredAt, next.WithdrawnAt, next.BannedAt,
			next.Reason, now, registration.version,
		)
		if err != nil {
			return ports.RegistrationResult{}, fmt.Errorf("update contest registration: %w", err)
		}
		updated, err := result.RowsAffected()
		if err != nil || updated != 1 {
			return ports.RegistrationResult{}, entity.ErrContestVersionConflict
		}
	} else {
		if _, err := client.Contest.ExecContext(ctx, `
INSERT INTO contest_registrations
    (id, contest_id, user_id, status, source, version, official_submission_count,
     registered_at, withdrawn_at, banned_at, reason, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)`,
			next.ID, next.ContestID, next.UserID, next.Status, next.Source, next.Version,
			next.OfficialSubmissionCount, next.RegisteredAt, next.WithdrawnAt, next.BannedAt,
			next.Reason, now,
		); err != nil {
			return ports.RegistrationResult{}, fmt.Errorf("insert contest registration: %w", err)
		}
		if err := appendRegistrationTransition(ctx, client, transitionID, command, mutation, fromStatus, next); err != nil {
			return ports.RegistrationResult{}, err
		}
	}
	return ports.RegistrationResult{
		Registration: next, ContestVersion: contest.version, OccurredAt: now,
		CommandID: command.IdempotencyKey,
	}, nil
}

func lockRegistrationCommandContest(ctx context.Context, client *generate.Client, contestID uuid.UUID) (lockedRegistrationContest, error) {
	rows, err := client.Contest.QueryContext(ctx, `
SELECT status, version, registration_mode, registration_opens_at,
       registration_closes_at, max_participants
FROM contests
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE`, contestID)
	if err != nil {
		return lockedRegistrationContest{}, fmt.Errorf("lock contest for registration: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return lockedRegistrationContest{}, entity.ErrContestNotFound
	}
	var result lockedRegistrationContest
	var state, mode string
	if err := rows.Scan(&state, &result.version, &mode, &result.opensAt, &result.closesAt, &result.maxParticipants); err != nil {
		return lockedRegistrationContest{}, fmt.Errorf("scan contest registration policy: %w", err)
	}
	result.state, result.mode = entity.ContestState(state), entity.RegistrationMode(mode)
	return result, rows.Err()
}

func validateRegistrationMutation(
	ctx context.Context,
	client *generate.Client,
	contest lockedRegistrationContest,
	registration lockedRegistration,
	command ports.RegistrationCommand,
	mutation ports.RegistrationMutation,
	now time.Time,
) error {
	if mutation == ports.RegistrationBan {
		if contest.state == entity.ContestEnded || contest.state == entity.ContestCancelled {
			return entity.ErrContestRegistrationTransition
		}
		return nil
	}
	if contest.state != entity.ContestUpcoming || contest.mode != entity.RegistrationModeRegistered ||
		!contest.opensAt.Valid || !contest.closesAt.Valid || now.Before(contest.opensAt.Time) || now.After(contest.closesAt.Time) {
		return entity.ErrContestRegistrationMode
	}
	switch mutation {
	case ports.RegistrationRegister:
		if registration.exists && registration.status != entity.RegistrationWithdrawn {
			return entity.ErrContestRegistrationTransition
		}
		if contest.maxParticipants.Valid {
			count, err := countActiveContestRegistrations(ctx, client, command.ContestID)
			if err != nil {
				return err
			}
			if count >= contest.maxParticipants.Int64 {
				return entity.ErrContestRegistrationFull
			}
		}
	case ports.RegistrationWithdraw:
		if !registration.exists || registration.status != entity.RegistrationRegistered {
			return entity.ErrContestRegistrationTransition
		}
		if registration.officialSubmissionCount > 0 {
			return entity.ErrContestSubmissionExists
		}
	default:
		return entity.ErrContestRegistrationTransition
	}
	return nil
}

func buildRegistrationMutation(
	current lockedRegistration,
	command ports.RegistrationCommand,
	mutation ports.RegistrationMutation,
	now time.Time,
) entity.ContestRegistration {
	result := entity.ContestRegistration{
		ID: current.id, ContestID: command.ContestID, UserID: command.UserID,
		Version: current.version + 1, OfficialSubmissionCount: current.officialSubmissionCount,
		CreatedAt: now, UpdatedAt: now,
	}
	switch mutation {
	case ports.RegistrationRegister:
		result.Status, result.Source, result.RegisteredAt = entity.RegistrationRegistered, entity.RegistrationSourceExplicit, &now
	case ports.RegistrationWithdraw:
		// A withdrawal records when access ended without rewriting the
		// original admission time. The SQL update preserves registered_at
		// whenever this command leaves it unset.
		result.Status, result.Source, result.WithdrawnAt = entity.RegistrationWithdrawn, current.source, &now
	case ports.RegistrationBan:
		reason := command.Reason
		result.Status, result.Source, result.BannedAt, result.Reason = entity.RegistrationBanned, entity.RegistrationSourceAdmin, &now, &reason
	}
	return result
}

func appendRegistrationTransition(
	ctx context.Context,
	client *generate.Client,
	transitionID uuid.UUID,
	command ports.RegistrationCommand,
	mutation ports.RegistrationMutation,
	fromStatus string,
	next entity.ContestRegistration,
) error {
	_, err := client.Contest.ExecContext(ctx, `
INSERT INTO contest_registration_transitions
    (id, contest_id, registration_id, user_id, command_id, command_hash,
     registration_version, official_submission_count, mutation, from_status, to_status, source,
     actor_user_id, actor_session_id, reason, occurred_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $16)`,
		transitionID, command.ContestID, next.ID, command.UserID, command.IdempotencyKey,
		command.CommandHash[:], next.Version, next.OfficialSubmissionCount, mutation, fromStatus,
		next.Status, next.Source, command.ActorUserID, command.ActorSessionID, next.Reason, next.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("append contest registration transition: %w", err)
	}
	return nil
}

func registrationReplay(
	ctx context.Context,
	client *generate.Client,
	command ports.RegistrationCommand,
) (ports.RegistrationResult, bool, error) {
	rows, err := client.Contest.QueryContext(ctx, `
SELECT registration_id, command_hash, registration_version, official_submission_count,
       to_status, source, occurred_at
FROM contest_registration_transitions
WHERE contest_id = $1 AND user_id = $2 AND command_id = $3`,
		command.ContestID, command.UserID, command.IdempotencyKey,
	)
	if err != nil {
		return ports.RegistrationResult{}, false, fmt.Errorf("read registration command receipt: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return ports.RegistrationResult{}, false, rows.Err()
	}
	var storedHash []byte
	var status, source string
	var result ports.RegistrationResult
	result.Registration.ContestID, result.Registration.UserID = command.ContestID, command.UserID
	if err := rows.Scan(
		&result.Registration.ID, &storedHash, &result.Registration.Version,
		&result.Registration.OfficialSubmissionCount, &status, &source, &result.OccurredAt,
	); err != nil {
		return ports.RegistrationResult{}, false, fmt.Errorf("scan registration command receipt: %w", err)
	}
	if rows.Next() || subtle.ConstantTimeCompare(storedHash, command.CommandHash[:]) != 1 {
		return ports.RegistrationResult{}, false, entity.ErrContestCommandConflict
	}
	result.Registration.Status = entity.RegistrationStatus(status)
	result.Registration.Source = entity.RegistrationSource(source)
	result.ContestVersion = command.ExpectedContestVersion
	result.CommandID, result.ExactReplay = command.IdempotencyKey, true
	return result, true, rows.Err()
}
