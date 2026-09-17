package operations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	internalent "github.com/huynhanx03/judgify/internal/ent"
)

var (
	ErrInvalidQuery      = errors.New("judge operations: invalid query")
	ErrInvalidProjection = errors.New("judge operations: invalid projection")
)

type Query struct {
	client      *internalent.EntClient
	transaction tx.Manager
	freshness   time.Duration
}

func NewQuery(
	client *internalent.EntClient,
	transaction tx.Manager,
	freshness time.Duration,
) (*Query, error) {
	if client == nil || client.Client() == nil || transaction == nil || freshness <= 0 {
		return nil, ErrInvalidQuery
	}
	return &Query{client: client, transaction: transaction, freshness: freshness}, nil
}

func (query *Query) Get(ctx context.Context) (Overview, error) {
	if query == nil || query.client == nil || query.transaction == nil ||
		ctx == nil || ctx.Err() != nil || query.freshness <= 0 {
		return Overview{}, apperr.New(
			apperr.CodeInternalServer,
			"judge operations unavailable",
			ErrInvalidQuery,
		)
	}
	var result Overview
	err := query.transaction.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		result.GeneratedAt, err = query.databaseTime(txCtx)
		if err != nil {
			return err
		}
		result.Queue, err = query.queue(txCtx)
		if err != nil {
			return err
		}
		result.Workers, err = query.workers(txCtx)
		if err != nil {
			return err
		}
		result.Activations, err = query.activations(txCtx)
		return err
	})
	if err != nil {
		return Overview{}, apperr.New(
			apperr.CodeInternalServer,
			"judge operations unavailable",
			err,
		)
	}
	result.WorkerHeartbeatFreshnessMS = query.freshness.Milliseconds()
	if err := validateOverview(result); err != nil {
		return Overview{}, apperr.New(
			apperr.CodeInternalServer,
			"judge operations unavailable",
			err,
		)
	}
	logger.FromContext(ctx).Debug(
		"judge operations snapshot loaded",
		zap.Int64("ready_jobs", result.Queue.Ready),
		zap.Int64("leased_jobs", result.Queue.Leased),
		zap.Int64("retry_wait_jobs", result.Queue.RetryWait),
		zap.Int("worker_count", len(result.Workers)),
		zap.Int("activation_count", len(result.Activations)),
	)
	return result, nil
}

func (query *Query) databaseTime(ctx context.Context) (time.Time, error) {
	rows, err := query.client.DB(ctx).ActivatedJudgeCapability.QueryContext(
		ctx,
		"SELECT clock_timestamp()",
	)
	if err != nil {
		return time.Time{}, fmt.Errorf("read judge operations clock: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return time.Time{}, fmt.Errorf("read judge operations clock: %w", err)
		}
		return time.Time{}, ErrInvalidProjection
	}
	var value time.Time
	if err := rows.Scan(&value); err != nil {
		return time.Time{}, fmt.Errorf("scan judge operations clock: %w", err)
	}
	if rows.Next() || rows.Err() != nil {
		return time.Time{}, ErrInvalidProjection
	}
	return value.UTC(), nil
}

func (query *Query) queue(ctx context.Context) (QueueSnapshot, error) {
	rows, err := query.client.DB(ctx).ActivatedJudgeCapability.QueryContext(ctx, `
SELECT
  COUNT(*) FILTER (WHERE state = 'ready'),
  COUNT(*) FILTER (WHERE state = 'leased'),
  COUNT(*) FILTER (WHERE state = 'retry_wait'),
  COUNT(*) FILTER (WHERE state = 'done'),
  COUNT(*) FILTER (WHERE state = 'cancelled'),
  COUNT(*) FILTER (WHERE state = 'dead'),
  MIN(created_at) FILTER (WHERE state IN ('ready', 'retry_wait'))
FROM judge_jobs`)
	if err != nil {
		return QueueSnapshot{}, fmt.Errorf("read judge queue overview: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return QueueSnapshot{}, fmt.Errorf("read judge queue overview: %w", err)
		}
		return QueueSnapshot{}, ErrInvalidProjection
	}
	var value QueueSnapshot
	if err := rows.Scan(
		&value.Ready,
		&value.Leased,
		&value.RetryWait,
		&value.Done,
		&value.Cancelled,
		&value.Dead,
		&value.OldestReadyAt,
	); err != nil {
		return QueueSnapshot{}, fmt.Errorf("scan judge queue overview: %w", err)
	}
	if rows.Next() || rows.Err() != nil {
		return QueueSnapshot{}, ErrInvalidProjection
	}
	if value.OldestReadyAt != nil {
		normalized := value.OldestReadyAt.UTC()
		value.OldestReadyAt = &normalized
	}
	return value, nil
}

func (query *Query) workers(ctx context.Context) ([]WorkerSnapshot, error) {
	rows, err := query.client.DB(ctx).ActivatedJudgeCapability.QueryContext(ctx, `
SELECT worker.id,
       worker.worker_key,
       worker.boot_id,
       worker.boot_started_at,
       worker.heartbeat_at,
       worker.claims_enabled,
       worker.total_capacity,
       worker.free_capacity,
       COUNT(evidence.id) FILTER (WHERE evidence.passing),
       MAX(evidence.probed_at) FILTER (WHERE evidence.passing)
FROM judge_worker_instances AS worker
LEFT JOIN judge_worker_capabilities AS evidence
  ON evidence.worker_instance_id = worker.id
 AND evidence.boot_id = worker.boot_id
GROUP BY worker.id
ORDER BY worker.heartbeat_at DESC, worker.id ASC
LIMIT $1`, MaximumWorkers)
	if err != nil {
		return nil, fmt.Errorf("read judge worker overview: %w", err)
	}
	defer func() { _ = rows.Close() }()
	values := make([]WorkerSnapshot, 0)
	for rows.Next() {
		var value WorkerSnapshot
		if err := rows.Scan(
			&value.ID,
			&value.WorkerKey,
			&value.BootID,
			&value.BootStartedAt,
			&value.HeartbeatAt,
			&value.ClaimsEnabled,
			&value.TotalCapacity,
			&value.FreeCapacity,
			&value.EvidenceCount,
			&value.LatestProbedAt,
		); err != nil {
			return nil, fmt.Errorf("scan judge worker overview: %w", err)
		}
		value.BootStartedAt = value.BootStartedAt.UTC()
		value.HeartbeatAt = value.HeartbeatAt.UTC()
		if value.LatestProbedAt != nil {
			normalized := value.LatestProbedAt.UTC()
			value.LatestProbedAt = &normalized
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read judge worker overview: %w", err)
	}
	return values, nil
}

func (query *Query) activations(ctx context.Context) ([]ActivationSnapshot, error) {
	rows, err := query.client.DB(ctx).ActivatedJudgeCapability.QueryContext(ctx, `
SELECT id,
       release_id,
       approved_release_manifest_checksum,
       runtime_key,
       runtime_profile_version,
       checker_key,
       checker_version,
       checker_implementation_checksum,
       capability_profile_hash,
       state,
       activated_at,
       retired_at
FROM activated_judge_capabilities
ORDER BY (state = 'active') DESC, activated_at DESC, id ASC
LIMIT $1`, MaximumActivations)
	if err != nil {
		return nil, fmt.Errorf("read judge activation overview: %w", err)
	}
	defer func() { _ = rows.Close() }()
	values := make([]ActivationSnapshot, 0)
	for rows.Next() {
		var value ActivationSnapshot
		if err := rows.Scan(
			&value.ID,
			&value.ReleaseID,
			&value.ApprovedManifestChecksum,
			&value.RuntimeKey,
			&value.RuntimeProfileVersion,
			&value.CheckerKey,
			&value.CheckerVersion,
			&value.CheckerImplementationChecksum,
			&value.CapabilityProfileHash,
			&value.State,
			&value.ActivatedAt,
			&value.RetiredAt,
		); err != nil {
			return nil, fmt.Errorf("scan judge activation overview: %w", err)
		}
		value.ActivatedAt = value.ActivatedAt.UTC()
		if value.RetiredAt != nil {
			normalized := value.RetiredAt.UTC()
			value.RetiredAt = &normalized
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read judge activation overview: %w", err)
	}
	return values, nil
}

func validateOverview(value Overview) error {
	if value.GeneratedAt.IsZero() || value.WorkerHeartbeatFreshnessMS <= 0 ||
		len(value.Workers) > MaximumWorkers ||
		len(value.Activations) > MaximumActivations ||
		value.Queue.Ready < 0 || value.Queue.Leased < 0 ||
		value.Queue.RetryWait < 0 || value.Queue.Done < 0 ||
		value.Queue.Cancelled < 0 || value.Queue.Dead < 0 {
		return ErrInvalidProjection
	}
	if value.Queue.OldestReadyAt != nil &&
		value.Queue.OldestReadyAt.After(value.GeneratedAt) {
		return ErrInvalidProjection
	}
	for _, worker := range value.Workers {
		if worker.ID == uuid.Nil || worker.BootID == uuid.Nil ||
			strings.TrimSpace(worker.WorkerKey) == "" ||
			worker.BootStartedAt.IsZero() || worker.HeartbeatAt.IsZero() ||
			worker.HeartbeatAt.Before(worker.BootStartedAt) ||
			worker.HeartbeatAt.After(value.GeneratedAt) ||
			worker.TotalCapacity < 1 || worker.FreeCapacity < 0 ||
			worker.FreeCapacity > worker.TotalCapacity || worker.EvidenceCount < 0 ||
			(worker.LatestProbedAt != nil &&
				(worker.LatestProbedAt.Before(worker.BootStartedAt) ||
					worker.LatestProbedAt.After(value.GeneratedAt))) {
			return ErrInvalidProjection
		}
	}
	for _, activation := range value.Activations {
		if activation.ID == uuid.Nil ||
			strings.TrimSpace(activation.ReleaseID) == "" ||
			strings.TrimSpace(activation.ApprovedManifestChecksum) == "" ||
			strings.TrimSpace(activation.RuntimeKey) == "" ||
			strings.TrimSpace(activation.RuntimeProfileVersion) == "" ||
			strings.TrimSpace(activation.CheckerKey) == "" ||
			strings.TrimSpace(activation.CheckerVersion) == "" ||
			strings.TrimSpace(activation.CheckerImplementationChecksum) == "" ||
			strings.TrimSpace(activation.CapabilityProfileHash) == "" ||
			activation.ActivatedAt.IsZero() ||
			activation.ActivatedAt.After(value.GeneratedAt) {
			return ErrInvalidProjection
		}
		switch activation.State {
		case "active":
			if activation.RetiredAt != nil {
				return ErrInvalidProjection
			}
		case "retired":
			if activation.RetiredAt == nil ||
				activation.RetiredAt.Before(activation.ActivatedAt) ||
				activation.RetiredAt.After(value.GeneratedAt) {
				return ErrInvalidProjection
			}
		default:
			return ErrInvalidProjection
		}
	}
	return nil
}
