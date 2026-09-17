package db

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsession"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestSessionManagementDoesNotPersistReceiptForAlreadyTerminalTarget(t *testing.T) {
	client, mock := newReceiptCleanupMockClient(t)
	repository := NewSessionManagementRepository(dbent.WrapClient(client, zap.NewNop())).(*SessionManagementRepository)
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	actorSessionID := uuid.New()
	targetSessionID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			user.FieldID, user.FieldStatus, user.FieldSecurityVersion, user.FieldDeletedAt,
		}).AddRow(userID, user.StatusActive, int64(1), nil))
	mock.ExpectQuery(`SELECT .* FROM "auth_session_command_receipts" WHERE .* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT .* FROM "auth_sessions" WHERE .* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{authsession.FieldID}).AddRow(actorSessionID))
	mock.ExpectQuery(`SELECT .* FROM "auth_sessions" WHERE .* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			authsession.FieldID, authsession.FieldStatus,
		}).AddRow(targetSessionID, authsession.StatusRevoked))
	mock.ExpectCommit()

	var mutation ports.SessionManagementMutation
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var revokeErr error
		mutation, revokeErr = repository.RevokeOwned(
			txCtx,
			ports.SessionRevokeCommand{
				UserID: userID, ActorSessionID: actorSessionID,
				TargetSessionID: targetSessionID, IdempotencyKey: uuid.New(),
			},
			now,
			now.Add(24*time.Hour),
		)
		return revokeErr
	})
	require.NoError(t, err)
	require.False(t, mutation.Applied)
	require.Equal(t, ports.SessionCommandRevoke, mutation.Kind)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionManagementLogoutAllLocksSessionsBeforeRefreshTokens(t *testing.T) {
	client, mock := newReceiptCleanupMockClient(t)
	repository := NewSessionManagementRepository(dbent.WrapClient(client, zap.NewNop())).(*SessionManagementRepository)
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	now := time.Date(2026, 7, 26, 10, 30, 0, 0, time.UTC)
	userID := uuid.New()
	actorSessionID := uuid.New()
	idempotencyKey := uuid.New()
	receiptExpiresAt := now.Add(24 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			user.FieldID, user.FieldStatus, user.FieldSecurityVersion, user.FieldDeletedAt,
		}).AddRow(userID, user.StatusActive, int64(1), nil))
	mock.ExpectQuery(`SELECT .* FROM "auth_session_command_receipts" WHERE .* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT .* FROM "auth_sessions" WHERE .* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{authsession.FieldID}).AddRow(actorSessionID))
	mock.ExpectExec(`UPDATE "users" SET .*"security_version".* WHERE .*"id".*"security_version".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*"id".*`).
		WillReturnRows(sqlmock.NewRows(user.Columns).AddRow(
			userID, now.Add(-time.Hour), now, nil, nil, "alice", user.StatusActive, int64(2), "private", int64(1), now, nil, nil,
		))
	// This order is the PostgreSQL row-lock contract shared with token lifecycle:
	// every affected session row must be acquired before any of its token rows.
	mock.ExpectExec(`UPDATE "auth_sessions" SET .*"status".*"revoked_at".*"revoke_reason".* WHERE .*"user_id".*"status".*`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`UPDATE "auth_refresh_tokens" SET .*"status".*"revoked_at".* WHERE .*`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`INSERT INTO "auth_session_command_receipts" .*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var mutation ports.SessionManagementMutation
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var mutationErr error
		mutation, mutationErr = repository.LogoutAll(
			txCtx,
			ports.SessionLogoutAllCommand{
				UserID: userID, ActorSessionID: actorSessionID, IdempotencyKey: idempotencyKey,
			},
			now,
			receiptExpiresAt,
		)
		return mutationErr
	})
	require.NoError(t, err)
	require.True(t, mutation.Applied)
	require.Equal(t, uint64(2), mutation.SecurityVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionManagementMutationsRequireCallerTransaction(t *testing.T) {
	client, mock := newRecoveryContactMockClient(t)
	repository := NewSessionManagementRepository(dbent.WrapClient(client, zap.NewNop())).(*SessionManagementRepository)
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	actorSessionID := uuid.New()

	_, err := repository.RevokeOwned(context.Background(), ports.SessionRevokeCommand{
		UserID: userID, ActorSessionID: actorSessionID,
		TargetSessionID: uuid.New(), IdempotencyKey: uuid.New(),
	}, now, now.Add(24*time.Hour))
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)

	_, err = repository.LogoutAll(context.Background(), ports.SessionLogoutAllCommand{
		UserID: userID, ActorSessionID: actorSessionID, IdempotencyKey: uuid.New(),
	}, now, now.Add(24*time.Hour))
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionManagementRepositoryRejectsInvalidInputsBeforeDatabase(t *testing.T) {
	client, mock := newRecoveryContactMockClient(t)
	repository := NewSessionManagementRepository(dbent.WrapClient(client, zap.NewNop())).(*SessionManagementRepository)
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	_, err := repository.List(context.Background(), ports.SessionListQuery{UserID: uuid.New(), Limit: 51})
	require.ErrorIs(t, err, ports.ErrInvalidSessionManagementPersistence)
	_, err = repository.RevokeOwned(context.Background(), ports.SessionRevokeCommand{}, now, now.Add(time.Hour))
	require.ErrorIs(t, err, ports.ErrInvalidSessionManagementPersistence)
	_, err = repository.LogoutAll(context.Background(), ports.SessionLogoutAllCommand{}, now, now.Add(time.Hour))
	require.ErrorIs(t, err, ports.ErrInvalidSessionManagementPersistence)
	require.NoError(t, mock.ExpectationsWereMet())
}

var _ ports.SessionManagementRepository = (*SessionManagementRepository)(nil)
