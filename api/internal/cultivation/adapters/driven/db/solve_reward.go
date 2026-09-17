package db

import (
	"context"
	"crypto/subtle"
	stdsql "database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/huynhanx03/go-common/pkg/correlation"
	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"

	cultivationcontract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ports"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

const (
	maximumSolveRewardOutboxPayloadBytes = 256 << 10
	maximumSolveRewardReconcileBatch     = 512
)

var (
	ErrInvalidSolveRewardProjector = errors.New("cultivation solve reward projector: invalid operation")
	ErrSolveRewardConflict         = errors.New("cultivation solve reward projector: conflicting transition evidence")
	ErrSolveRewardInvariant        = errors.New("cultivation solve reward projector: projection invariant violated")
)

type solveRewardSQLDatabase interface {
	BeginTx(context.Context, *stdsql.TxOptions) (*stdsql.Tx, error)
	QueryContext(context.Context, string, ...any) (*stdsql.Rows, error)
}

type solveRewardCursor struct {
	ID                  uuid.UUID
	SolveID             uuid.UUID
	UserID              uuid.UUID
	ProblemID           uuid.UUID
	LastAppliedVersion  int64
	CurrentStatus       string
	ActiveLedgerEntryID *uuid.UUID
	LastTransitionID    *uuid.UUID
	LastEventID         *uuid.UUID
	LastTransitionHash  []byte
	ReconciliationState string
	MissingFromVersion  *int64
	MissingToVersion    *int64
}

type solveRewardApplication struct {
	Outcome              ports.SolveRewardOutcome
	LedgerEntryID        *uuid.UUID
	TotalExperienceDelta int64
	ElementDeltaCount    int
}

// SolveRewardProjector serializes every solve aggregate through a cursor and
// writes accounting, projections, and immutable processed evidence in one
// PostgreSQL transaction. Forge is only a low-latency delivery path.
type SolveRewardProjector struct {
	db    solveRewardSQLDatabase
	newID func() uuid.UUID
}

func NewSolveRewardProjector(client *dbent.EntClient) (*SolveRewardProjector, error) {
	if client == nil {
		return nil, ErrInvalidSolveRewardProjector
	}
	driver, ok := client.DialectDriver()
	if !ok {
		return nil, ErrInvalidSolveRewardProjector
	}
	sqlDriver, ok := driver.(*entsql.Driver)
	if !ok || sqlDriver.DB() == nil {
		return nil, ErrInvalidSolveRewardProjector
	}
	return &SolveRewardProjector{db: sqlDriver.DB(), newID: uuid.New}, nil
}

var _ ports.SolveRewardProjector = (*SolveRewardProjector)(nil)

func (projector *SolveRewardProjector) ApplySolveTransition(
	ctx context.Context,
	event submissioncontract.ProblemSolveStatusChangedV1,
) (ports.SolveRewardResult, error) {
	if projector == nil || projector.db == nil || projector.newID == nil || ctx == nil {
		return ports.SolveRewardResult{}, ErrInvalidSolveRewardProjector
	}
	if err := ctx.Err(); err != nil {
		return ports.SolveRewardResult{}, err
	}
	incoming, err := cultivationcontract.DecodeSolveRewardTransition(event)
	if err != nil {
		return ports.SolveRewardResult{}, ports.PermanentSolveReward(err)
	}

	tx, err := projector.db.BeginTx(ctx, &stdsql.TxOptions{Isolation: stdsql.LevelReadCommitted})
	if err != nil {
		return ports.SolveRewardResult{}, fmt.Errorf("begin solve reward projection: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := solveRewardDatabaseNow(ctx, tx)
	if err != nil {
		return ports.SolveRewardResult{}, err
	}
	canonical, err := readCanonicalSolveRewardTransition(ctx, tx, incoming.TransitionID)
	if errors.Is(err, stdsql.ErrNoRows) {
		return ports.SolveRewardResult{}, permanentSolveReward(ErrSolveRewardConflict)
	}
	if err != nil {
		return ports.SolveRewardResult{}, err
	}
	if !sameSolveRewardTransition(incoming, canonical) {
		return ports.SolveRewardResult{}, permanentSolveReward(ErrSolveRewardConflict)
	}
	if err := verifySolveRewardOutboxEvidence(ctx, tx, canonical); err != nil {
		return ports.SolveRewardResult{}, err
	}

	cursor, err := projector.lockOrCreateSolveRewardCursor(ctx, tx, canonical, now)
	if err != nil {
		return ports.SolveRewardResult{}, err
	}
	if canonical.To.Version <= cursor.LastAppliedVersion {
		result, err := readDuplicateSolveRewardResult(ctx, tx, cursor, canonical)
		if err != nil {
			return ports.SolveRewardResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return ports.SolveRewardResult{}, fmt.Errorf("commit duplicate solve reward delivery: %w", err)
		}
		return result, nil
	}
	if canonical.To.Version > cursor.LastAppliedVersion+1 {
		if err := markSolveRewardGap(ctx, tx, cursor, canonical.From.Version, now); err != nil {
			return ports.SolveRewardResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return ports.SolveRewardResult{}, fmt.Errorf("commit solve reward gap: %w", err)
		}
		return ports.SolveRewardResult{Outcome: ports.SolveRewardGapDeferred}, nil
	}
	if canonical.From.Version != cursor.LastAppliedVersion ||
		canonical.From.Status != cursor.CurrentStatus {
		return ports.SolveRewardResult{}, permanentSolveReward(ErrSolveRewardConflict)
	}

	application, err := projector.applyContiguousSolveReward(ctx, tx, cursor, canonical, now)
	if err != nil {
		return ports.SolveRewardResult{}, err
	}
	if err := projector.insertProcessedSolveRewardTransition(ctx, tx, canonical, application, now); err != nil {
		return ports.SolveRewardResult{}, err
	}
	if err := advanceSolveRewardCursor(ctx, tx, cursor, canonical, application, now); err != nil {
		return ports.SolveRewardResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.SolveRewardResult{}, fmt.Errorf("commit solve reward projection: %w", err)
	}
	return ports.SolveRewardResult{
		Outcome: application.Outcome, TransitionApplied: true,
		LedgerEntryID:        application.LedgerEntryID,
		TotalExperienceDelta: application.TotalExperienceDelta,
		ElementDeltaCount:    application.ElementDeltaCount,
	}, nil
}

// ReconcileSolveRewards replays immutable Submission transition history. It
// does not depend on Forge or retained outbox rows, so local queue loss and
// outbox retention cannot lose a progression transition.
func (projector *SolveRewardProjector) ReconcileSolveRewards(
	ctx context.Context,
	limit int,
) (int, error) {
	if projector == nil || projector.db == nil || ctx == nil ||
		limit <= 0 || limit > maximumSolveRewardReconcileBatch {
		return 0, ErrInvalidSolveRewardProjector
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	rows, err := projector.db.QueryContext(
		ctx,
		canonicalSolveRewardTransitionSelect+`
		LEFT JOIN solve_reward_cursors AS cursor
		  ON cursor.solve_id = transition.solve_id
		WHERE NOT EXISTS (
			SELECT 1
			FROM solve_reward_processed_transitions AS processed
			WHERE processed.solve_id = transition.solve_id
			  AND processed.aggregate_version = transition.to_version
		)
		  AND (
			(cursor.id IS NOT NULL AND (
				transition.to_version <= cursor.last_applied_version
				OR transition.to_version = cursor.last_applied_version + 1
			))
			OR (cursor.id IS NULL AND transition.to_version = (
				SELECT MIN(candidate.to_version)
				FROM user_problem_solve_transitions AS candidate
				WHERE candidate.solve_id = transition.solve_id
			))
		  )
		ORDER BY transition.solve_id, transition.to_version
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return 0, fmt.Errorf("query unreconciled solve reward transitions: %w", err)
	}
	events := make([]submissioncontract.ProblemSolveStatusChangedV1, 0, limit)
	for rows.Next() {
		event, scanErr := scanCanonicalSolveRewardEvent(rows)
		if scanErr != nil {
			_ = rows.Close()
			return 0, scanErr
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("iterate unreconciled solve reward transitions: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close solve reward reconciliation rows: %w", err)
	}

	processed := 0
	for _, event := range events {
		eventCtx := correlation.WithContext(ctx, event.CorrelationID)
		_, applyErr := projector.ApplySolveTransition(eventCtx, event)
		if applyErr != nil {
			return processed, fmt.Errorf("apply reconciled solve reward transition: %w", applyErr)
		}
		processed++
	}
	return processed, nil
}

func solveRewardDatabaseNow(ctx context.Context, tx *stdsql.Tx) (time.Time, error) {
	var now time.Time
	if err := tx.QueryRowContext(ctx, "SELECT CURRENT_TIMESTAMP").Scan(&now); err != nil {
		return time.Time{}, fmt.Errorf("read solve reward database time: %w", err)
	}
	if now.IsZero() {
		return time.Time{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	return now.UTC(), nil
}

func (projector *SolveRewardProjector) lockOrCreateSolveRewardCursor(
	ctx context.Context,
	tx *stdsql.Tx,
	transition cultivationcontract.SolveRewardTransition,
	now time.Time,
) (solveRewardCursor, error) {
	id := projector.newID()
	if id == uuid.Nil {
		return solveRewardCursor{}, ErrInvalidSolveRewardProjector
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO solve_reward_cursors (
			id, solve_id, user_id, problem_id, last_applied_version,
			current_status, reconciliation_state, created_at, updated_at
		) VALUES ($1,$2,$3,$4,0,'unsolved','ready',$5,$5)
		ON CONFLICT (solve_id) DO NOTHING`,
		id, transition.SolveID, transition.UserID, transition.ProblemID, now,
	); err != nil {
		return solveRewardCursor{}, mapSolveRewardWriteError("create cursor", err)
	}

	var (
		cursor                                  solveRewardCursor
		activeLedger, lastTransition, lastEvent uuid.NullUUID
		missingFrom, missingTo                  stdsql.NullInt64
	)
	err := tx.QueryRowContext(ctx, `
		SELECT id, solve_id, user_id, problem_id, last_applied_version,
			current_status, active_ledger_entry_id, last_transition_id,
			last_event_id, last_transition_hash, reconciliation_state,
			missing_from_version, missing_to_version
		FROM solve_reward_cursors
		WHERE solve_id = $1
		FOR UPDATE`, transition.SolveID).Scan(
		&cursor.ID, &cursor.SolveID, &cursor.UserID, &cursor.ProblemID,
		&cursor.LastAppliedVersion, &cursor.CurrentStatus, &activeLedger,
		&lastTransition, &lastEvent, &cursor.LastTransitionHash,
		&cursor.ReconciliationState, &missingFrom, &missingTo,
	)
	if err != nil {
		return solveRewardCursor{}, fmt.Errorf("lock solve reward cursor: %w", err)
	}
	cursor.ActiveLedgerEntryID = nullUUIDPointer(activeLedger)
	cursor.LastTransitionID = nullUUIDPointer(lastTransition)
	cursor.LastEventID = nullUUIDPointer(lastEvent)
	cursor.MissingFromVersion = nullInt64Pointer(missingFrom)
	cursor.MissingToVersion = nullInt64Pointer(missingTo)
	if !validSolveRewardCursor(cursor, transition) {
		return solveRewardCursor{}, permanentSolveReward(ErrSolveRewardConflict)
	}
	return cursor, nil
}

func validSolveRewardCursor(
	cursor solveRewardCursor,
	transition cultivationcontract.SolveRewardTransition,
) bool {
	if cursor.ID == uuid.Nil || cursor.SolveID != transition.SolveID ||
		cursor.UserID != transition.UserID || cursor.ProblemID != transition.ProblemID ||
		cursor.LastAppliedVersion < 0 ||
		(cursor.CurrentStatus != "solved" && cursor.CurrentStatus != "unsolved") {
		return false
	}
	if (cursor.CurrentStatus == "solved") != (cursor.ActiveLedgerEntryID != nil) {
		return false
	}
	if cursor.LastAppliedVersion == 0 {
		if cursor.LastTransitionID != nil || cursor.LastEventID != nil || len(cursor.LastTransitionHash) != 0 {
			return false
		}
	} else if cursor.LastTransitionID == nil || cursor.LastEventID == nil ||
		len(cursor.LastTransitionHash) != sha256Size {
		return false
	}
	if cursor.ReconciliationState == "ready" {
		return cursor.MissingFromVersion == nil && cursor.MissingToVersion == nil
	}
	return cursor.ReconciliationState == "blocked_gap" &&
		cursor.MissingFromVersion != nil && cursor.MissingToVersion != nil &&
		*cursor.MissingFromVersion == cursor.LastAppliedVersion+1 &&
		*cursor.MissingToVersion >= *cursor.MissingFromVersion
}

func markSolveRewardGap(
	ctx context.Context,
	tx *stdsql.Tx,
	cursor solveRewardCursor,
	missingTo int64,
	now time.Time,
) error {
	if missingTo < cursor.LastAppliedVersion+1 {
		return permanentSolveReward(ErrSolveRewardConflict)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE solve_reward_cursors
		SET reconciliation_state = 'blocked_gap',
			missing_from_version = last_applied_version + 1,
			missing_to_version = GREATEST(COALESCE(missing_to_version, $3), $3),
			updated_at = $4
		WHERE id = $1 AND last_applied_version = $2`,
		cursor.ID, cursor.LastAppliedVersion, missingTo, now,
	)
	if err != nil {
		return mapSolveRewardWriteError("mark cursor gap", err)
	}
	return requireOneSolveRewardRow(result)
}

func readDuplicateSolveRewardResult(
	ctx context.Context,
	tx *stdsql.Tx,
	cursor solveRewardCursor,
	transition cultivationcontract.SolveRewardTransition,
) (ports.SolveRewardResult, error) {
	var (
		transitionID, eventID uuid.UUID
		hash                  []byte
		outcome               string
		ledgerID              uuid.NullUUID
	)
	err := tx.QueryRowContext(ctx, `
		SELECT transition_id, event_id, transition_hash, outcome, ledger_entry_id
		FROM solve_reward_processed_transitions
		WHERE solve_id = $1 AND aggregate_version = $2`,
		transition.SolveID, transition.To.Version,
	).Scan(&transitionID, &eventID, &hash, &outcome, &ledgerID)
	if errors.Is(err, stdsql.ErrNoRows) {
		return ports.SolveRewardResult{}, permanentSolveReward(ErrSolveRewardConflict)
	}
	if err != nil {
		return ports.SolveRewardResult{}, fmt.Errorf("read processed solve reward transition: %w", err)
	}
	if transitionID != transition.TransitionID || eventID != transition.EventID ||
		outcome != string(transition.Kind) || !equalDigest(hash, transition.Digest[:]) ||
		transition.To.Version > cursor.LastAppliedVersion ||
		(ledgerID.Valid != (transition.Kind != cultivationcontract.SolveRewardPointerMove)) {
		return ports.SolveRewardResult{}, permanentSolveReward(ErrSolveRewardConflict)
	}
	return ports.SolveRewardResult{
		Outcome:       ports.SolveRewardDuplicate,
		LedgerEntryID: nullUUIDPointer(ledgerID),
	}, nil
}

func (projector *SolveRewardProjector) insertProcessedSolveRewardTransition(
	ctx context.Context,
	tx *stdsql.Tx,
	transition cultivationcontract.SolveRewardTransition,
	application solveRewardApplication,
	now time.Time,
) error {
	id := projector.newID()
	if id == uuid.Nil {
		return ErrInvalidSolveRewardProjector
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO solve_reward_processed_transitions (
			id, solve_id, user_id, problem_id, aggregate_version,
			transition_id, event_id, transition_hash, outcome,
			ledger_entry_id, correlation_id, occurred_at, processed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		id, transition.SolveID, transition.UserID, transition.ProblemID,
		transition.To.Version, transition.TransitionID, transition.EventID,
		transition.Digest[:], string(transition.Kind), application.LedgerEntryID,
		transition.CorrelationID, transition.OccurredAt, now,
	)
	if err != nil {
		return mapSolveRewardWriteError("insert processed transition", err)
	}
	return nil
}

func advanceSolveRewardCursor(
	ctx context.Context,
	tx *stdsql.Tx,
	cursor solveRewardCursor,
	transition cultivationcontract.SolveRewardTransition,
	application solveRewardApplication,
	now time.Time,
) error {
	activeLedger := cursor.ActiveLedgerEntryID
	switch transition.Kind {
	case cultivationcontract.SolveRewardGrant:
		activeLedger = application.LedgerEntryID
	case cultivationcontract.SolveRewardReversal:
		activeLedger = nil
	case cultivationcontract.SolveRewardPointerMove:
	default:
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	state := "ready"
	var missingFrom, missingTo *int64
	if cursor.ReconciliationState == "blocked_gap" && cursor.MissingToVersion != nil &&
		transition.To.Version < *cursor.MissingToVersion {
		state = "blocked_gap"
		from := transition.To.Version + 1
		to := *cursor.MissingToVersion
		missingFrom, missingTo = &from, &to
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE solve_reward_cursors
		SET last_applied_version = $3, current_status = $4,
			active_ledger_entry_id = $5, last_transition_id = $6,
			last_event_id = $7, last_transition_hash = $8,
			reconciliation_state = $9, missing_from_version = $10,
			missing_to_version = $11, updated_at = $12
		WHERE id = $1 AND last_applied_version = $2`,
		cursor.ID, cursor.LastAppliedVersion, transition.To.Version,
		transition.To.Status, activeLedger, transition.TransitionID,
		transition.EventID, transition.Digest[:], state, missingFrom, missingTo, now,
	)
	if err != nil {
		return mapSolveRewardWriteError("advance cursor", err)
	}
	return requireOneSolveRewardRow(result)
}

func verifySolveRewardOutboxEvidence(
	ctx context.Context,
	tx *stdsql.Tx,
	transition cultivationcontract.SolveRewardTransition,
) error {
	var (
		eventType, aggregateType string
		aggregateID              uuid.UUID
		aggregateVersion         int64
		payload                  []byte
	)
	err := tx.QueryRowContext(ctx, `
		SELECT event_type, aggregate_type, aggregate_id, aggregate_version, payload
		FROM outbox_events
		WHERE id = $1`, transition.EventID).Scan(
		&eventType, &aggregateType, &aggregateID, &aggregateVersion, &payload,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		// Retained transition history is the long-term replay authority. A safely
		// pruned outbox row is therefore not required once its identity is bound to
		// the immutable transition.
		return nil
	}
	if err != nil {
		return fmt.Errorf("read solve reward outbox evidence: %w", err)
	}
	if eventType != submissioncontract.EventProblemSolveStatusChangedV1 ||
		aggregateType != submissioncontract.AggregateUserProblemSolve ||
		aggregateID != transition.SolveID || aggregateVersion != transition.To.Version {
		return permanentSolveReward(ErrSolveRewardConflict)
	}
	var event submissioncontract.ProblemSolveStatusChangedV1
	if err := commonjson.UnmarshalStrictLimit(
		payload,
		&event,
		maximumSolveRewardOutboxPayloadBytes,
	); err != nil {
		return permanentSolveReward(fmt.Errorf("%w: decode retained outbox payload", ErrSolveRewardConflict))
	}
	retained, err := cultivationcontract.DecodeSolveRewardTransition(event)
	if err != nil || !sameSolveRewardTransition(retained, transition) {
		return permanentSolveReward(ErrSolveRewardConflict)
	}
	return nil
}

func sameSolveRewardTransition(
	left cultivationcontract.SolveRewardTransition,
	right cultivationcontract.SolveRewardTransition,
) bool {
	return left.EventID == right.EventID && left.TransitionID == right.TransitionID &&
		left.SolveID == right.SolveID && left.UserID == right.UserID &&
		left.ProblemID == right.ProblemID && left.To.Version == right.To.Version &&
		equalDigest(left.Digest[:], right.Digest[:])
}

func equalDigest(left, right []byte) bool {
	return len(left) == sha256Size && len(right) == sha256Size &&
		subtle.ConstantTimeCompare(left, right) == 1
}

func permanentSolveReward(cause error) error { return ports.PermanentSolveReward(cause) }

type solveRewardSQLStateError interface {
	error
	SQLState() string
}

func mapSolveRewardWriteError(operation string, err error) error {
	if err == nil {
		return nil
	}
	wrapped := fmt.Errorf("solve reward %s: %w", operation, err)
	var state solveRewardSQLStateError
	if errors.As(err, &state) {
		code := state.SQLState()
		if len(code) >= 2 && code[:2] == "23" {
			return permanentSolveReward(errors.Join(ErrSolveRewardInvariant, wrapped))
		}
	}
	return wrapped
}

func requireOneSolveRewardRow(result stdsql.Result) error {
	if result == nil {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read solve reward affected rows: %w", err)
	}
	if affected != 1 {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	return nil
}

func nullUUIDPointer(value uuid.NullUUID) *uuid.UUID {
	if !value.Valid || value.UUID == uuid.Nil {
		return nil
	}
	result := value.UUID
	return &result
}

func nullInt64Pointer(value stdsql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}
