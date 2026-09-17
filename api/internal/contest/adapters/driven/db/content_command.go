package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const maximumContentCommandProjectionBytes = 1 << 20

// ReserveCommand serializes one idempotency identity before any aggregate
// write. The unique insert waits for a concurrent transaction to commit, then
// returns its exact durable result or rejects a changed payload or actor.
func (repository *ContentRepository) ReserveCommand(
	ctx context.Context,
	receipt ports.ContentCommandReceipt,
) (ports.ContentCommandReservation, error) {
	if !validContentReceipt(receipt) {
		return ports.ContentCommandReservation{},
			ports.ErrInvalidContestContentCommand
	}
	result, err := repository.client.DB(ctx).Contest.ExecContext(
		ctx,
		`INSERT INTO contest_content_commands
			(id, contest_id, command_kind, idempotency_key, request_hash,
			 actor_user_id, actor_session_id, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,clock_timestamp())
		 ON CONFLICT (command_kind,idempotency_key) DO NOTHING`,
		uuid.New(),
		receipt.ContestID,
		receipt.Kind,
		receipt.IdempotencyKey,
		receipt.RequestHash[:],
		receipt.ActorUserID,
		receipt.ActorSessionID,
	)
	if err != nil {
		return ports.ContentCommandReservation{},
			fmt.Errorf("reserve contest content command: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ports.ContentCommandReservation{},
			fmt.Errorf("inspect contest content command reservation: %w", err)
	}
	if affected == 1 {
		return ports.ContentCommandReservation{Receipt: receipt}, nil
	}

	rows, err := repository.client.DB(ctx).Contest.QueryContext(
		ctx,
		`SELECT contest_id, command_kind, idempotency_key, request_hash,
		        actor_user_id, actor_session_id, result_owner_type,
		        result_content_id, result_revision_id, result_version,
		        result_projection
		   FROM contest_content_commands
		  WHERE command_kind=$1 AND idempotency_key=$2
		  FOR UPDATE`,
		receipt.Kind,
		receipt.IdempotencyKey,
	)
	if err != nil {
		return ports.ContentCommandReservation{},
			fmt.Errorf("read contest content command receipt: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return ports.ContentCommandReservation{},
			ports.ErrInvalidContestContentCommand
	}
	stored, err := scanContentReceipt(rows)
	if err != nil {
		return ports.ContentCommandReservation{}, err
	}
	if rows.Next() {
		return ports.ContentCommandReservation{},
			ports.ErrInvalidContestContentCommand
	}
	if err := rows.Err(); err != nil {
		return ports.ContentCommandReservation{}, err
	}
	if stored.ContestID != receipt.ContestID ||
		stored.RequestHash != receipt.RequestHash ||
		stored.ActorUserID != receipt.ActorUserID ||
		stored.ActorSessionID != receipt.ActorSessionID {
		return ports.ContentCommandReservation{},
			entity.ErrContestContentVersionConflict
	}
	if !validCompletedContentReceipt(stored) {
		return ports.ContentCommandReservation{},
			ports.ErrInvalidContestContentCommand
	}
	return ports.ContentCommandReservation{
		ExactReplay: true,
		Receipt:     stored,
	}, nil
}

func (repository *ContentRepository) CompleteCommand(
	ctx context.Context,
	receipt ports.ContentCommandReceipt,
) error {
	if !validCompletedContentReceipt(receipt) {
		return ports.ErrInvalidContestContentCommand
	}
	result, err := repository.client.DB(ctx).Contest.ExecContext(
		ctx,
		`UPDATE contest_content_commands
		    SET result_owner_type=$1,
		        result_content_id=$2,
		        result_revision_id=$3,
		        result_version=$4,
		        result_projection=$5
		  WHERE command_kind=$6
		    AND idempotency_key=$7
		    AND contest_id=$8
		    AND request_hash=$9
		    AND actor_user_id=$10
		    AND actor_session_id=$11
		    AND result_content_id IS NULL
		    AND result_projection IS NULL`,
		receipt.ResultOwner,
		receipt.ResultContentID,
		receipt.ResultRevisionID,
		receipt.ResultVersion,
		receipt.ResultProjection,
		receipt.Kind,
		receipt.IdempotencyKey,
		receipt.ContestID,
		receipt.RequestHash[:],
		receipt.ActorUserID,
		receipt.ActorSessionID,
	)
	if err != nil {
		return fmt.Errorf("complete contest content command: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect contest content command completion: %w", err)
	}
	if affected != 1 {
		return ports.ErrInvalidContestContentCommand
	}
	return nil
}

type contentReceiptScanner interface {
	Scan(...any) error
}

func scanContentReceipt(
	row contentReceiptScanner,
) (ports.ContentCommandReceipt, error) {
	var receipt ports.ContentCommandReceipt
	var kind, owner sql.NullString
	var requestHash, projection []byte
	var revision uuid.NullUUID
	var content uuid.NullUUID
	var version sql.NullInt64
	if err := row.Scan(
		&receipt.ContestID,
		&kind,
		&receipt.IdempotencyKey,
		&requestHash,
		&receipt.ActorUserID,
		&receipt.ActorSessionID,
		&owner,
		&content,
		&revision,
		&version,
		&projection,
	); err != nil {
		return receipt, err
	}
	receipt.Kind = ports.ContentCommandKind(kind.String)
	if len(requestHash) != len(receipt.RequestHash) {
		return receipt, ports.ErrInvalidContestContentCommand
	}
	copy(receipt.RequestHash[:], requestHash)
	if owner.Valid {
		receipt.ResultOwner = entity.ContentOwnerType(owner.String)
	}
	if content.Valid {
		receipt.ResultContentID = content.UUID
	}
	if revision.Valid {
		value := revision.UUID
		receipt.ResultRevisionID = &value
	}
	if version.Valid {
		receipt.ResultVersion = version.Int64
	}
	receipt.ResultProjection = bytes.Clone(projection)
	if !validContentReceipt(receipt) {
		return receipt, ports.ErrInvalidContestContentCommand
	}
	return receipt, nil
}

func validContentReceipt(value ports.ContentCommandReceipt) bool {
	return value.ContestID != uuid.Nil &&
		value.Kind.Valid() &&
		value.IdempotencyKey != uuid.Nil &&
		value.RequestHash != ([32]byte{}) &&
		value.ActorUserID != uuid.Nil &&
		value.ActorSessionID != uuid.Nil
}

func validCompletedContentReceipt(value ports.ContentCommandReceipt) bool {
	return validContentReceipt(value) &&
		value.ResultOwner.Valid() &&
		value.ResultContentID != uuid.Nil &&
		value.ResultVersion >= 1 &&
		(value.ResultRevisionID == nil ||
			*value.ResultRevisionID != uuid.Nil) &&
		validContentProjection(value.ResultProjection)
}

func validContentProjection(value []byte) bool {
	return len(value) >= 2 &&
		len(value) <= maximumContentCommandProjectionBytes &&
		json.Valid(value)
}
