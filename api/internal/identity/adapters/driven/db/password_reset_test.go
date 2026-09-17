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
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestPasswordResetMutationsRequireCallerTransaction(t *testing.T) {
	client, mock := newRecoveryContactMockClient(t)
	repository := NewPasswordResetRepository(dbent.WrapClient(client, zap.NewNop())).(*PasswordResetRepository)
	issuedAt := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	token := entity.KeyedTokenHash{KeyID: "reset-hmac-v1", Hash: make([]byte, 32)}
	err := repository.Issue(context.Background(), ports.PasswordResetIssue{
		ID:                    uuid.New(),
		UserID:                uuid.New(),
		ContactID:             uuid.New(),
		ContactVersion:        1,
		UserSecurityVersion:   1,
		Token:                 token,
		IssuedAt:              issuedAt,
		ExpiresAt:             issuedAt.Add(30 * time.Minute),
		ShortWindowStart:      issuedAt.Add(-15 * time.Minute),
		DailyWindowStart:      issuedAt.Add(-24 * time.Hour),
		MaximumPerShortWindow: 3,
		MaximumPerDay:         10,
	})

	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)

	_, err = repository.LockIssued(context.Background(), []entity.KeyedTokenHash{token}, issuedAt)
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)

	err = repository.Consume(context.Background(), &entity.PasswordResetIntent{
		ID: uuid.New(), UserID: uuid.New(), ContactID: uuid.New(),
		ContactVersion: 1, UserSecurityVersion: 1,
		ExpiresAt: issuedAt.Add(30 * time.Minute),
	}, issuedAt)
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPasswordResetSupersedeCancelsAndErasesPendingDelivery(t *testing.T) {
	client, mock := newRecoveryContactMockClient(t)
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	tokenID := uuid.New()
	deliveryID := uuid.New()
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .*"id".* FROM "security_email_deliveries" WHERE .*"password_reset_token_id".*"status".* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(deliveryID))
	mock.ExpectExec(`UPDATE "password_reset_tokens" SET .*"status".*"superseded_at".* WHERE .*"id".*"status".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "security_email_deliveries" SET .*"payload_ciphertext".*"status".*"cancelled_at".*"ciphertext_erased_at".* WHERE .*"id".*"status".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		return supersedePasswordResetTokens(txCtx, client, []uuid.UUID{tokenID}, now)
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

var _ ports.PasswordResetRepository = (*PasswordResetRepository)(nil)
