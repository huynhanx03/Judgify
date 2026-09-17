package db

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

type IntakeGate struct {
	client               *dbent.EntClient
	maximumActivePerUser int
}

func NewIntakeGate(
	client *dbent.EntClient,
	maximumActivePerUser int,
) (*IntakeGate, error) {
	if client == nil || maximumActivePerUser < 1 {
		return nil, intake.ErrAdmissionDenied
	}
	return &IntakeGate{
		client:               client,
		maximumActivePerUser: maximumActivePerUser,
	}, nil
}

func (gate *IntakeGate) LockAndReplay(ctx context.Context, identity intake.CommandIdentity) (*intake.Result, error) {
	if gate == nil || gate.client == nil || ctx == nil ||
		identity.UserID == uuid.Nil ||
		intake.ValidateIdempotencyKey(identity.IdempotencyKey) != nil ||
		identity.RequestHash == ([32]byte{}) {
		return nil, intake.ErrInvalidCommand
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return nil, err
	}
	client := gate.client.DB(ctx)
	// PostgreSQL text values cannot contain NUL bytes. The user UUID has one
	// canonical fixed-width representation, so a colon at byte 36 gives this
	// advisory-lock preimage an unambiguous, database-safe boundary even when
	// the opaque idempotency key itself contains punctuation.
	lockPreimage := identity.UserID.String() + ":" + identity.IdempotencyKey
	lockRows, err := client.Submission.QueryContext(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 7415105))`,
		lockPreimage,
	)
	if err != nil {
		return nil, fmt.Errorf("lock submission command: %w", err)
	}
	if !lockRows.Next() {
		if err := lockRows.Err(); err != nil {
			return nil, errors.Join(fmt.Errorf("lock submission command: %w", err), lockRows.Close())
		}
		return nil, errors.Join(intake.ErrPersistence, lockRows.Close())
	}
	if err := lockRows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("lock submission command: %w", err), lockRows.Close())
	}
	if err := lockRows.Close(); err != nil {
		return nil, fmt.Errorf("close submission lock: %w", err)
	}
	rows, err := client.Submission.QueryContext(ctx, `
SELECT id, request_hash, submitted_at, contest_submission_sequence, origin_cid
FROM submissions WHERE user_id = $1 AND idempotency_key = $2`, identity.UserID, identity.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("read submission command receipt: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var storedHash []byte
	var contestSequence *int64
	result := intake.Result{
		Generation:       1,
		Phase:            entity.PhaseQueued,
		AggregateVersion: 1,
		ExactReplay:      true,
	}
	if err := rows.Scan(
		&result.SubmissionID,
		&storedHash,
		&result.SubmittedAt,
		&contestSequence,
		&result.CorrelationID,
	); err != nil {
		return nil, fmt.Errorf("scan submission command receipt: %w", err)
	}
	if rows.Next() {
		return nil, intake.ErrPersistence
	}
	if len(storedHash) != len(identity.RequestHash) {
		return nil, intake.ErrPersistence
	}
	if subtle.ConstantTimeCompare(storedHash, identity.RequestHash[:]) != 1 {
		return nil, intake.ErrIdempotencyConflict
	}
	result.ContestSequence = contestSequence
	return &result, rows.Err()
}

func (gate *IntakeGate) Reserve(ctx context.Context, userID uuid.UUID) error {
	if gate == nil || gate.client == nil || ctx == nil || userID == uuid.Nil {
		return intake.ErrAdmissionDenied
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return err
	}
	lockRows, err := client.Submission.QueryContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 7415106))`, userID.String())
	if err != nil {
		return fmt.Errorf("lock user submission admission: %w", err)
	}
	if !lockRows.Next() {
		if err := lockRows.Err(); err != nil {
			return errors.Join(fmt.Errorf("lock user submission admission: %w", err), lockRows.Close())
		}
		return errors.Join(intake.ErrAdmissionDenied, lockRows.Close())
	}
	if err := lockRows.Err(); err != nil {
		return errors.Join(fmt.Errorf("lock user submission admission: %w", err), lockRows.Close())
	}
	if err := lockRows.Close(); err != nil {
		return fmt.Errorf("close user submission lock: %w", err)
	}
	rows, err := client.Submission.QueryContext(ctx, `SELECT COUNT(*) FROM submissions WHERE user_id = $1 AND latest_generation_phase IN ('queued','claimed','compiling','running','finalizing')`, userID)
	if err != nil {
		return fmt.Errorf("count active user submissions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fmt.Errorf("count active user submissions: %w", err)
		}
		return intake.ErrPersistence
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return fmt.Errorf("scan active user submissions: %w", err)
	}
	if count >= gate.maximumActivePerUser {
		return intake.ErrAdmissionDenied
	}
	return rows.Err()
}

type IntakeDatabaseClock struct{ client *dbent.EntClient }

func NewIntakeDatabaseClock(client *dbent.EntClient) (*IntakeDatabaseClock, error) {
	if client == nil {
		return nil, intake.ErrPersistence
	}
	return &IntakeDatabaseClock{client: client}, nil
}

func (clock *IntakeDatabaseClock) Now(ctx context.Context) (time.Time, error) {
	if clock == nil || clock.client == nil || ctx == nil {
		return time.Time{}, intake.ErrPersistence
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return time.Time{}, err
	}
	rows, err := clock.client.DB(ctx).Submission.QueryContext(ctx, `SELECT clock_timestamp()`)
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return time.Time{}, err
		}
		return time.Time{}, intake.ErrPersistence
	}
	var now time.Time
	if err := rows.Scan(&now); err != nil {
		return time.Time{}, err
	}
	if now.IsZero() {
		return time.Time{}, intake.ErrPersistence
	}
	return now.UTC(), rows.Err()
}

var _ intake.CommandGate = (*IntakeGate)(nil)
var _ intake.SubmissionAdmission = (*IntakeGate)(nil)
var _ intake.DatabaseClock = (*IntakeDatabaseClock)(nil)
