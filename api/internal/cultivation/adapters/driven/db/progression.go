package db

import (
	"context"
	"crypto/subtle"
	stdsql "database/sql"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	manualProgressionSourceType  = "manual_adjustment"
	maximumProgressionLedgerPage = 100
	maximumManualProvenanceBytes = 8 << 10
)

type ProgressionRepository struct {
	client *internalent.EntClient
}

func NewProgressionRepository(client *internalent.EntClient) (*ProgressionRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, entity.ErrInvalidProgressionCommand
	}
	return &ProgressionRepository{client: client}, nil
}

var _ ports.ProgressionRepository = (*ProgressionRepository)(nil)

func (repository *ProgressionRepository) Snapshot(
	ctx context.Context,
	userID uuid.UUID,
) (entity.ProgressionSnapshot, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil {
		return entity.ProgressionSnapshot{}, entity.ErrInvalidProgressionCommand
	}
	var snapshot entity.ProgressionSnapshot
	snapshot.UserID = userID
	err := querySingleRow(ctx, repository.client.DB(ctx).UserStats, `
		SELECT total_exp, progression_version,
			COALESCE(progression_updated_at, created_at)
		FROM user_stats
		WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(
		&snapshot.Balance, &snapshot.Version, &snapshot.UpdatedAt,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.ProgressionSnapshot{}, entity.ErrProgressionNotFound
	}
	if err != nil {
		return entity.ProgressionSnapshot{}, fmt.Errorf("read progression snapshot: %w", err)
	}
	if snapshot.Balance < 0 || snapshot.Version < 0 || snapshot.UpdatedAt.IsZero() {
		return entity.ProgressionSnapshot{}, ErrSolveRewardInvariant
	}
	return snapshot, nil
}

func (repository *ProgressionRepository) AdjustmentDelta(
	ctx context.Context,
	userID uuid.UUID,
	entryID uuid.UUID,
) (int64, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		userID == uuid.Nil || entryID == uuid.Nil {
		return 0, entity.ErrInvalidProgressionCommand
	}
	var delta int64
	err := querySingleRow(ctx, repository.client.DB(ctx).ProgressionLedgerEntry, `
		SELECT original.total_exp_delta
		FROM progression_ledger_entries AS original
		WHERE original.id = $1 AND original.user_id = $2
		  AND original.kind = 'manual_adjustment'
		  AND NOT EXISTS (
			SELECT 1 FROM progression_ledger_entries AS reversal
			WHERE reversal.reversed_entry_id = original.id
		  )`, entryID, userID).Scan(&delta)
	if errors.Is(err, stdsql.ErrNoRows) {
		var exists bool
		lookupErr := querySingleRow(ctx, repository.client.DB(ctx).ProgressionLedgerEntry, `
			SELECT EXISTS (
				SELECT 1 FROM progression_ledger_entries
				WHERE id = $1 AND user_id = $2 AND kind = 'manual_adjustment'
			)`, entryID, userID).Scan(&exists)
		if lookupErr != nil {
			return 0, fmt.Errorf("classify progression reversal target: %w", lookupErr)
		}
		if exists {
			return 0, entity.ErrProgressionAlreadyReversed
		}
		return 0, entity.ErrProgressionNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("read progression reversal target: %w", err)
	}
	if delta == 0 || delta == -maximumSignedBigint-1 {
		return 0, ErrSolveRewardInvariant
	}
	return -delta, nil
}

func (repository *ProgressionRepository) ApplyAdjustment(
	ctx context.Context,
	command entity.AdjustmentCommand,
) (entity.AdjustmentResult, error) {
	if repository == nil || repository.client == nil || ctx == nil {
		return entity.AdjustmentResult{}, entity.ErrInvalidProgressionCommand
	}
	executor := repository.client.DB(ctx).ProgressionLedgerEntry
	checksum := entity.AdjustmentSourceChecksum(command)
	if checksum == ([32]byte{}) {
		return entity.AdjustmentResult{}, entity.ErrInvalidProgressionCommand
	}
	if replay, found, err := readAdjustmentReplay(ctx, executor, command, checksum); err != nil || found {
		return replay, err
	}

	now, err := progressionDatabaseNow(ctx, executor)
	if err != nil {
		return entity.AdjustmentResult{}, err
	}
	var balance, version int64
	err = querySingleRow(ctx, executor, `
		SELECT total_exp, progression_version
		FROM user_stats
		WHERE user_id = $1 AND deleted_at IS NULL
		FOR UPDATE`, command.UserID).Scan(&balance, &version)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.AdjustmentResult{}, entity.ErrProgressionNotFound
	}
	if err != nil {
		return entity.AdjustmentResult{}, fmt.Errorf("lock progression adjustment target: %w", err)
	}
	if replay, found, err := readAdjustmentReplay(ctx, executor, command, checksum); err != nil || found {
		return replay, err
	}
	if balance != command.ExpectedBalance || version != command.ExpectedVersion {
		return entity.AdjustmentResult{}, entity.ErrProgressionConflict
	}
	preview, err := entity.NewAdjustmentPreview(
		entity.ProgressionSnapshot{UserID: command.UserID, Balance: balance, Version: version, UpdatedAt: now},
		command.Delta,
		command.ReversedEntryID,
		maximumSignedBigint,
	)
	if err != nil || subtle.ConstantTimeCompare(
		preview.Checksum[:], command.PreviewChecksum[:],
	) != 1 {
		return entity.AdjustmentResult{}, entity.ErrProgressionConflict
	}
	if command.ReversedEntryID != nil {
		if err := validateManualReversal(ctx, executor, command); err != nil {
			return entity.AdjustmentResult{}, err
		}
	}

	newVersion := version + 1
	result, err := executor.ExecContext(ctx, `
		UPDATE user_stats
		SET total_exp = $2, progression_version = $3,
			progression_updated_at = $4, updated_at = $4
		WHERE user_id = $1 AND total_exp = $5
		  AND progression_version = $3 - 1 AND deleted_at IS NULL`,
		command.UserID, preview.BalanceAfter, newVersion, now, balance,
	)
	if err != nil {
		return entity.AdjustmentResult{}, fmt.Errorf("apply progression adjustment: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return entity.AdjustmentResult{}, entity.ErrProgressionConflict
	}
	if err := synchronizeProgressionRanking(ctx, executor, command.UserID, now); err != nil {
		return entity.AdjustmentResult{}, err
	}
	provenance, err := manualAdjustmentProvenance(command)
	if err != nil {
		return entity.AdjustmentResult{}, err
	}
	kind := "manual_adjustment"
	if command.ReversedEntryID != nil {
		kind = "manual_reversal"
	}
	ledgerID := uuid.New()
	_, err = executor.ExecContext(ctx, `
		INSERT INTO progression_ledger_entries (
			id, user_id, kind, source_type, source_aggregate_id,
			source_aggregate_version, source_transition_id, source_event_id,
			reversed_entry_id, reward_rule_revision_id, total_exp_delta,
			balance_before, balance_after, progression_version,
			element_deltas, difficulty_deltas, tag_deltas, source_checksum,
			provenance, correlation_id, occurred_at, created_at
		) VALUES ($1,$2,$3,$4,$2,$5,$6,$6,$7,NULL,$8,$9,$10,$5,
			'[]'::jsonb,'[]'::jsonb,'[]'::jsonb,$11,$12,$13,$14,$14)`,
		ledgerID, command.UserID, kind, manualProgressionSourceType,
		newVersion, command.ID, command.ReversedEntryID, command.Delta,
		balance, preview.BalanceAfter, checksum[:], provenance,
		correlation.FromContext(ctx), now,
	)
	if err != nil {
		return entity.AdjustmentResult{}, mapSolveRewardWriteError("append manual progression ledger", err)
	}
	return entity.AdjustmentResult{
		LedgerEntryID: ledgerID, UserID: command.UserID, Delta: command.Delta,
		BalanceBefore: balance, BalanceAfter: preview.BalanceAfter,
		Version: newVersion, CreatedAt: now,
	}, nil
}

func readAdjustmentReplay(
	ctx context.Context,
	executor rankingSQLExecutor,
	command entity.AdjustmentCommand,
	checksum [32]byte,
) (entity.AdjustmentResult, bool, error) {
	var (
		result           entity.AdjustmentResult
		storedChecksum   []byte
		storedReversal   uuid.NullUUID
		kind, sourceType string
	)
	err := querySingleRow(ctx, executor, `
		SELECT id, user_id, kind, source_type, source_event_id,
			reversed_entry_id, total_exp_delta, balance_before,
			balance_after, progression_version, source_checksum, created_at
		FROM progression_ledger_entries
		WHERE source_event_id = $1`, command.ID).Scan(
		&result.LedgerEntryID, &result.UserID, &kind, &sourceType,
		&command.ID, &storedReversal, &result.Delta, &result.BalanceBefore,
		&result.BalanceAfter, &result.Version, &storedChecksum, &result.CreatedAt,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.AdjustmentResult{}, false, nil
	}
	if err != nil {
		return entity.AdjustmentResult{}, false, fmt.Errorf("read progression adjustment replay: %w", err)
	}
	wantKind := "manual_adjustment"
	if command.ReversedEntryID != nil {
		wantKind = "manual_reversal"
	}
	reversalMatches := command.ReversedEntryID == nil && !storedReversal.Valid
	if command.ReversedEntryID != nil {
		reversalMatches = storedReversal.Valid && storedReversal.UUID == *command.ReversedEntryID
	}
	if result.UserID != command.UserID || kind != wantKind ||
		sourceType != manualProgressionSourceType || result.Delta != command.Delta ||
		result.BalanceBefore != command.ExpectedBalance ||
		result.Version != command.ExpectedVersion+1 || !reversalMatches ||
		len(storedChecksum) != len(checksum) ||
		subtle.ConstantTimeCompare(storedChecksum, checksum[:]) != 1 {
		return entity.AdjustmentResult{}, false, entity.ErrProgressionConflict
	}
	result.Replayed = true
	return result, true, nil
}

func validateManualReversal(
	ctx context.Context,
	executor rankingSQLExecutor,
	command entity.AdjustmentCommand,
) error {
	var originalDelta int64
	// progression_ledger_entries is append-only; original can never be updated
	// or deleted, so FOR SHARE OF original provides no protection. Concurrent
	// double-reversal is prevented by the unique constraint on source_event_id
	// and the FOR UPDATE on user_stats acquired earlier in ApplyAdjustment.
	err := querySingleRow(ctx, executor, `
		SELECT original.total_exp_delta
		FROM progression_ledger_entries AS original
		WHERE original.id = $1 AND original.user_id = $2
		  AND original.kind = 'manual_adjustment'
		  AND NOT EXISTS (
			SELECT 1 FROM progression_ledger_entries AS reversal
			WHERE reversal.reversed_entry_id = original.id
		  )`, *command.ReversedEntryID, command.UserID).Scan(&originalDelta)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.ErrProgressionAlreadyReversed
	}
	if err != nil {
		return fmt.Errorf("validate manual progression reversal: %w", err)
	}
	if originalDelta == -maximumSignedBigint-1 || command.Delta != -originalDelta {
		return entity.ErrProgressionConflict
	}
	return nil
}

func manualAdjustmentProvenance(command entity.AdjustmentCommand) ([]byte, error) {
	document := struct {
		Schema          string     `json:"schema"`
		IncidentID      string     `json:"incident_id"`
		Reason          string     `json:"reason"`
		ExpectedBalance int64      `json:"expected_balance"`
		ExpectedVersion int64      `json:"expected_version"`
		ReversedEntryID *uuid.UUID `json:"reversed_entry_id,omitempty"`
	}{
		Schema:     "judgify.cultivation.progression-ledger.manual.v1",
		IncidentID: command.IncidentID, Reason: command.Reason,
		ExpectedBalance: command.ExpectedBalance,
		ExpectedVersion: command.ExpectedVersion,
		ReversedEntryID: command.ReversedEntryID,
	}
	encoded, err := stdjson.Marshal(document)
	if err != nil || len(encoded) == 0 || len(encoded) > maximumManualProvenanceBytes {
		return nil, entity.ErrInvalidProgressionCommand
	}
	return encoded, nil
}

func progressionDatabaseNow(ctx context.Context, executor rankingSQLExecutor) (time.Time, error) {
	var now time.Time
	if err := querySingleRow(ctx, executor, `SELECT CURRENT_TIMESTAMP`).Scan(&now); err != nil {
		return time.Time{}, fmt.Errorf("read progression database time: %w", err)
	}
	return now.UTC(), nil
}

func (repository *ProgressionRepository) ListLedger(
	ctx context.Context,
	userID uuid.UUID,
	cursor *uuid.UUID,
	limit int,
) (ports.ProgressionLedgerPage, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil ||
		limit <= 0 || limit > maximumProgressionLedgerPage ||
		(cursor != nil && *cursor == uuid.Nil) {
		return ports.ProgressionLedgerPage{}, entity.ErrInvalidProgressionCommand
	}
	executor := repository.client.DB(ctx).ProgressionLedgerEntry
	var cursorTime *time.Time
	if cursor != nil {
		var occurredAt time.Time
		err := querySingleRow(ctx, executor, `
			SELECT occurred_at FROM progression_ledger_entries
			WHERE id = $1 AND user_id = $2`, *cursor, userID).Scan(&occurredAt)
		if errors.Is(err, stdsql.ErrNoRows) {
			return ports.ProgressionLedgerPage{}, entity.ErrProgressionNotFound
		}
		if err != nil {
			return ports.ProgressionLedgerPage{}, fmt.Errorf("read progression ledger cursor: %w", err)
		}
		occurredAt = occurredAt.UTC()
		cursorTime = &occurredAt
	}
	rows, err := executor.QueryContext(ctx, `
		SELECT id, user_id, kind, source_type, source_aggregate_id,
			source_aggregate_version, source_event_id, reversed_entry_id,
			reward_rule_revision_id, reward_profile_revision_id, source_checksum,
			total_exp_delta, balance_before, balance_after,
			progression_version, provenance, correlation_id, occurred_at
		FROM progression_ledger_entries
		WHERE user_id = $1
		  AND ($2::timestamptz IS NULL OR occurred_at < $2
			OR (occurred_at = $2 AND id < $3))
		ORDER BY occurred_at DESC, id DESC
		LIMIT $4`, userID, cursorTime, cursor, limit+1)
	if err != nil {
		return ports.ProgressionLedgerPage{}, fmt.Errorf("query progression ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()
	entries := make([]entity.ProgressionLedgerEntry, 0, limit+1)
	for rows.Next() {
		var (
			entry          entity.ProgressionLedgerEntry
			reversal       uuid.NullUUID
			rewardRule     uuid.NullUUID
			rewardProfile  uuid.NullUUID
			sourceChecksum []byte
			provenance     []byte
		)
		if err := rows.Scan(
			&entry.ID, &entry.UserID, &entry.Kind, &entry.SourceType,
			&entry.SourceAggregateID, &entry.SourceVersion, &entry.SourceEventID,
			&reversal, &rewardRule, &rewardProfile, &sourceChecksum,
			&entry.Delta, &entry.BalanceBefore, &entry.BalanceAfter,
			&entry.ProgressionVersion, &provenance, &entry.CorrelationID,
			&entry.OccurredAt,
		); err != nil {
			return ports.ProgressionLedgerPage{}, fmt.Errorf("scan progression ledger: %w", err)
		}
		if reversal.Valid {
			value := reversal.UUID
			entry.ReversedEntryID = &value
		}
		if rewardRule.Valid {
			value := rewardRule.UUID
			entry.RewardRuleRevisionID = &value
		}
		if rewardProfile.Valid {
			value := rewardProfile.UUID
			entry.RewardProfileRevisionID = &value
		}
		if len(sourceChecksum) != 32 {
			return ports.ProgressionLedgerPage{}, ErrSolveRewardInvariant
		}
		copy(entry.SourceChecksum[:], sourceChecksum)
		var manual struct {
			IncidentID string `json:"incident_id"`
			Reason     string `json:"reason"`
		}
		if entry.SourceType == manualProgressionSourceType {
			if err := commonjson.UnmarshalStrictLimit(provenance, &manual, maximumManualProvenanceBytes); err != nil {
				return ports.ProgressionLedgerPage{}, ErrSolveRewardInvariant
			}
			entry.IncidentID, entry.Reason = manual.IncidentID, manual.Reason
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return ports.ProgressionLedgerPage{}, fmt.Errorf("iterate progression ledger: %w", err)
	}
	page := ports.ProgressionLedgerPage{Entries: entries}
	if len(entries) > limit {
		page.Entries = entries[:limit]
		value := page.Entries[len(page.Entries)-1].ID
		page.NextCursor = &value
	}
	return page, nil
}
