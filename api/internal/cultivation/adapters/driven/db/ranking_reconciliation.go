package db

import (
	"context"
	stdsql "database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

var _ ports.RankingReconciliationRepository = (*RankingProjectionRepository)(nil)

type rankingReconciliationCandidate struct {
	UserID      uuid.UUID
	NeedsUpsert bool
}

func (repository *RankingProjectionRepository) Prepare(
	ctx context.Context,
	operationID uuid.UUID,
	expected entity.RankingReconciliationPlan,
) (entity.RankingReconciliationPlan, error) {
	if repository == nil || repository.client == nil || ctx == nil || ctx.Err() != nil {
		return entity.RankingReconciliationPlan{}, entity.ErrInvalidRankingReconciliation
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return entity.RankingReconciliationPlan{}, err
	}
	if operationID == uuid.Nil || !validRankingReconciliationExpectedPlan(expected) {
		return entity.RankingReconciliationPlan{}, entity.ErrInvalidRankingReconciliation
	}
	executor := repository.client.DB(ctx).CultivationRankingProjection
	if _, err := executor.ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"cultivation:ranking:reconciliation",
	); err != nil {
		return entity.RankingReconciliationPlan{}, fmt.Errorf(
			"lock cultivation ranking reconciliation admission: %w",
			err,
		)
	}
	var (
		active        bool
		sameOperation bool
	)
	err := querySingleRow(ctx, executor, `
		SELECT EXISTS (
			SELECT 1 FROM operations
			WHERE kind = $1 AND id <> $2
			  AND status IN ('pending', 'running', 'paused')
		), EXISTS (
			SELECT 1 FROM operations WHERE id = $2
		)`, entity.RankingReconciliationOperationKind, operationID).Scan(
		&active,
		&sameOperation,
	)
	if err != nil {
		return entity.RankingReconciliationPlan{}, fmt.Errorf(
			"inspect active cultivation ranking reconciliation: %w",
			err,
		)
	}
	if active || sameOperation {
		return entity.RankingReconciliationPlan{}, entity.ErrRankingReconciliationActive
	}
	if err := lockCultivationRankingProjection(ctx, executor); err != nil {
		return entity.RankingReconciliationPlan{}, err
	}
	plan, err := repository.rankingReconciliationPlan(ctx, expected.SourceCutoff)
	if err != nil {
		return entity.RankingReconciliationPlan{}, err
	}
	if !plan.SourceCutoff.Equal(expected.SourceCutoff) ||
		plan.Total != expected.Total || plan.RepairCount != expected.RepairCount ||
		plan.ProjectionRevision != expected.ProjectionRevision {
		return entity.RankingReconciliationPlan{}, entity.ErrRankingReconciliationPreviewConflict
	}
	return plan, nil
}

func (repository *RankingProjectionRepository) Preview(
	ctx context.Context,
) (entity.RankingReconciliationPlan, error) {
	if repository == nil || repository.client == nil || ctx == nil || ctx.Err() != nil {
		return entity.RankingReconciliationPlan{}, entity.ErrInvalidRankingReconciliation
	}
	return repository.rankingReconciliationPlan(ctx, time.Time{})
}

func (repository *RankingProjectionRepository) ReconcileBatch(
	ctx context.Context,
	query entity.RankingReconciliationBatchQuery,
) (entity.RankingReconciliationBatch, error) {
	if repository == nil || repository.client == nil || ctx == nil || ctx.Err() != nil ||
		query.SourceCutoff.IsZero() || query.SourceCutoff.Location() != time.UTC ||
		query.Limit < 1 || query.Limit > 1000 ||
		(query.ThroughUserID == uuid.Nil && query.AfterUserID != uuid.Nil) ||
		((query.ExpectedTotal == 0) != (query.ThroughUserID == uuid.Nil)) {
		return entity.RankingReconciliationBatch{}, entity.ErrInvalidRankingReconciliation
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return entity.RankingReconciliationBatch{}, err
	}
	if query.ExpectedTotal < 0 {
		return entity.RankingReconciliationBatch{}, entity.ErrInvalidRankingReconciliation
	}
	if err := lockCultivationRankingProjection(
		ctx,
		repository.client.DB(ctx).CultivationRankingProjection,
	); err != nil {
		return entity.RankingReconciliationBatch{}, err
	}
	candidates, done, err := repository.rankingReconciliationCandidates(ctx, query)
	if err != nil {
		return entity.RankingReconciliationBatch{}, err
	}
	batch := entity.RankingReconciliationBatch{
		Processed:  int64(len(candidates)),
		LastUserID: query.AfterUserID,
		Done:       done,
	}
	for _, candidate := range candidates {
		batch.LastUserID = candidate.UserID
		if candidate.NeedsUpsert {
			batch.Repaired++
		}
	}
	if batch.Repaired > 0 {
		var databaseNow time.Time
		err = querySingleRow(
			ctx, repository.client.DB(ctx).CultivationRankingProjection,
			`SELECT clock_timestamp()`,
		).Scan(&databaseNow)
		if err != nil {
			return entity.RankingReconciliationBatch{}, fmt.Errorf(
				"read ranking reconciliation database time: %w",
				err,
			)
		}
		batch.ProjectionRevision, err = advanceCultivationRankingRevision(
			ctx,
			repository.client.DB(ctx).CultivationRankingProjection,
			databaseNow.UTC(),
		)
		if err != nil {
			return entity.RankingReconciliationBatch{}, err
		}
		for _, candidate := range candidates {
			switch {
			case candidate.NeedsUpsert:
				err = syncCultivationRankingProjection(
					ctx,
					repository.client.DB(ctx).CultivationRankingProjection,
					candidate.UserID,
					uuid.New(),
					batch.ProjectionRevision,
					databaseNow.UTC(),
				)
			default:
				continue
			}
			if err != nil {
				return entity.RankingReconciliationBatch{}, err
			}
		}
	}
	if done {
		if err := repository.verifyRankingReconciliationFence(ctx, query); err != nil {
			return entity.RankingReconciliationBatch{}, err
		}
		batch.ProjectionRevision, err = repository.markRankingReconciled(
			ctx,
			query.SourceCutoff,
		)
		if err != nil {
			return entity.RankingReconciliationBatch{}, err
		}
	} else if batch.ProjectionRevision == 0 {
		batch.ProjectionRevision, err = currentCultivationRankingRevision(
			ctx,
			repository.client.DB(ctx).CultivationRankingProjection,
		)
		if err != nil {
			return entity.RankingReconciliationBatch{}, err
		}
	}
	return batch, nil
}

func (repository *RankingProjectionRepository) State(
	ctx context.Context,
) (entity.RankingReconciliationState, error) {
	if repository == nil || repository.client == nil || ctx == nil || ctx.Err() != nil {
		return entity.RankingReconciliationState{}, entity.ErrInvalidRankingReconciliation
	}
	var (
		state            entity.RankingReconciliationState
		lastReconciledAt stdsql.NullTime
	)
	err := querySingleRow(ctx, repository.client.DB(ctx).CultivationRankingState, `
		SELECT revision, last_reconciled_at
		FROM cultivation_ranking_states
		WHERE key = $1`, cultivationRankingStateKey).Scan(
		&state.ProjectionRevision,
		&lastReconciledAt,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.RankingReconciliationState{}, nil
	}
	if err != nil {
		return entity.RankingReconciliationState{}, fmt.Errorf(
			"read cultivation ranking reconciliation state: %w",
			err,
		)
	}
	if state.ProjectionRevision < 0 {
		return entity.RankingReconciliationState{}, ErrSolveRewardInvariant
	}
	if lastReconciledAt.Valid {
		value := lastReconciledAt.Time.UTC()
		state.LastReconciledAt = &value
	}
	return state, nil
}

func (repository *RankingProjectionRepository) rankingReconciliationCandidates(
	ctx context.Context,
	query entity.RankingReconciliationBatchQuery,
) ([]rankingReconciliationCandidate, bool, error) {
	rows, err := repository.client.DB(ctx).CultivationRankingProjection.QueryContext(ctx, `
		WITH cohort AS (
			SELECT stats.user_id
			FROM user_stats AS stats
			WHERE stats.created_at <= $1
			UNION
			SELECT projection.user_id
			FROM cultivation_ranking_projections AS projection
			WHERE projection.created_at <= $1
		), page AS (
			SELECT cohort.user_id
			FROM cohort
			WHERE ($2::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR cohort.user_id > $2)
			  AND $3::uuid <> '00000000-0000-0000-0000-000000000000'::uuid
			  AND cohort.user_id <= $3
			ORDER BY cohort.user_id
			LIMIT $4
		)
		SELECT page.user_id,
			(
				stats.user_id IS NOT NULL AND stats.deleted_at IS NULL AND
				account.id IS NOT NULL AND account.deleted_at IS NULL AND account.status = 'active' AND
				(
					projection.user_id IS NULL OR
					projection.progression_source_version IS DISTINCT FROM stats.progression_version OR
					projection.rating_source_version IS DISTINCT FROM stats.rating_version OR
					projection.total_exp IS DISTINCT FROM stats.total_exp OR
					projection.rating IS DISTINCT FROM stats.rating OR
					projection.progression_updated_at IS DISTINCT FROM COALESCE(stats.progression_updated_at, stats.created_at) OR
					projection.rating_updated_at IS DISTINCT FROM COALESCE(stats.rating_updated_at, stats.created_at)
				)
			) AS needs_upsert
		FROM page
		LEFT JOIN user_stats AS stats ON stats.user_id = page.user_id
		LEFT JOIN users AS account ON account.id = page.user_id
		LEFT JOIN cultivation_ranking_projections AS projection ON projection.user_id = page.user_id
		ORDER BY page.user_id`,
		query.SourceCutoff,
		query.AfterUserID,
		query.ThroughUserID,
		query.Limit+1,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"query cultivation ranking reconciliation batch: %w",
			err,
		)
	}
	defer func() { _ = rows.Close() }()
	candidates := make([]rankingReconciliationCandidate, 0, query.Limit+1)
	for rows.Next() {
		var candidate rankingReconciliationCandidate
		if err := rows.Scan(
			&candidate.UserID,
			&candidate.NeedsUpsert,
		); err != nil {
			return nil, false, fmt.Errorf(
				"scan cultivation ranking reconciliation batch: %w",
				err,
			)
		}
		if candidate.UserID == uuid.Nil {
			return nil, false, ErrSolveRewardInvariant
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf(
			"iterate cultivation ranking reconciliation batch: %w",
			err,
		)
	}
	done := len(candidates) <= query.Limit
	if !done {
		candidates = candidates[:query.Limit]
	}
	return candidates, done, nil
}

func (repository *RankingProjectionRepository) markRankingReconciled(
	ctx context.Context,
	sourceCutoff time.Time,
) (int64, error) {
	if sourceCutoff.IsZero() || sourceCutoff.Location() != time.UTC {
		return 0, entity.ErrInvalidRankingReconciliation
	}
	if err := lockCultivationRankingProjection(
		ctx,
		repository.client.DB(ctx).CultivationRankingState,
	); err != nil {
		return 0, err
	}
	var revision int64
	err := querySingleRow(ctx, repository.client.DB(ctx).CultivationRankingState, `
		WITH database_clock AS (
			SELECT clock_timestamp() AS now
		)
		INSERT INTO cultivation_ranking_states (
			id, key, revision, last_reconciled_at, created_at, updated_at
		)
		SELECT $1, $2, 0, $3, database_clock.now, database_clock.now
		FROM database_clock
		ON CONFLICT (key) DO UPDATE
		SET last_reconciled_at = GREATEST(
			COALESCE(cultivation_ranking_states.last_reconciled_at, '-infinity'::timestamptz),
			EXCLUDED.last_reconciled_at
		),
		updated_at = EXCLUDED.updated_at
		RETURNING revision`,
		cultivationRankingStateID,
		cultivationRankingStateKey,
		sourceCutoff,
	).Scan(&revision)
	if err != nil {
		return 0, fmt.Errorf("mark cultivation ranking reconciled: %w", err)
	}
	if revision < 0 {
		return 0, ErrSolveRewardInvariant
	}
	return revision, nil
}

func validRankingReconciliationExpectedPlan(plan entity.RankingReconciliationPlan) bool {
	return !plan.SourceCutoff.IsZero() && plan.SourceCutoff.Location() == time.UTC &&
		plan.Total >= 0 && plan.RepairCount >= 0 && plan.RepairCount <= plan.Total &&
		plan.ProjectionRevision >= 0 && plan.ThroughUserID == uuid.Nil
}

func (repository *RankingProjectionRepository) rankingReconciliationPlan(
	ctx context.Context,
	sourceCutoff time.Time,
) (entity.RankingReconciliationPlan, error) {
	if repository == nil || repository.client == nil || ctx == nil || ctx.Err() != nil ||
		(!sourceCutoff.IsZero() && sourceCutoff.Location() != time.UTC) {
		return entity.RankingReconciliationPlan{}, entity.ErrInvalidRankingReconciliation
	}
	cutoffExpression := "clock_timestamp()"
	arguments := []any{}
	if !sourceCutoff.IsZero() {
		cutoffExpression = "$1::timestamptz"
		arguments = append(arguments, sourceCutoff)
	}
	statement := fmt.Sprintf(`
		WITH database_clock AS MATERIALIZED (
			SELECT %s AS source_cutoff
		), cohort AS MATERIALIZED (
			SELECT stats.user_id
			FROM user_stats AS stats CROSS JOIN database_clock
			WHERE stats.created_at <= database_clock.source_cutoff
			UNION
			SELECT projection.user_id
			FROM cultivation_ranking_projections AS projection CROSS JOIN database_clock
			WHERE projection.created_at <= database_clock.source_cutoff
		), inspection AS (
			SELECT COUNT(*) AS source_count,
				(SELECT user_id FROM cohort ORDER BY user_id DESC LIMIT 1) AS through_user_id,
				COUNT(*) FILTER (WHERE
					stats.user_id IS NOT NULL AND stats.deleted_at IS NULL AND
					account.id IS NOT NULL AND account.deleted_at IS NULL AND account.status = 'active' AND
					(
						projection.user_id IS NULL OR
						projection.progression_source_version IS DISTINCT FROM stats.progression_version OR
						projection.rating_source_version IS DISTINCT FROM stats.rating_version OR
						projection.total_exp IS DISTINCT FROM stats.total_exp OR
						projection.rating IS DISTINCT FROM stats.rating OR
						projection.progression_updated_at IS DISTINCT FROM COALESCE(stats.progression_updated_at, stats.created_at) OR
						projection.rating_updated_at IS DISTINCT FROM COALESCE(stats.rating_updated_at, stats.created_at)
					)
				) AS repair_count
			FROM cohort
			LEFT JOIN user_stats AS stats ON stats.user_id = cohort.user_id
			LEFT JOIN users AS account ON account.id = cohort.user_id
			LEFT JOIN cultivation_ranking_projections AS projection ON projection.user_id = cohort.user_id
		), state AS (
			SELECT COALESCE((
				SELECT revision FROM cultivation_ranking_states WHERE key = $%d
			), 0)::bigint AS revision
		)
		SELECT database_clock.source_cutoff, inspection.source_count,
			inspection.repair_count, inspection.through_user_id, state.revision
		FROM database_clock CROSS JOIN inspection CROSS JOIN state`,
		cutoffExpression,
		len(arguments)+1,
	)
	arguments = append(arguments, cultivationRankingStateKey)
	var (
		plan          entity.RankingReconciliationPlan
		throughUserID uuid.NullUUID
	)
	err := querySingleRow(
		ctx, repository.client.DB(ctx).CultivationRankingProjection,
		statement,
		arguments...,
	).Scan(
		&plan.SourceCutoff,
		&plan.Total,
		&plan.RepairCount,
		&throughUserID,
		&plan.ProjectionRevision,
	)
	if err != nil {
		return entity.RankingReconciliationPlan{}, fmt.Errorf(
			"inspect cultivation ranking reconciliation plan: %w",
			err,
		)
	}
	plan.SourceCutoff = plan.SourceCutoff.UTC()
	if throughUserID.Valid {
		plan.ThroughUserID = throughUserID.UUID
	}
	if plan.SourceCutoff.IsZero() || plan.Total < 0 || plan.RepairCount < 0 ||
		plan.RepairCount > plan.Total || plan.ProjectionRevision < 0 ||
		((plan.Total == 0) != (plan.ThroughUserID == uuid.Nil)) {
		return entity.RankingReconciliationPlan{}, ErrSolveRewardInvariant
	}
	return plan, nil
}

func (repository *RankingProjectionRepository) verifyRankingReconciliationFence(
	ctx context.Context,
	query entity.RankingReconciliationBatchQuery,
) error {
	var (
		actualTotal   int64
		actualThrough uuid.NullUUID
	)
	err := querySingleRow(ctx, repository.client.DB(ctx).CultivationRankingProjection, `
		WITH cohort AS (
			SELECT stats.user_id FROM user_stats AS stats WHERE stats.created_at <= $1
			UNION
			SELECT projection.user_id FROM cultivation_ranking_projections AS projection
			WHERE projection.created_at <= $1
		)
		SELECT COUNT(*),
			(SELECT user_id FROM cohort ORDER BY user_id DESC LIMIT 1)
		FROM cohort`, query.SourceCutoff).Scan(
		&actualTotal,
		&actualThrough,
	)
	if err != nil {
		return fmt.Errorf("verify cultivation ranking reconciliation fence: %w", err)
	}
	throughUserID := uuid.Nil
	if actualThrough.Valid {
		throughUserID = actualThrough.UUID
	}
	if actualTotal != query.ExpectedTotal || throughUserID != query.ThroughUserID {
		return entity.ErrRankingReconciliationPreviewConflict
	}
	return nil
}
