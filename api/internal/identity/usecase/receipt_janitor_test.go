package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/huynhanx03/judgify/internal/ports"
)

type receiptJanitorRepositoryStub struct {
	mu        sync.Mutex
	result    ports.ReceiptCleanupResult
	err       error
	calls     int
	requests  []ports.ReceiptCleanupRequest
	notifyAt  int
	notified  chan struct{}
	notifyOne sync.Once
}

func (repository *receiptJanitorRepositoryStub) EraseExpired(
	ctx context.Context,
	request ports.ReceiptCleanupRequest,
) (ports.ReceiptCleanupResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.ReceiptCleanupResult{}, err
	}
	repository.mu.Lock()
	repository.calls++
	repository.requests = append(repository.requests, request)
	calls := repository.calls
	result := repository.result
	err := repository.err
	repository.mu.Unlock()
	if repository.notified != nil && calls >= repository.notifyAt {
		repository.notifyOne.Do(func() { close(repository.notified) })
	}
	return result, err
}

type receiptJanitorTx struct{}

func (receiptJanitorTx) DoInTx(
	ctx context.Context,
	callback func(context.Context) error,
) error {
	return callback(ctx)
}

func TestReceiptJanitorRunsImmediateBoundedBatchesAndReportsHealth(t *testing.T) {
	notified := make(chan struct{})
	repository := &receiptJanitorRepositoryStub{
		result: ports.ReceiptCleanupResult{
			SessionIssuanceErased:                  2,
			RefreshRotationErased:                  2,
			RecoveryCommandReceiptsDeleted:         2,
			SessionCommandReceiptsDeleted:          2,
			VerificationTokensExpired:              2,
			PasswordResetTokensExpired:             2,
			SecurityEmailDeliveriesCancelled:       2,
			PendingRecoveryContactsRevoked:         2,
			RevokedRecoveryContactsErased:          2,
			TerminalSecurityEmailDeliveriesDeleted: 2,
			TerminalVerificationTokensDeleted:      2,
			TerminalPasswordResetTokensDeleted:     2,
			TerminalRecoveryContactsDeleted:        2,
			MayHaveMore:                            true,
		},
		notifyAt: 3,
		notified: notified,
	}
	now := time.Date(2026, 7, 19, 15, 0, 0, 0, time.UTC)
	janitor := newReceiptJanitorForTest(t, repository, func() time.Time { return now })
	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- janitor.Run(runCtx) }()

	select {
	case <-notified:
	case <-time.After(time.Second):
		t.Fatal("immediate cleanup did not finish")
	}
	require.Eventually(t, func() bool {
		return janitor.Stats().LastBatches == 3
	}, time.Second, time.Millisecond)
	stats := janitor.Stats()
	require.True(t, stats.Running)
	require.Equal(t, 3, stats.LastBatches)
	require.True(t, stats.LastBoundHit)
	require.Equal(t, 6, stats.LastSessionIssuanceErased)
	require.Equal(t, 6, stats.LastRefreshRotationErased)
	require.Equal(t, uint64(6), stats.TotalSessionIssuanceErased)
	require.Equal(t, uint64(6), stats.TotalRefreshRotationErased)
	require.Equal(t, 6, stats.LastRecoveryCommandReceiptsDeleted)
	require.Equal(t, uint64(6), stats.TotalRecoveryCommandReceiptsDeleted)
	require.Equal(t, 6, stats.LastSessionCommandReceiptsDeleted)
	require.Equal(t, uint64(6), stats.TotalSessionCommandReceiptsDeleted)
	require.Equal(t, 6, stats.LastVerificationTokensExpired)
	require.Equal(t, uint64(6), stats.TotalVerificationTokensExpired)
	require.Equal(t, 6, stats.LastPasswordResetTokensExpired)
	require.Equal(t, uint64(6), stats.TotalPasswordResetTokensExpired)
	require.Equal(t, 6, stats.LastSecurityEmailDeliveriesCancelled)
	require.Equal(t, uint64(6), stats.TotalSecurityEmailDeliveriesCancelled)
	require.Equal(t, 6, stats.LastPendingRecoveryContactsRevoked)
	require.Equal(t, uint64(6), stats.TotalPendingRecoveryContactsRevoked)
	require.Equal(t, 6, stats.LastRevokedRecoveryContactsErased)
	require.Equal(t, uint64(6), stats.TotalRevokedRecoveryContactsErased)
	require.Equal(t, 6, stats.LastTerminalSecurityEmailDeliveriesDeleted)
	require.Equal(t, uint64(6), stats.TotalTerminalSecurityEmailDeliveriesDeleted)
	require.Equal(t, 6, stats.LastTerminalVerificationTokensDeleted)
	require.Equal(t, uint64(6), stats.TotalTerminalVerificationTokensDeleted)
	require.Equal(t, 6, stats.LastTerminalPasswordResetTokensDeleted)
	require.Equal(t, uint64(6), stats.TotalTerminalPasswordResetTokensDeleted)
	require.Equal(t, 6, stats.LastTerminalRecoveryContactsDeleted)
	require.Equal(t, uint64(6), stats.TotalTerminalRecoveryContactsDeleted)
	repository.mu.Lock()
	requests := append([]ports.ReceiptCleanupRequest(nil), repository.requests...)
	repository.mu.Unlock()
	require.Len(t, requests, 3)
	for _, request := range requests {
		require.Equal(t, now, request.Now)
		require.Equal(t, now.Add(-30*24*time.Hour), request.TerminalBefore)
		require.Equal(t, 28, request.BatchSize)
	}
	require.NoError(t, janitor.Check(context.Background()))

	cancel()
	require.NoError(t, <-runDone)
	require.ErrorIs(t, janitor.Check(context.Background()), ErrReceiptJanitorHealth)
}

func TestReceiptJanitorFailureIsClassifiedWithoutRawError(t *testing.T) {
	notified := make(chan struct{})
	repository := &receiptJanitorRepositoryStub{
		err:      errors.New("database said secret-looking-value"),
		notifyAt: 1,
		notified: notified,
	}
	janitor := newReceiptJanitorForTest(t, repository, time.Now)
	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- janitor.Run(runCtx) }()

	select {
	case <-notified:
	case <-time.After(time.Second):
		t.Fatal("failed cleanup did not finish")
	}
	require.Eventually(t, func() bool {
		return janitor.Stats().ConsecutiveFailures == 1
	}, time.Second, time.Millisecond)
	stats := janitor.Stats()
	require.Equal(t, "storage", stats.LastFailureClassificationCode)
	require.NotContains(t, stats.LastFailureClassificationCode, "secret-looking-value")
	require.ErrorIs(t, janitor.Check(context.Background()), ErrReceiptJanitorHealth)

	cancel()
	require.NoError(t, <-runDone)
}

func TestReceiptJanitorShutdownBeforeRunIsIdempotent(t *testing.T) {
	janitor := newReceiptJanitorForTest(t, &receiptJanitorRepositoryStub{}, time.Now)
	require.NoError(t, janitor.Shutdown(context.Background()))
	require.NoError(t, janitor.Shutdown(context.Background()))
	require.ErrorIs(t, janitor.Run(context.Background()), ErrInvalidReceiptJanitor)
}

func TestReceiptJanitorRejectsUnboundedConfiguration(t *testing.T) {
	_, err := NewReceiptJanitor(
		&receiptJanitorRepositoryStub{},
		receiptJanitorTx{},
		ReceiptJanitorConfig{
			Interval: time.Minute, Timeout: time.Second,
			BatchSize: 1, MaxBatches: 1, TerminalRetention: 30 * 24 * time.Hour,
		},
	)
	require.ErrorIs(t, err, ErrInvalidReceiptJanitor)
}

func TestReceiptCleanupLogFieldsAreCompleteAndLowCardinality(t *testing.T) {
	result := ports.ReceiptCleanupResult{
		RecoveryCommandReceiptsDeleted:         1,
		SessionCommandReceiptsDeleted:          2,
		VerificationTokensExpired:              3,
		PasswordResetTokensExpired:             4,
		SecurityEmailDeliveriesCancelled:       5,
		PendingRecoveryContactsRevoked:         6,
		RevokedRecoveryContactsErased:          7,
		TerminalSecurityEmailDeliveriesDeleted: 8,
		TerminalVerificationTokensDeleted:      9,
		TerminalPasswordResetTokensDeleted:     10,
		TerminalRecoveryContactsDeleted:        11,
	}
	core, observed := observer.New(zap.DebugLevel)
	zap.New(core).Info("cleanup", receiptCleanupLogFields(result, 2, true, time.Second)...)
	contextMap := observed.All()[0].ContextMap()
	for _, key := range []string{
		"recovery_command_receipts_deleted",
		"session_command_receipts_deleted",
		"verification_tokens_expired",
		"password_reset_tokens_expired",
		"security_email_deliveries_cancelled",
		"pending_recovery_contacts_revoked",
		"revoked_recovery_contacts_erased",
		"terminal_security_email_deliveries_deleted",
		"terminal_verification_tokens_deleted",
		"terminal_password_reset_tokens_deleted",
		"terminal_recovery_contacts_deleted",
	} {
		require.Contains(t, contextMap, key)
	}
	for key := range contextMap {
		lower := strings.ToLower(key)
		require.NotContains(t, lower, "address")
		require.NotContains(t, lower, "ciphertext")
		require.NotContains(t, lower, "user_id")
		require.NotContains(t, lower, "contact_id")
		require.NotContains(t, lower, "token_id")
	}
}

func newReceiptJanitorForTest(
	t *testing.T,
	repository ports.ReceiptCleanupRepository,
	clock func() time.Time,
) *ReceiptJanitor {
	t.Helper()
	janitor, err := NewReceiptJanitor(
		repository,
		receiptJanitorTx{},
		ReceiptJanitorConfig{
			Interval:          time.Hour,
			Timeout:           time.Minute,
			BatchSize:         28,
			MaxBatches:        3,
			TerminalRetention: 30 * 24 * time.Hour,
			Clock:             clock,
		},
	)
	require.NoError(t, err)
	return janitor
}
