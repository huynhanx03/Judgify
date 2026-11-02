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
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authrefreshreceipt"
	"github.com/huynhanx03/judgify/internal/ent/generate/authrefreshtoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsession"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestRefreshRotationAcceptsSessionIdleExpiryAdvancedByLastSeen(t *testing.T) {
	now := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	tokenID := uuid.New()
	expected := ports.RefreshContext{
		TokenID: tokenID, SessionID: sessionID, UserID: userID,
		Username: "alice", SecurityVersion: 7,
		AuthenticatedAt: now.Add(-time.Minute),
		IdleExpiresAt:   now.Add(time.Hour), AbsoluteExpiresAt: now.Add(24 * time.Hour),
		RefreshExpiresAt: now.Add(48 * time.Hour),
	}
	err := validateLockedRefreshContext(
		&generate.AuthRefreshToken{
			ID: tokenID, SessionID: sessionID, Status: authrefreshtoken.StatusIssued,
			ExpiresAt: expected.RefreshExpiresAt,
		},
		&generate.AuthSession{
			ID: sessionID, UserID: userID, UserSecurityVersion: int64(expected.SecurityVersion),
			Status: authsession.StatusActive, LastReauthenticatedAt: expected.AuthenticatedAt,
			IdleExpiresAt:     expected.IdleExpiresAt.Add(5 * time.Minute),
			AbsoluteExpiresAt: expected.AbsoluteExpiresAt,
		},
		&generate.User{
			ID: userID, Username: expected.Username, Status: user.StatusActive,
			SecurityVersion: int64(expected.SecurityVersion),
		},
		expected,
		now,
	)
	require.NoError(t, err)
}

func TestRefreshRotationLocksUserSessionThenToken(t *testing.T) {
	client, mock := newReceiptCleanupMockClient(t)
	now := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	tokenID := uuid.New()
	hash := make([]byte, refreshTokenHashBytes)
	for index := range hash {
		hash[index] = byte(index + 1)
	}
	expected := ports.RefreshContext{
		TokenID: tokenID, SessionID: sessionID, UserID: userID,
		Username: "alice", SecurityVersion: 7,
		AuthenticatedAt: now.Add(-time.Minute),
		IdleExpiresAt:   now.Add(time.Hour), AbsoluteExpiresAt: now.Add(24 * time.Hour),
		RefreshExpiresAt: now.Add(48 * time.Hour),
	}

	// The order itself is the concurrency contract shared with session
	// management: every command locks user -> session -> credential.
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			user.FieldID, user.FieldUsername, user.FieldStatus,
			user.FieldSecurityVersion, user.FieldDeletedAt,
		}).AddRow(userID, expected.Username, user.StatusActive, int64(expected.SecurityVersion), nil))
	mock.ExpectQuery(`SELECT .* FROM "auth_sessions" WHERE .*"id".*"user_id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			authsession.FieldID, authsession.FieldUserID, authsession.FieldUserSecurityVersion,
			authsession.FieldStatus, authsession.FieldLastReauthenticatedAt,
			authsession.FieldIdleExpiresAt, authsession.FieldAbsoluteExpiresAt,
		}).AddRow(
			sessionID, userID, int64(expected.SecurityVersion), authsession.StatusActive,
			expected.AuthenticatedAt, expected.IdleExpiresAt, expected.AbsoluteExpiresAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "auth_refresh_tokens" WHERE .*"id".*"session_id".*"token_hash".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			authrefreshtoken.FieldID, authrefreshtoken.FieldSessionID,
			authrefreshtoken.FieldStatus, authrefreshtoken.FieldExpiresAt,
		}).AddRow(tokenID, sessionID, authrefreshtoken.StatusIssued, expected.RefreshExpiresAt))

	presented, session, owner, err := lockRefreshRotationState(
		context.Background(), client, expected, hash,
	)
	require.NoError(t, err)
	require.Equal(t, tokenID, presented.ID)
	require.Equal(t, sessionID, session.ID)
	require.Equal(t, userID, owner.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshRotationNeverRecoversReceiptAfterLockedSessionFence(t *testing.T) {
	client, mock := newReceiptCleanupMockClient(t)
	repository := NewRefreshRotationRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	now := time.Date(2026, 7, 26, 9, 30, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	tokenID := uuid.New()
	receiptID := uuid.New()
	replacementTokenID := uuid.New()
	idempotencyKey := uuid.New()
	hash := make([]byte, refreshTokenHashBytes)
	for index := range hash {
		hash[index] = byte(index + 1)
	}
	expected := ports.RefreshContext{
		TokenID: tokenID, SessionID: sessionID, UserID: userID,
		Username: "alice", SecurityVersion: 7,
		AuthenticatedAt: now.Add(-time.Minute),
		IdleExpiresAt:   now.Add(time.Hour), AbsoluteExpiresAt: now.Add(24 * time.Hour),
		RefreshExpiresAt: now.Add(48 * time.Hour),
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			user.FieldID, user.FieldUsername, user.FieldStatus,
			user.FieldSecurityVersion, user.FieldDeletedAt,
		}).AddRow(userID, expected.Username, user.StatusActive, int64(expected.SecurityVersion), nil))
	mock.ExpectQuery(`SELECT .* FROM "auth_sessions" WHERE .*"id".*"user_id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			authsession.FieldID, authsession.FieldUserID, authsession.FieldUserSecurityVersion,
			authsession.FieldStatus, authsession.FieldLastReauthenticatedAt,
			authsession.FieldIdleExpiresAt, authsession.FieldAbsoluteExpiresAt,
		}).AddRow(
			sessionID, userID, int64(expected.SecurityVersion), authsession.StatusRevoked,
			expected.AuthenticatedAt, expected.IdleExpiresAt, expected.AbsoluteExpiresAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "auth_refresh_tokens" WHERE .*"id".*"session_id".*"token_hash".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			authrefreshtoken.FieldID, authrefreshtoken.FieldSessionID,
			authrefreshtoken.FieldStatus, authrefreshtoken.FieldExpiresAt,
		}).AddRow(tokenID, sessionID, authrefreshtoken.StatusConsumed, expected.RefreshExpiresAt))
	mock.ExpectQuery(`SELECT .* FROM "auth_refresh_receipts" WHERE .* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows(authrefreshreceipt.Columns).AddRow(
			receiptID, now.Add(-time.Minute), now.Add(-time.Minute), sessionID, tokenID,
			idempotencyKey, replacementTokenID, []byte{0x01}, "test-key", now.Add(time.Minute), nil, nil,
		))
	mock.ExpectRollback()

	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		_, rotateErr := repository.Rotate(txCtx, ports.RefreshRotationMutation{
			PresentedHash: hash,
			Expected:      expected,
			Replacement:   &entity.RefreshToken{},
			Receipt: &entity.RefreshReceipt{
				IdempotencyKey: idempotencyKey,
			},
		}, now)
		return rotateErr
	})
	require.ErrorContains(t, err, "live state changed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshRotationContainsConsumedReplayWhenExpectedContextDrifts(t *testing.T) {
	client, mock := newReceiptCleanupMockClient(t)
	repository := NewRefreshRotationRepository(dbent.WrapClient(client, zap.NewNop()))
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	tokenID := uuid.New()
	hash := make([]byte, refreshTokenHashBytes)
	for index := range hash {
		hash[index] = byte(index + 1)
	}
	expected := ports.RefreshContext{
		TokenID: tokenID, SessionID: sessionID, UserID: userID,
		Username: "alice", SecurityVersion: 7,
		AuthenticatedAt: now.Add(-time.Minute),
		IdleExpiresAt:   now.Add(time.Hour), AbsoluteExpiresAt: now.Add(24 * time.Hour),
		RefreshExpiresAt: now.Add(48 * time.Hour),
	}
	advancedIdleExpiry := expected.IdleExpiresAt.Add(5 * time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .*"id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			user.FieldID, user.FieldUsername, user.FieldStatus,
			user.FieldSecurityVersion, user.FieldDeletedAt,
		}).AddRow(userID, expected.Username, user.StatusActive, int64(expected.SecurityVersion), nil))
	mock.ExpectQuery(`SELECT .* FROM "auth_sessions" WHERE .*"id".*"user_id".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			authsession.FieldID, authsession.FieldUserID, authsession.FieldUserSecurityVersion,
			authsession.FieldStatus, authsession.FieldLastReauthenticatedAt,
			authsession.FieldIdleExpiresAt, authsession.FieldAbsoluteExpiresAt,
		}).AddRow(
			sessionID, userID, int64(expected.SecurityVersion), authsession.StatusActive,
			expected.AuthenticatedAt, advancedIdleExpiry, expected.AbsoluteExpiresAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "auth_refresh_tokens" WHERE .*"id".*"session_id".*"token_hash".* LIMIT 2 FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			authrefreshtoken.FieldID, authrefreshtoken.FieldSessionID,
			authrefreshtoken.FieldStatus, authrefreshtoken.FieldExpiresAt,
		}).AddRow(tokenID, sessionID, authrefreshtoken.StatusConsumed, expected.RefreshExpiresAt))
	mock.ExpectQuery(`SELECT .* FROM "auth_refresh_receipts" WHERE .* LIMIT 2`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(`UPDATE "auth_refresh_tokens" SET .*"status".*"revoked_at".* WHERE .*"session_id".*"status".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "auth_sessions" SET .*"status".*"revoked_at".*"revoke_reason".* WHERE .*"id".*"status".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var result entity.RefreshRotationResult
	var rotationErr error
	transactionErr := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		result, rotationErr = repository.Rotate(txCtx, ports.RefreshRotationMutation{
			PresentedHash: hash,
			Expected:      expected,
			Replacement:   &entity.RefreshToken{},
			Receipt: &entity.RefreshReceipt{
				IdempotencyKey: uuid.New(),
			},
		}, now)
		// Keep the transaction harness deterministic so the assertion below reports
		// the repository's branch error directly during the red TDD phase.
		return nil
	})
	require.NoError(t, rotationErr)
	require.NoError(t, transactionErr)
	require.Equal(t, entity.RefreshRotationReplayContained, result.Outcome)
	require.Equal(t, sessionID, result.SessionID)
	require.NoError(t, mock.ExpectationsWereMet())
}
