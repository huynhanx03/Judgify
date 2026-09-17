package operation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

const operationColumns = `
	id, kind, idempotency_key, request_hash, request_payload,
	actor_user_id, actor_session_id, audit_id, reason, correlation_id,
	status, requested_action, total_count, processed_count, succeeded_count,
	failed_count, next_batch_sequence, batch_size, checkpoint, version,
	lease_owner, lease_token, leased_until, not_before, available_at, attempt_count,
	last_error_code, started_at, finished_at, created_at, updated_at`

type rowScanner interface {
	Scan(...any) error
}

type EntRepository struct {
	client *dbent.EntClient
}

func NewEntRepository(client *dbent.EntClient) (*EntRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, ErrInvalidOperation
	}
	return &EntRepository{client: client}, nil
}

var _ Repository = (*EntRepository)(nil)
var _ OperationBatchReader = (*EntRepository)(nil)

func (repository *EntRepository) LockStart(
	ctx context.Context,
	command CreateCommand,
) (Operation, bool, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		ValidateCreateIntent(command) != nil {
		return Operation{}, false, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return Operation{}, false, err
	}
	lockKey := command.Kind + ":" + command.IdempotencyKey.String()
	if _, err := client.AuditLog.ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		lockKey,
	); err != nil {
		return Operation{}, false, fmt.Errorf("operation: lock start: %w", err)
	}
	value, err := queryOneOperation(
		ctx,
		client,
		`SELECT `+operationColumns+`
		 FROM operations
		 WHERE kind = $1 AND idempotency_key = $2
		 FOR UPDATE`,
		command.Kind,
		command.IdempotencyKey,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, false, nil
	}
	if err != nil {
		return Operation{}, false, fmt.Errorf("operation: read start replay: %w", err)
	}
	return value, true, nil
}

func (repository *EntRepository) Create(
	ctx context.Context,
	command CreateCommand,
) (CreateResult, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		ValidateCreate(command) != nil {
		return CreateResult{}, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return CreateResult{}, err
	}
	value, err := queryOneOperation(
		ctx,
		client,
		`WITH database_clock AS (
			SELECT clock_timestamp() AS now
		)
		INSERT INTO operations (
			id, kind, idempotency_key, request_hash, request_payload,
			actor_user_id, actor_session_id, audit_id, reason, correlation_id,
			status, requested_action, total_count, processed_count, succeeded_count,
			failed_count, next_batch_sequence, batch_size, checkpoint, version,
			not_before, available_at, attempt_count, created_at, updated_at
		)
		SELECT
			$1, $2, $3, $4, $5::jsonb,
			$6, $7, $8, $9, $10,
			'pending', 'none', $11, 0, 0,
			0, 1, $12, $13::jsonb, 1,
			$14::timestamptz,
			COALESCE($14::timestamptz, database_clock.now), 0,
			database_clock.now, database_clock.now
		FROM database_clock
		RETURNING `+operationColumns,
		command.ID,
		command.Kind,
		command.IdempotencyKey,
		command.RequestHash[:],
		[]byte(command.Request),
		command.ActorUserID,
		command.ActorSessionID,
		command.AuditID,
		command.Reason,
		command.CorrelationID,
		command.Total,
		command.BatchSize,
		[]byte(command.Checkpoint),
		command.NotBefore,
	)
	if err != nil {
		if operationSQLState(err) == "23505" {
			return CreateResult{}, errors.Join(ErrConflict, err)
		}
		return CreateResult{}, fmt.Errorf("operation: create: %w", err)
	}
	return CreateResult{Operation: value}, nil
}

func (repository *EntRepository) Get(
	ctx context.Context,
	id uuid.UUID,
) (Operation, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		id == uuid.Nil {
		return Operation{}, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, repository.client.Client())
	if err != nil {
		return Operation{}, err
	}
	value, err := queryOneOperation(
		ctx,
		client,
		`SELECT `+operationColumns+` FROM operations WHERE id = $1`,
		id,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, ErrNotFound
	}
	if err != nil {
		return Operation{}, fmt.Errorf("operation: get: %w", err)
	}
	return value, nil
}

func (repository *EntRepository) List(
	ctx context.Context,
	filter ListFilter,
) (Page, error) {
	if repository == nil || repository.client == nil || ctx == nil {
		return Page{}, ErrInvalidOperation
	}
	if err := ValidateListFilter(filter); err != nil {
		return Page{}, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, repository.client.Client())
	if err != nil {
		return Page{}, err
	}
	var beforeAt any
	var beforeID any
	if !filter.BeforeAt.IsZero() {
		beforeAt = filter.BeforeAt.UTC()
		beforeID = filter.BeforeID
	}
	rows, err := client.AuditLog.QueryContext(
		ctx,
		`SELECT `+operationColumns+`
		 FROM operations
		 WHERE ($1 = '' OR status = $1)
		   AND ($2 = '' OR kind = $2)
		   AND ($3::uuid IS NULL OR actor_user_id = $3)
		   AND ($4::timestamptz IS NULL OR (created_at, id) < ($4, $5))
		 ORDER BY created_at DESC, id DESC
		 LIMIT $6`,
		string(filter.Status),
		filter.Kind,
		nullableUUID(filter.ActorID),
		beforeAt,
		beforeID,
		filter.Limit+1,
	)
	if err != nil {
		return Page{}, fmt.Errorf("operation: list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]Operation, 0, filter.Limit+1)
	for rows.Next() {
		value, scanErr := scanOperation(rows)
		if scanErr != nil {
			return Page{}, fmt.Errorf("operation: list projection: %w", scanErr)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("operation: list rows: %w", err)
	}
	page := Page{Items: items}
	if len(items) > filter.Limit {
		last := items[filter.Limit-1]
		page.Items = items[:filter.Limit]
		page.NextCursor, err = EncodeCursor(last.CreatedAt, last.ID)
		if err != nil {
			return Page{}, err
		}
	}
	return page, nil
}

func (repository *EntRepository) ListBatches(
	ctx context.Context,
	operationID uuid.UUID,
	filter BatchListFilter,
) (BatchProjectionPage, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		ctx.Err() != nil || operationID == uuid.Nil {
		return BatchProjectionPage{}, ErrInvalidOperation
	}
	if err := ValidateBatchListFilter(filter); err != nil {
		return BatchProjectionPage{}, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, repository.client.Client())
	if err != nil {
		return BatchProjectionPage{}, err
	}
	rows, err := client.AuditLog.QueryContext(
		ctx,
		`SELECT operation_id, sequence, processed_count, succeeded_count,
		        failed_count, failure_summary, started_at, completed_at
		 FROM operation_batches
		 WHERE operation_id = $1
		   AND ($2::bigint = 0 OR sequence < $2)
		 ORDER BY sequence DESC
		 LIMIT $3`,
		operationID,
		filter.BeforeSequence,
		filter.Limit+1,
	)
	if err != nil {
		return BatchProjectionPage{}, fmt.Errorf("operation: list batches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]BatchProjection, 0, filter.Limit+1)
	for rows.Next() {
		var (
			item       BatchProjection
			failureRaw []byte
		)
		if err := rows.Scan(
			&item.OperationID,
			&item.Sequence,
			&item.Processed,
			&item.Succeeded,
			&item.Failed,
			&failureRaw,
			&item.StartedAt,
			&item.CompletedAt,
		); err != nil {
			return BatchProjectionPage{}, fmt.Errorf("operation: scan batch: %w", err)
		}
		item.Failures, err = decodeBatchFailures(failureRaw)
		if item.OperationID != operationID || err != nil ||
			validateBatchProjection(item) != nil {
			return BatchProjectionPage{}, ErrInvalidProjection
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return BatchProjectionPage{}, fmt.Errorf("operation: list batch rows: %w", err)
	}
	page := BatchProjectionPage{Items: items}
	if len(items) > filter.Limit {
		last := items[filter.Limit-1]
		page.Items = items[:filter.Limit]
		page.NextCursor = fmt.Sprintf("%d", last.Sequence)
	}
	return page, nil
}

func decodeBatchFailures(raw []byte) ([]ItemFailure, error) {
	if len(raw) == 0 || len(raw) > MaxCheckpointBytes {
		return nil, ErrInvalidProjection
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var failures []ItemFailure
	if err := decoder.Decode(&failures); err != nil || failures == nil {
		return nil, ErrInvalidProjection
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidProjection
	}
	return failures, nil
}

func (repository *EntRepository) Claim(
	ctx context.Context,
	command ClaimCommand,
) (Operation, bool, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		ctx.Err() != nil || !validIdentifier(command.Owner, 128) ||
		command.LeaseDuration < minimumLeaseDuration ||
		command.LeaseDuration > maximumLeaseDuration {
		return Operation{}, false, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, repository.client.Client())
	if err != nil {
		return Operation{}, false, err
	}
	token, err := uuid.NewV7()
	if err != nil {
		return Operation{}, false, ErrInvalidOperation
	}
	leaseMilliseconds := command.LeaseDuration.Milliseconds()
	value, err := queryOneOperation(
		ctx,
		client,
		`WITH settled AS (
			UPDATE operations
			SET
				status = CASE requested_action
					WHEN 'pause' THEN 'paused'
					WHEN 'cancel' THEN 'cancelled'
					ELSE status
				END,
				requested_action = 'none',
				lease_owner = NULL,
				lease_token = NULL,
				leased_until = NULL,
				finished_at = CASE
					WHEN requested_action = 'cancel' THEN clock_timestamp()
					ELSE finished_at
				END,
				version = version + 1,
				updated_at = clock_timestamp()
			WHERE status = 'running'
			  AND requested_action IN ('pause', 'cancel')
			  AND (leased_until IS NULL OR leased_until <= clock_timestamp())
		),
		candidate AS (
			SELECT id
			FROM operations
			WHERE status IN ('pending', 'running')
			  AND requested_action = 'none'
			  AND available_at <= clock_timestamp()
			  AND (lease_owner IS NULL OR leased_until <= clock_timestamp())
			ORDER BY available_at ASC, created_at ASC, id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE operations AS operation
		SET
			status = 'running',
			lease_owner = $1,
			lease_token = $2,
			leased_until = clock_timestamp() + ($3 * interval '1 millisecond'),
			attempt_count = attempt_count + 1,
			started_at = COALESCE(started_at, clock_timestamp()),
			last_error_code = NULL,
			version = version + 1,
			updated_at = clock_timestamp()
		FROM candidate
		WHERE operation.id = candidate.id
		RETURNING `+qualifiedOperationColumns("operation"),
		command.Owner,
		token,
		leaseMilliseconds,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, false, nil
	}
	if err != nil {
		return Operation{}, false, fmt.Errorf("operation: claim: %w", err)
	}
	return value, true, nil
}

func (repository *EntRepository) LockClaim(
	ctx context.Context,
	lease Lease,
) (Operation, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		lease.OperationID == uuid.Nil || lease.Token == uuid.Nil ||
		!validIdentifier(lease.Owner, 128) {
		return Operation{}, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return Operation{}, err
	}
	value, err := queryOneOperation(
		ctx,
		client,
		`SELECT `+operationColumns+`
		 FROM operations
		 WHERE id = $1
		   AND status = 'running'
		   AND lease_owner = $2
		   AND lease_token = $3
		   AND leased_until > clock_timestamp()
		 FOR UPDATE`,
		lease.OperationID,
		lease.Owner,
		lease.Token,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, ErrLeaseLost
	}
	if err != nil {
		return Operation{}, fmt.Errorf("operation: lock claim: %w", err)
	}
	return value, nil
}

// SettleRequestedControl acknowledges a pause/cancel request at the batch
// boundary before domain code runs. This closes the lease/control race without
// making the generic operation package aware of any domain semantics.
func (repository *EntRepository) SettleRequestedControl(
	ctx context.Context,
	lease Lease,
) (Operation, bool, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		lease.OperationID == uuid.Nil || lease.Token == uuid.Nil ||
		!validIdentifier(lease.Owner, 128) {
		return Operation{}, false, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return Operation{}, false, err
	}
	value, err := queryOneOperation(
		ctx,
		client,
		`UPDATE operations
		 SET
			status = CASE requested_action
				WHEN 'pause' THEN 'paused'
				WHEN 'cancel' THEN 'cancelled'
			END,
			requested_action = 'none',
			lease_owner = NULL,
			lease_token = NULL,
			leased_until = NULL,
			finished_at = CASE
				WHEN requested_action = 'cancel' THEN clock_timestamp()
				ELSE NULL
			END,
			version = version + 1,
			updated_at = clock_timestamp()
		 WHERE id = $1
		   AND status = 'running'
		   AND lease_owner = $2
		   AND lease_token = $3
		   AND requested_action IN ('pause', 'cancel')
		 RETURNING `+operationColumns,
		lease.OperationID,
		lease.Owner,
		lease.Token,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, false, nil
	}
	if err != nil {
		return Operation{}, false, fmt.Errorf(
			"operation: settle requested control: %w",
			err,
		)
	}
	return value, true, nil
}

func (repository *EntRepository) CommitBatch(
	ctx context.Context,
	lease Lease,
	result BatchResult,
	startedAt time.Time,
) (Operation, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		startedAt.IsZero() {
		return Operation{}, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return Operation{}, err
	}
	current, err := repository.LockClaim(ctx, lease)
	if err != nil {
		return Operation{}, err
	}
	if err := ValidateBatchResult(current, result); err != nil {
		return Operation{}, err
	}
	failures := result.Failures
	if failures == nil {
		failures = []ItemFailure{}
	}
	failureSummary, err := json.Marshal(failures)
	if err != nil || len(failureSummary) > MaxCheckpointBytes {
		return Operation{}, ErrInvalidBatchResult
	}
	batchID, err := uuid.NewV7()
	if err != nil {
		return Operation{}, ErrInvalidOperation
	}
	if _, err := client.AuditLog.ExecContext(
		ctx,
		`INSERT INTO operation_batches (
			id, operation_id, sequence, lease_token,
			input_checkpoint, output_checkpoint,
			processed_count, succeeded_count, failed_count, failure_summary,
			started_at, completed_at, created_at
		) VALUES (
			$1, $2, $3, $4,
			$5::jsonb, $6::jsonb,
			$7, $8, $9, $10::jsonb,
			LEAST($11, clock_timestamp()), clock_timestamp(), clock_timestamp()
		)`,
		batchID,
		current.ID,
		current.NextBatchSequence,
		lease.Token,
		[]byte(current.Checkpoint),
		[]byte(result.NextCheckpoint),
		result.Processed,
		result.Succeeded,
		result.Failed,
		failureSummary,
		startedAt.UTC(),
	); err != nil {
		if operationSQLState(err) == "23505" {
			return Operation{}, errors.Join(ErrLeaseLost, err)
		}
		return Operation{}, fmt.Errorf("operation: append batch receipt: %w", err)
	}

	nextStatus := StatusRunning
	nextRequestedAction := "none"
	var failureCode any
	var finished bool
	switch current.RequestedAction {
	case "cancel":
		nextStatus = StatusCancelled
		finished = true
	case "pause":
		nextStatus = StatusPaused
	case "none":
		switch {
		case result.FailureCode != "":
			nextStatus = StatusFailed
			failureCode = result.FailureCode
			finished = true
		case result.Done:
			nextStatus = StatusCompleted
			finished = true
		}
	default:
		return Operation{}, ErrInvalidOperation
	}
	value, err := queryOneOperation(
		ctx,
		client,
		`UPDATE operations
		 SET
			status = $4,
			requested_action = $5,
			processed_count = processed_count + $6,
			succeeded_count = succeeded_count + $7,
			failed_count = failed_count + $8,
			next_batch_sequence = next_batch_sequence + 1,
			checkpoint = $9::jsonb,
			version = version + 1,
			lease_owner = NULL,
			lease_token = NULL,
			leased_until = NULL,
			available_at = clock_timestamp(),
			last_error_code = $10,
			finished_at = CASE WHEN $11 THEN clock_timestamp() ELSE NULL END,
			updated_at = clock_timestamp()
		 WHERE id = $1
		   AND status = 'running'
		   AND lease_owner = $2
		   AND lease_token = $3
		 RETURNING `+operationColumns,
		lease.OperationID,
		lease.Owner,
		lease.Token,
		string(nextStatus),
		nextRequestedAction,
		result.Processed,
		result.Succeeded,
		result.Failed,
		[]byte(result.NextCheckpoint),
		failureCode,
		finished,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, ErrLeaseLost
	}
	if err != nil {
		return Operation{}, fmt.Errorf("operation: commit batch: %w", err)
	}
	return value, nil
}

func (repository *EntRepository) ReleaseRetry(
	ctx context.Context,
	lease Lease,
	directive RetryDirective,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		lease.OperationID == uuid.Nil || lease.Token == uuid.Nil ||
		!validIdentifier(lease.Owner, 128) ||
		!validIdentifier(directive.Code, MaxFailureCodeBytes) ||
		directive.AvailableAt.IsZero() {
		return ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, repository.client.Client())
	if err != nil {
		return err
	}
	result, err := client.AuditLog.ExecContext(
		ctx,
		`UPDATE operations
		 SET
			status = CASE requested_action
				WHEN 'pause' THEN 'paused'
				WHEN 'cancel' THEN 'cancelled'
				ELSE 'running'
			END,
			requested_action = 'none',
			lease_owner = NULL,
			lease_token = NULL,
			leased_until = NULL,
			available_at = CASE
				WHEN requested_action = 'none' THEN $4
				ELSE available_at
			END,
			last_error_code = CASE
				WHEN requested_action = 'none' THEN $5
				ELSE last_error_code
			END,
			finished_at = CASE
				WHEN requested_action = 'cancel' THEN clock_timestamp()
				ELSE finished_at
			END,
			version = version + 1,
			updated_at = clock_timestamp()
		 WHERE id = $1
		   AND status = 'running'
		   AND lease_owner = $2
		   AND lease_token = $3`,
		lease.OperationID,
		lease.Owner,
		lease.Token,
		directive.AvailableAt.UTC(),
		directive.Code,
	)
	if err != nil {
		return fmt.Errorf("operation: release retry: %w", err)
	}
	return requireOneOperationRow(result)
}

func (repository *EntRepository) LockControl(
	ctx context.Context,
	command ControlCommand,
) (ControlResult, bool, error) {
	requestHash, err := ControlRequestHash(command)
	if repository == nil || repository.client == nil || ctx == nil || err != nil {
		return ControlResult{}, false, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return ControlResult{}, false, err
	}
	lockKey := command.OperationID.String() + ":" + command.IdempotencyKey.String()
	if _, err := client.AuditLog.ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		lockKey,
	); err != nil {
		return ControlResult{}, false, fmt.Errorf("operation: lock control: %w", err)
	}
	// operation_control_receipts is append-only: once a receipt is written it
	// can never be updated or deleted (trigger + ACL revocation). FOR SHARE would
	// guard against mutations that cannot occur. Serialisation against concurrent
	// callers with the same idempotency key is already provided by the
	// pg_advisory_xact_lock taken above.
	rows, err := client.AuditLog.QueryContext(
		ctx,
		`SELECT request_hash, audit_id, resulting_version, resulting_status,
		        result_projection
		 FROM operation_control_receipts
		 WHERE operation_id = $1 AND idempotency_key = $2`,
		command.OperationID,
		command.IdempotencyKey,
	)
	if err != nil {
		return ControlResult{}, false, fmt.Errorf("operation: read control replay: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return ControlResult{}, false, err
		}
		return ControlResult{}, false, nil
	}
	var persistedHash []byte
	var auditID uuid.UUID
	var resultingVersion int64
	var resultingStatus string
	var resultProjection []byte
	if err := rows.Scan(
		&persistedHash,
		&auditID,
		&resultingVersion,
		&resultingStatus,
		&resultProjection,
	); err != nil {
		return ControlResult{}, false, err
	}
	if rows.Next() || rows.Err() != nil || len(persistedHash) != sha256.Size ||
		!equalDigest(persistedHash, requestHash) || auditID == uuid.Nil {
		return ControlResult{}, false, ErrConflict
	}
	projected, err := decodeProjection(resultProjection)
	if err != nil ||
		validateProjectionMatches(
			projected,
			command.OperationID,
			resultingVersion,
			Status(resultingStatus),
		) != nil {
		return ControlResult{}, false, errors.Join(ErrInvalidProjection, err)
	}
	return ControlResult{
		Operation: projected, AuditID: auditID, Replay: true,
	}, true, nil
}

func (repository *EntRepository) ApplyControl(
	ctx context.Context,
	command ControlCommand,
	auditID uuid.UUID,
) (Operation, error) {
	requestHash, err := ControlRequestHash(command)
	if repository == nil || repository.client == nil || ctx == nil ||
		err != nil || auditID == uuid.Nil {
		return Operation{}, ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return Operation{}, err
	}
	current, err := queryOneOperation(
		ctx,
		client,
		`SELECT `+operationColumns+`
		 FROM operations
		 WHERE id = $1
		 FOR UPDATE`,
		command.OperationID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, ErrNotFound
	}
	if err != nil {
		return Operation{}, fmt.Errorf("operation: lock control target: %w", err)
	}
	if current.Version != command.ExpectedVersion {
		return Operation{}, ErrConflict
	}

	nextStatus := current.Status
	var nextRequestedAction string
	finished := false
	switch command.Action {
	case ControlPause:
		switch current.Status {
		case StatusPending:
			nextStatus = StatusPaused
			nextRequestedAction = "none"
		case StatusRunning:
			if current.RequestedAction != "none" {
				return Operation{}, ErrConflict
			}
			if current.Lease == nil {
				nextStatus = StatusPaused
				nextRequestedAction = "none"
			} else {
				nextRequestedAction = "pause"
			}
		default:
			return Operation{}, ErrConflict
		}
	case ControlResume:
		if current.Status != StatusPaused || current.Lease != nil ||
			current.RequestedAction != "none" {
			return Operation{}, ErrConflict
		}
		nextStatus = StatusRunning
		nextRequestedAction = "none"
	case ControlCancel:
		switch current.Status {
		case StatusPending, StatusPaused:
			nextStatus = StatusCancelled
			nextRequestedAction = "none"
			finished = true
		case StatusRunning:
			if current.Lease == nil {
				nextStatus = StatusCancelled
				nextRequestedAction = "none"
				finished = true
			} else {
				nextRequestedAction = "cancel"
			}
		default:
			return Operation{}, ErrConflict
		}
	default:
		return Operation{}, ErrInvalidOperation
	}

	updated, err := queryOneOperation(
		ctx,
		client,
		`UPDATE operations
		 SET
			status = $3,
			requested_action = $4,
			available_at = CASE
				WHEN $5 = 'resume' THEN GREATEST(
					clock_timestamp(),
					COALESCE(not_before, clock_timestamp())
				)
				ELSE available_at
			END,
			finished_at = CASE WHEN $6 THEN clock_timestamp() ELSE finished_at END,
			version = version + 1,
			updated_at = clock_timestamp()
		 WHERE id = $1 AND version = $2
		 RETURNING `+operationColumns,
		command.OperationID,
		command.ExpectedVersion,
		string(nextStatus),
		nextRequestedAction,
		string(command.Action),
		finished,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, ErrConflict
	}
	if err != nil {
		return Operation{}, fmt.Errorf("operation: apply control: %w", err)
	}
	projected, err := Project(updated)
	if err != nil {
		return Operation{}, err
	}
	resultProjection, err := json.Marshal(projected)
	if err != nil {
		return Operation{}, ErrInvalidOperation
	}
	receiptID, err := uuid.NewV7()
	if err != nil {
		return Operation{}, ErrInvalidOperation
	}
	if _, err := client.AuditLog.ExecContext(
		ctx,
		`INSERT INTO operation_control_receipts (
			id, operation_id, idempotency_key, request_hash, action,
			expected_version, resulting_version, resulting_status,
			result_projection, actor_user_id, actor_session_id, audit_id, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9::jsonb, $10, $11, $12, clock_timestamp()
		)`,
		receiptID,
		command.OperationID,
		command.IdempotencyKey,
		requestHash[:],
		string(command.Action),
		command.ExpectedVersion,
		updated.Version,
		string(updated.Status),
		resultProjection,
		command.ActorUserID,
		command.ActorSessionID,
		auditID,
	); err != nil {
		if operationSQLState(err) == "23505" {
			return Operation{}, errors.Join(ErrConflict, err)
		}
		return Operation{}, fmt.Errorf("operation: append control receipt: %w", err)
	}
	return updated, nil
}

func (repository *EntRepository) MarkFailed(
	ctx context.Context,
	lease Lease,
	code string,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		lease.OperationID == uuid.Nil || lease.Token == uuid.Nil ||
		!validIdentifier(lease.Owner, 128) ||
		!validIdentifier(code, MaxFailureCodeBytes) {
		return ErrInvalidOperation
	}
	client, err := platformtx.Client(ctx, repository.client.Client())
	if err != nil {
		return err
	}
	result, err := client.AuditLog.ExecContext(
		ctx,
		`UPDATE operations
		 SET
			status = CASE requested_action
				WHEN 'pause' THEN 'paused'
				WHEN 'cancel' THEN 'cancelled'
				ELSE 'failed'
			END,
			requested_action = 'none',
			lease_owner = NULL,
			lease_token = NULL,
			leased_until = NULL,
			last_error_code = CASE
				WHEN requested_action = 'none' THEN $4
				ELSE last_error_code
			END,
			finished_at = CASE
				WHEN requested_action = 'pause' THEN NULL
				ELSE clock_timestamp()
			END,
			version = version + 1,
			updated_at = clock_timestamp()
		 WHERE id = $1
		   AND status = 'running'
		   AND lease_owner = $2
		   AND lease_token = $3`,
		lease.OperationID,
		lease.Owner,
		lease.Token,
		code,
	)
	if err != nil {
		return fmt.Errorf("operation: mark failed: %w", err)
	}
	return requireOneOperationRow(result)
}

func queryOneOperation(
	ctx context.Context,
	client *generate.Client,
	query string,
	arguments ...any,
) (Operation, error) {
	if client == nil {
		return Operation{}, ErrInvalidOperation
	}
	rows, err := client.AuditLog.QueryContext(ctx, query, arguments...)
	if err != nil {
		return Operation{}, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return Operation{}, err
		}
		return Operation{}, sql.ErrNoRows
	}
	value, err := scanOperation(rows)
	if err != nil {
		return Operation{}, err
	}
	if rows.Next() {
		return Operation{}, ErrInvalidOperation
	}
	if err := rows.Err(); err != nil {
		return Operation{}, err
	}
	return value, nil
}

func scanOperation(scanner rowScanner) (Operation, error) {
	var (
		value          Operation
		requestHash    []byte
		requestPayload []byte
		checkpoint     []byte
		status         string
		actorSessionID uuid.NullUUID
		leaseOwner     sql.NullString
		leaseToken     uuid.NullUUID
		leasedUntil    sql.NullTime
		notBefore      sql.NullTime
		lastErrorCode  sql.NullString
		startedAt      sql.NullTime
		finishedAt     sql.NullTime
		attemptCount   int64
	)
	if err := scanner.Scan(
		&value.ID,
		&value.Kind,
		&value.IdempotencyKey,
		&requestHash,
		&requestPayload,
		&value.ActorUserID,
		&actorSessionID,
		&value.AuditID,
		&value.Reason,
		&value.CorrelationID,
		&status,
		&value.RequestedAction,
		&value.Total,
		&value.Processed,
		&value.Succeeded,
		&value.Failed,
		&value.NextBatchSequence,
		&value.BatchSize,
		&checkpoint,
		&value.Version,
		&leaseOwner,
		&leaseToken,
		&leasedUntil,
		&notBefore,
		&value.AvailableAt,
		&attemptCount,
		&lastErrorCode,
		&startedAt,
		&finishedAt,
		&value.CreatedAt,
		&value.UpdatedAt,
	); err != nil {
		return Operation{}, err
	}
	if len(requestHash) != sha256.Size || attemptCount < 0 ||
		attemptCount > math.MaxUint32 {
		return Operation{}, ErrInvalidOperation
	}
	copy(value.RequestHash[:], requestHash)
	value.Request = append(json.RawMessage(nil), requestPayload...)
	value.Checkpoint = append(json.RawMessage(nil), checkpoint...)
	value.Status = Status(status)
	value.AttemptCount = uint32(attemptCount)
	if actorSessionID.Valid {
		value.ActorSessionID = cloneUUID(&actorSessionID.UUID)
	}
	if lastErrorCode.Valid {
		value.LastErrorCode = lastErrorCode.String
	}
	if startedAt.Valid {
		started := startedAt.Time.UTC()
		value.StartedAt = &started
	}
	if finishedAt.Valid {
		finished := finishedAt.Time.UTC()
		value.FinishedAt = &finished
	}
	if notBefore.Valid {
		scheduled := notBefore.Time.UTC()
		value.NotBefore = &scheduled
	}
	if leaseOwner.Valid || leaseToken.Valid || leasedUntil.Valid {
		if !leaseOwner.Valid || !leaseToken.Valid || !leasedUntil.Valid {
			return Operation{}, ErrInvalidOperation
		}
		value.Lease = &Lease{
			OperationID: value.ID,
			Token:       leaseToken.UUID,
			Owner:       leaseOwner.String,
			Until:       leasedUntil.Time.UTC(),
		}
	}
	value.AvailableAt = value.AvailableAt.UTC()
	value.CreatedAt = value.CreatedAt.UTC()
	value.UpdatedAt = value.UpdatedAt.UTC()
	if err := Validate(value); err != nil {
		return Operation{}, err
	}
	return value, nil
}

type operationSQLStateError interface {
	error
	SQLState() string
}

func operationSQLState(err error) string {
	var state operationSQLStateError
	if errors.As(err, &state) {
		return state.SQLState()
	}
	return ""
}

func nullableUUID(value uuid.UUID) any {
	if value == uuid.Nil {
		return nil
	}
	return value
}

func qualifiedOperationColumns(alias string) string {
	if !validIdentifier(alias, MaxKindBytes) {
		panic("operation: invalid SQL alias")
	}
	columns := strings.Split(operationColumns, ",")
	for index := range columns {
		columns[index] = alias + "." + strings.TrimSpace(columns[index])
	}
	return strings.Join(columns, ", ")
}

func requireOneOperationRow(result sql.Result) error {
	if result == nil {
		return ErrLeaseLost
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrLeaseLost
	}
	return nil
}

func equalDigest(value []byte, expected [sha256.Size]byte) bool {
	if len(value) != sha256.Size {
		return false
	}
	var actual [sha256.Size]byte
	copy(actual[:], value)
	return actual == expected
}

func decodeProjection(value []byte) (Projection, error) {
	if len(value) < 2 || len(value) > MaxCheckpointBytes ||
		!json.Valid(value) {
		return Projection{}, ErrInvalidProjection
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	var projected Projection
	if err := decoder.Decode(&projected); err != nil {
		return Projection{}, errors.Join(ErrInvalidProjection, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Projection{}, ErrInvalidProjection
	}
	if err := ValidateProjection(projected); err != nil {
		return Projection{}, err
	}
	return projected, nil
}
