package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/realtime"
)

type replaySQLDatabase interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

type ReplayProjection struct {
	database replaySQLDatabase
	config   realtime.ReplayConfig
}

const realtimeCursorStateID = "global"

func NewReplayProjection(
	client *internalent.EntClient,
	config realtime.ReplayConfig,
) (*ReplayProjection, error) {
	if client == nil || config.Validate() != nil {
		return nil, realtime.ErrInvalidConfig
	}
	driver, ok := client.DialectDriver()
	if !ok {
		return nil, realtime.ErrInvalidConfig
	}
	sqlDriver, ok := driver.(*entsql.Driver)
	if !ok || sqlDriver.DB() == nil {
		return nil, realtime.ErrInvalidConfig
	}
	return newReplayProjection(sqlDriver.DB(), config)
}

func newReplayProjection(
	database replaySQLDatabase,
	config realtime.ReplayConfig,
) (*ReplayProjection, error) {
	if database == nil || config.Validate() != nil {
		return nil, realtime.ErrInvalidConfig
	}
	return &ReplayProjection{database: database, config: config}, nil
}

func (projection *ReplayProjection) Project(
	ctx context.Context,
	routed realtime.RoutedMessage,
) (realtime.ProjectedMessage, error) {
	projected, err := projection.ProjectBatch(ctx, []realtime.RoutedMessage{routed})
	if err != nil {
		return realtime.ProjectedMessage{}, err
	}
	if len(projected) != 1 {
		return realtime.ProjectedMessage{}, realtime.ErrReplayConflict
	}
	return projected[0], nil
}

type preparedReplayEvent struct {
	routed        realtime.RoutedMessage
	evidenceHash  [sha256.Size]byte
	deliveryClass string
}

// ProjectBatch assigns one global committed order to the complete routed
// source-event batch. A crash can expose either every topic projection or none
// of them; the in-memory Hub is only contacted after this transaction commits.
func (projection *ReplayProjection) ProjectBatch(
	ctx context.Context,
	routed []realtime.RoutedMessage,
) ([]realtime.ProjectedMessage, error) {
	if projection == nil || projection.database == nil || ctx == nil ||
		ctx.Err() != nil || len(routed) == 0 ||
		len(routed) > projection.config.MaximumBatchSize {
		return nil, realtime.ErrInvalidChange
	}
	prepared := make([]preparedReplayEvent, len(routed))
	identities := make(map[string]struct{}, len(routed))
	for index, candidate := range routed {
		if candidate.Topic == "" ||
			candidate.Message.Envelope.Operation != commonwebsocket.OperationEvent ||
			candidate.Message.Envelope.Topic != candidate.Topic {
			return nil, realtime.ErrInvalidChange
		}
		if _, err := realtime.ParseTopic(candidate.Topic); err != nil {
			return nil, realtime.ErrInvalidChange
		}
		if err := candidate.Message.Validate(); err != nil {
			return nil, errors.Join(realtime.ErrInvalidChange, err)
		}
		identity := candidate.Topic + "\x00" + candidate.Message.Envelope.ID
		if _, duplicate := identities[identity]; duplicate {
			return nil, realtime.ErrReplayConflict
		}
		identities[identity] = struct{}{}
		evidenceHash, err := replayEvidenceHash(candidate)
		if err != nil {
			return nil, err
		}
		deliveryClass, err := encodeDeliveryClass(candidate.Message.Class)
		if err != nil {
			return nil, err
		}
		prepared[index] = preparedReplayEvent{
			routed:        candidate,
			evidenceHash:  evidenceHash,
			deliveryClass: deliveryClass,
		}
	}

	tx, err := projection.database.BeginTx(
		ctx,
		&sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return nil, replayUnavailable("begin projection", err)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := replayDatabaseNow(ctx, tx)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO realtime_cursor_states (id, last_sequence, updated_at)
		VALUES ($1, 0, $2)
		ON CONFLICT (id) DO NOTHING`, realtimeCursorStateID, now); err != nil {
		return nil, replayUnavailable("initialize global cursor state", err)
	}
	var originalHead int64
	if err := tx.QueryRowContext(ctx, `
		SELECT last_sequence
		FROM realtime_cursor_states
		WHERE id = $1
		FOR UPDATE`, realtimeCursorStateID).Scan(&originalHead); err != nil {
		return nil, replayUnavailable("lock global cursor state", err)
	}
	if originalHead < 0 {
		return nil, realtime.ErrReplayConflict
	}

	head := originalHead
	projected := make([]realtime.ProjectedMessage, len(prepared))
	for index, candidate := range prepared {
		existingSequence, existingHash, readErr := readProjectedEvidence(
			ctx,
			tx,
			candidate.routed.Topic,
			candidate.routed.Message.Envelope.ID,
		)
		if readErr == nil {
			if existingSequence > head ||
				subtle.ConstantTimeCompare(existingHash, candidate.evidenceHash[:]) != 1 {
				return nil, realtime.ErrReplayConflict
			}
			message, cursorErr := withReplayCursor(
				candidate.routed.Message,
				existingSequence,
			)
			if cursorErr != nil {
				return nil, cursorErr
			}
			projected[index] = realtime.ProjectedMessage{
				Message:  message,
				Inserted: false,
			}
			continue
		}
		if !errors.Is(readErr, sql.ErrNoRows) {
			return nil, readErr
		}

		head++
		if head < 1 {
			return nil, realtime.ErrReplayConflict
		}
		rowID := uuid.NewSHA1(
			uuid.NameSpaceOID,
			[]byte(candidate.routed.Topic+"\x00"+candidate.routed.Message.Envelope.ID),
		)
		var coalesceKey any
		if candidate.routed.Message.CoalesceKey != "" {
			coalesceKey = candidate.routed.Message.CoalesceKey
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO realtime_events (
				id, event_id, topic, sequence, type, aggregate_version,
				occurred_at, data, delivery_class, coalesce_key,
				evidence_hash, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12)`,
			rowID,
			candidate.routed.Message.Envelope.ID,
			candidate.routed.Topic,
			head,
			candidate.routed.Message.Envelope.Type,
			candidate.routed.Message.Envelope.AggregateVersion,
			candidate.routed.Message.Envelope.OccurredAt.UTC(),
			string(candidate.routed.Message.Envelope.Data),
			candidate.deliveryClass,
			coalesceKey,
			candidate.evidenceHash[:],
			now,
		); err != nil {
			return nil, replayUnavailable("insert replay event", err)
		}
		message, cursorErr := withReplayCursor(candidate.routed.Message, head)
		if cursorErr != nil {
			return nil, cursorErr
		}
		projected[index] = realtime.ProjectedMessage{Message: message, Inserted: true}
	}

	if head != originalHead {
		advance, err := tx.ExecContext(ctx, `
			UPDATE realtime_cursor_states
			SET last_sequence = $2, updated_at = $3
			WHERE id = $1 AND last_sequence = $4`,
			realtimeCursorStateID,
			head,
			now,
			originalHead,
		)
		if err != nil {
			return nil, replayUnavailable("advance global cursor state", err)
		}
		advanced, err := advance.RowsAffected()
		if err != nil {
			return nil, replayUnavailable("inspect global cursor advance", err)
		}
		if advanced != 1 {
			return nil, realtime.ErrReplayConflict
		}
	}
	countThreshold := head - int64(projection.config.MaximumEvents)
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM realtime_events
		WHERE sequence <= $1 OR created_at < $2`,
		countThreshold,
		now.Add(-projection.config.MaximumAge),
	); err != nil {
		return nil, replayUnavailable("prune replay events", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, replayUnavailable("commit projection batch", err)
	}
	return projected, nil
}

func (projection *ReplayProjection) Replay(
	ctx context.Context,
	topic string,
	afterCursor string,
	requestedLimit int,
) (realtime.ReplayPage, error) {
	if projection == nil || projection.database == nil || ctx == nil {
		return realtime.ReplayPage{}, realtime.ErrInvalidConfig
	}
	if _, err := realtime.ParseTopic(topic); err != nil {
		return realtime.ReplayPage{}, realtime.ErrInvalidReplayRequest
	}
	limit, err := realtime.NormalizeReplayLimit(projection.config, requestedLimit)
	if err != nil {
		return realtime.ReplayPage{}, err
	}
	afterSequence := int64(0)
	if afterCursor != "" {
		afterSequence, err = realtime.DecodeReplayCursor(afterCursor)
		if err != nil {
			return realtime.ReplayPage{}, err
		}
	}

	tx, err := projection.database.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return realtime.ReplayPage{}, replayUnavailable("begin replay", err)
	}
	defer func() { _ = tx.Rollback() }()
	var headSequence int64
	err = tx.QueryRowContext(ctx, `
		SELECT last_sequence
		FROM realtime_cursor_states
		WHERE id = $1`, realtimeCursorStateID).Scan(&headSequence)
	if errors.Is(err, sql.ErrNoRows) {
		if afterCursor != "" {
			return commitReplayPage(tx, realtime.ReplayPage{ResyncRequired: true})
		}
		return commitReplayPage(tx, realtime.ReplayPage{})
	}
	if err != nil {
		return realtime.ReplayPage{}, replayUnavailable("read replay head", err)
	}
	headCursor := ""
	if headSequence > 0 {
		headCursor, err = realtime.EncodeReplayCursor(headSequence)
		if err != nil {
			return realtime.ReplayPage{}, realtime.ErrReplayConflict
		}
	}
	var minimum sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
		SELECT MIN(sequence)
		FROM realtime_events`).Scan(&minimum); err != nil {
		return realtime.ReplayPage{}, replayUnavailable("read replay retention floor", err)
	}
	if afterCursor != "" &&
		(afterSequence > headSequence ||
			(minimum.Valid && afterSequence < minimum.Int64-1) ||
			(!minimum.Valid && afterSequence < headSequence)) {
		return commitReplayPage(tx, realtime.ReplayPage{
			HeadCursor:     headCursor,
			ResyncRequired: true,
		})
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT event_id, sequence, type, aggregate_version, occurred_at,
		       data, delivery_class, coalesce_key
		FROM realtime_events
		WHERE topic = $1 AND sequence > $2
		ORDER BY sequence ASC
		LIMIT $3`, topic, afterSequence, limit+1)
	if err != nil {
		return realtime.ReplayPage{}, replayUnavailable("read replay events", err)
	}
	defer func() { _ = rows.Close() }()
	messages := make([]commonwebsocket.Message, 0, limit+1)
	for rows.Next() {
		message, err := scanReplayMessage(rows, topic)
		if err != nil {
			return realtime.ReplayPage{}, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return realtime.ReplayPage{}, replayUnavailable("iterate replay events", err)
	}
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	return commitReplayPage(tx, realtime.ReplayPage{
		Messages:   messages,
		HeadCursor: headCursor,
		HasMore:    hasMore,
	})
}

type replayRowScanner interface {
	Scan(...any) error
}

func scanReplayMessage(scanner replayRowScanner, topic string) (commonwebsocket.Message, error) {
	var (
		eventID          string
		sequence         int64
		eventType        string
		aggregateVersion int64
		occurredAt       time.Time
		data             []byte
		deliveryClass    string
		coalesceKey      sql.NullString
	)
	if err := scanner.Scan(
		&eventID,
		&sequence,
		&eventType,
		&aggregateVersion,
		&occurredAt,
		&data,
		&deliveryClass,
		&coalesceKey,
	); err != nil {
		return commonwebsocket.Message{}, replayUnavailable("scan replay event", err)
	}
	class, err := decodeDeliveryClass(deliveryClass)
	if err != nil {
		return commonwebsocket.Message{}, err
	}
	cursor, err := realtime.EncodeReplayCursor(sequence)
	if err != nil {
		return commonwebsocket.Message{}, realtime.ErrReplayConflict
	}
	message := commonwebsocket.Message{
		Envelope: commonwebsocket.Envelope{
			Version:          commonwebsocket.ProtocolVersion,
			Operation:        commonwebsocket.OperationEvent,
			ID:               eventID,
			Cursor:           cursor,
			Topic:            topic,
			Type:             eventType,
			AggregateVersion: aggregateVersion,
			OccurredAt:       occurredAt.UTC(),
			Data:             append(json.RawMessage(nil), data...),
		},
		Class: class,
	}
	if coalesceKey.Valid {
		message.CoalesceKey = coalesceKey.String
	}
	if err := message.Validate(); err != nil {
		return commonwebsocket.Message{}, errors.Join(realtime.ErrReplayConflict, err)
	}
	return message, nil
}

func replayDatabaseNow(ctx context.Context, tx *sql.Tx) (time.Time, error) {
	var now time.Time
	if err := tx.QueryRowContext(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return time.Time{}, replayUnavailable("read database clock", err)
	}
	return now.UTC(), nil
}

func readProjectedEvidence(
	ctx context.Context,
	tx *sql.Tx,
	topic string,
	eventID string,
) (int64, []byte, error) {
	var sequence int64
	var evidenceHash []byte
	if err := tx.QueryRowContext(ctx, `
		SELECT sequence, evidence_hash
		FROM realtime_events
		WHERE topic = $1 AND event_id = $2`, topic, eventID).
		Scan(&sequence, &evidenceHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil, sql.ErrNoRows
		}
		return 0, nil, replayUnavailable("read projected evidence", err)
	}
	if sequence < 1 || len(evidenceHash) != sha256.Size {
		return 0, nil, realtime.ErrReplayConflict
	}
	return sequence, evidenceHash, nil
}

func replayEvidenceHash(routed realtime.RoutedMessage) ([sha256.Size]byte, error) {
	var empty [sha256.Size]byte
	var canonicalData any
	decoder := json.NewDecoder(bytes.NewReader(routed.Message.Envelope.Data))
	decoder.UseNumber()
	if err := decoder.Decode(&canonicalData); err != nil {
		return empty, errors.Join(realtime.ErrInvalidChange, err)
	}
	data, err := json.Marshal(struct {
		Topic            string `json:"topic"`
		EventID          string `json:"event_id"`
		Type             string `json:"type"`
		AggregateVersion int64  `json:"aggregate_version"`
		OccurredAt       string `json:"occurred_at"`
		Data             any    `json:"data"`
		DeliveryClass    uint8  `json:"delivery_class"`
		CoalesceKey      string `json:"coalesce_key"`
	}{
		Topic:            routed.Topic,
		EventID:          routed.Message.Envelope.ID,
		Type:             routed.Message.Envelope.Type,
		AggregateVersion: routed.Message.Envelope.AggregateVersion,
		OccurredAt:       routed.Message.Envelope.OccurredAt.UTC().Format(time.RFC3339Nano),
		Data:             canonicalData,
		DeliveryClass:    uint8(routed.Message.Class),
		CoalesceKey:      routed.Message.CoalesceKey,
	})
	if err != nil {
		return empty, errors.Join(realtime.ErrInvalidChange, err)
	}
	return sha256.Sum256(data), nil
}

func withReplayCursor(
	message commonwebsocket.Message,
	sequence int64,
) (commonwebsocket.Message, error) {
	cursor, err := realtime.EncodeReplayCursor(sequence)
	if err != nil {
		return commonwebsocket.Message{}, err
	}
	message.Envelope.Cursor = cursor
	if err := message.Validate(); err != nil {
		return commonwebsocket.Message{}, errors.Join(realtime.ErrReplayConflict, err)
	}
	return message, nil
}

func encodeDeliveryClass(class commonwebsocket.DeliveryClass) (string, error) {
	switch class {
	case commonwebsocket.DeliveryLatestState:
		return "latest_state", nil
	case commonwebsocket.DeliveryEphemeral:
		return "ephemeral", nil
	case commonwebsocket.DeliveryCritical:
		return "critical", nil
	default:
		return "", realtime.ErrInvalidChange
	}
}

func decodeDeliveryClass(value string) (commonwebsocket.DeliveryClass, error) {
	switch value {
	case "latest_state":
		return commonwebsocket.DeliveryLatestState, nil
	case "ephemeral":
		return commonwebsocket.DeliveryEphemeral, nil
	case "critical":
		return commonwebsocket.DeliveryCritical, nil
	default:
		return 0, realtime.ErrReplayConflict
	}
}

func commitReplayPage(
	tx *sql.Tx,
	page realtime.ReplayPage,
) (realtime.ReplayPage, error) {
	if err := tx.Commit(); err != nil {
		return realtime.ReplayPage{}, replayUnavailable("commit replay snapshot", err)
	}
	return page, nil
}

func replayUnavailable(operation string, err error) error {
	return fmt.Errorf("%w: %s: %w", realtime.ErrReplayUnavailable, operation, err)
}

var _ realtime.ReplayProjection = (*ReplayProjection)(nil)
