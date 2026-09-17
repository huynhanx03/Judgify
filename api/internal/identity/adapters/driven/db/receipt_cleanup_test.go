package db

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestReceiptCleanupLeasesAndErasesSessionCiphertextsAtomically(t *testing.T) {
	client, mock := newReceiptCleanupMockClient(t)
	wrapper := dbent.WrapClient(client, zap.NewNop())
	repository := NewReceiptCleanupRepository(wrapper)
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	before := time.Date(2026, 7, 19, 15, 0, 0, 0, time.UTC)
	issuanceID := uuid.New()
	refreshID := uuid.New()

	mock.ExpectBegin()
	expectEmptyRecoveryLifecycleLeases(mock)
	mock.ExpectQuery(`SELECT .* FROM "auth_session_issuance_receipts".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(issuanceID))
	mock.ExpectQuery(`SELECT .* FROM "auth_refresh_receipts".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(refreshID))
	expectEmptyCommandReceiptLeases(mock)
	mock.ExpectExec(`UPDATE "auth_session_issuance_receipts" SET .*"recovery_ciphertext".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "auth_refresh_receipts" SET .*"recovery_ciphertext".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var resultSession, resultRefresh int
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		result, err := repository.EraseExpired(txCtx, ports.ReceiptCleanupRequest{
			Now: before, TerminalBefore: before.Add(-30 * 24 * time.Hour), BatchSize: 28,
		})
		resultSession = result.SessionIssuanceErased
		resultRefresh = result.RefreshRotationErased
		return err
	})
	require.NoError(t, err)
	require.Equal(t, 1, resultSession)
	require.Equal(t, 1, resultRefresh)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReceiptCleanupRejectsMissingTransaction(t *testing.T) {
	client, mock := newReceiptCleanupMockClient(t)
	repository := NewReceiptCleanupRepository(dbent.WrapClient(client, zap.NewNop()))
	now := time.Now()
	_, err := repository.EraseExpired(context.Background(), ports.ReceiptCleanupRequest{
		Now: now, TerminalBefore: now.Add(-30 * 24 * time.Hour), BatchSize: 28,
	})
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReceiptCleanupAdvancesRecoveryLifecycleInLockAndFKOrder(t *testing.T) {
	client, mock := newReceiptCleanupMockClient(t)
	wrapper := dbent.WrapClient(client, zap.NewNop())
	repository := NewReceiptCleanupRepository(wrapper)
	unitOfWork := platformtx.NewEntUnitOfWork(client)
	now := time.Date(2026, 7, 22, 15, 0, 0, 0, time.UTC)
	terminalBefore := now.Add(-30 * 24 * time.Hour)
	pendingContactID := uuid.New()
	revokedContactID := uuid.New()
	terminalContactID := uuid.New()
	verificationTokenID := uuid.New()
	resetTokenID := uuid.New()
	terminalVerificationTokenID := uuid.New()
	terminalResetTokenID := uuid.New()
	expiredDeliveryID := uuid.New()
	terminalDeliveryID := uuid.New()
	commandReceiptID := uuid.New()
	sessionCommandReceiptID := uuid.New()

	mock.ExpectBegin()
	// Contacts are locked before their token and delivery children. Terminal
	// rows are only selected when no retained child still references them.
	mock.ExpectQuery(`SELECT .* FROM "user_recovery_contacts".*"status".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(pendingContactID))
	mock.ExpectQuery(`SELECT .* FROM "user_recovery_contacts".*"address_ciphertext".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(revokedContactID))
	mock.ExpectQuery(`SELECT .* FROM "user_recovery_contacts".*"updated_at".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(terminalContactID))
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_verification_tokens".*"expires_at".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(verificationTokenID))
	mock.ExpectQuery(`SELECT .* FROM "password_reset_tokens".*"expires_at".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(resetTokenID))
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_verification_tokens".*"updated_at".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(terminalVerificationTokenID))
	mock.ExpectQuery(`SELECT .* FROM "password_reset_tokens".*"updated_at".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(terminalResetTokenID))
	mock.ExpectQuery(`SELECT .* FROM "security_email_deliveries".*"payload_expires_at".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expiredDeliveryID))
	mock.ExpectQuery(`SELECT .* FROM "security_email_deliveries".*"updated_at".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(terminalDeliveryID))
	mock.ExpectQuery(`SELECT .* FROM "auth_session_issuance_receipts".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT .* FROM "auth_refresh_receipts".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_command_receipts".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(commandReceiptID))
	mock.ExpectQuery(`SELECT .* FROM "auth_session_command_receipts".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(sessionCommandReceiptID))

	mock.ExpectExec(`UPDATE "user_recovery_contacts" SET `).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "user_recovery_contacts" SET `).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "recovery_contact_verification_tokens" SET .*"status".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "password_reset_tokens" SET .*"status".*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "security_email_deliveries" SET `).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Reverse FK deletion order: delivery -> tokens -> contact.
	mock.ExpectExec(`DELETE FROM "security_email_deliveries"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM "recovery_contact_verification_tokens"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM "password_reset_tokens"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM "user_recovery_contacts"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM "recovery_contact_command_receipts"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM "auth_session_command_receipts"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var result ports.ReceiptCleanupResult
	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		var cleanupErr error
		result, cleanupErr = repository.EraseExpired(txCtx, ports.ReceiptCleanupRequest{
			Now: now, TerminalBefore: terminalBefore, BatchSize: 28,
		})
		return cleanupErr
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.RecoveryCommandReceiptsDeleted)
	require.Equal(t, 1, result.SessionCommandReceiptsDeleted)
	require.Equal(t, 1, result.VerificationTokensExpired)
	require.Equal(t, 1, result.PasswordResetTokensExpired)
	require.Equal(t, 1, result.SecurityEmailDeliveriesCancelled)
	require.Equal(t, 1, result.PendingRecoveryContactsRevoked)
	require.Equal(t, 1, result.RevokedRecoveryContactsErased)
	require.Equal(t, 1, result.TerminalSecurityEmailDeliveriesDeleted)
	require.Equal(t, 1, result.TerminalVerificationTokensDeleted)
	require.Equal(t, 1, result.TerminalPasswordResetTokensDeleted)
	require.Equal(t, 1, result.TerminalRecoveryContactsDeleted)
	require.False(t, result.MayHaveMore)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReceiptCleanupRollsBackWhenConditionalFenceChanges(t *testing.T) {
	client, mock := newReceiptCleanupMockClient(t)
	wrapper := dbent.WrapClient(client, zap.NewNop())
	repository := NewReceiptCleanupRepository(wrapper)
	unitOfWork := platformtx.NewEntUnitOfWork(client)

	mock.ExpectBegin()
	expectEmptyRecoveryLifecycleLeases(mock)
	mock.ExpectQuery(`SELECT .* FROM "auth_session_issuance_receipts".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectQuery(`SELECT .* FROM "auth_refresh_receipts".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	expectEmptyCommandReceiptLeases(mock)
	mock.ExpectExec(`UPDATE "auth_session_issuance_receipts" SET .*"recovery_ciphertext".*`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := unitOfWork.Within(context.Background(), func(txCtx context.Context) error {
		now := time.Now()
		_, cleanupErr := repository.EraseExpired(txCtx, ports.ReceiptCleanupRequest{
			Now: now, TerminalBefore: now.Add(-30 * 24 * time.Hour), BatchSize: 28,
		})
		return cleanupErr
	})
	require.ErrorContains(t, err, "cleanup fence changed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanupBatchSharesAreBoundedAndFair(t *testing.T) {
	t.Parallel()

	for batchSize := ports.MinimumReceiptCleanupBatchSize; batchSize <= 52; batchSize++ {
		limits := cleanupBatchShares(batchSize)
		shares := []int{
			limits.pendingContacts, limits.revokedContactSecrets, limits.terminalContacts,
			limits.verificationExpiry, limits.resetExpiry, limits.terminalVerification,
			limits.terminalReset, limits.expiredDeliveries, limits.terminalDeliveries,
			limits.sessionIssuance, limits.refreshRotation,
			limits.commandReceipts, limits.sessionCommandReceipts,
		}
		total, minimum, maximum := 0, shares[0], shares[0]
		for _, share := range shares {
			total += share
			minimum = min(minimum, share)
			maximum = max(maximum, share)
			require.Positive(t, share)
		}
		require.Equal(t, batchSize, total)
		require.LessOrEqual(t, maximum-minimum, 1)
	}
}

func newReceiptCleanupMockClient(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

func expectEmptyRecoveryLifecycleLeases(mock sqlmock.Sqlmock) {
	for _, table := range []string{
		"user_recovery_contacts",
		"user_recovery_contacts",
		"user_recovery_contacts",
		"recovery_contact_verification_tokens",
		"password_reset_tokens",
		"recovery_contact_verification_tokens",
		"password_reset_tokens",
		"security_email_deliveries",
		"security_email_deliveries",
	} {
		mock.ExpectQuery(`SELECT .* FROM "` + table + `".*FOR UPDATE SKIP LOCKED`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))
	}
}

func expectEmptyCommandReceiptLeases(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT .* FROM "recovery_contact_command_receipts".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT .* FROM "auth_session_command_receipts".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
}
