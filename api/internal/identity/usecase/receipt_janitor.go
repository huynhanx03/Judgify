package usecase

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/huynhanx03/go-common/pkg/common/health"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	maximumReceiptCleanupInterval = 24 * time.Hour
	maximumReceiptCleanupTimeout  = 30 * time.Minute
	maximumReceiptCleanupBatches  = 100
	maximumReceiptRetention       = 10 * 365 * 24 * time.Hour
)

var (
	ErrInvalidReceiptJanitor = errors.New("identity: invalid receipt janitor")
	ErrReceiptJanitorHealth  = errors.New("identity: receipt janitor unhealthy")
)

type receiptJanitorState uint8

const (
	receiptJanitorNew receiptJanitorState = iota
	receiptJanitorRunning
	receiptJanitorStopping
	receiptJanitorStopped
)

type ReceiptJanitorConfig struct {
	Interval          time.Duration
	Timeout           time.Duration
	BatchSize         int
	MaxBatches        int
	TerminalRetention time.Duration
	Clock             func() time.Time
}

// ReceiptJanitorStats is a secret-free operational snapshot suitable for a
// health adapter or metrics collector. It intentionally contains no IDs.
type ReceiptJanitorStats struct {
	Running                                     bool
	LastAttemptAt                               time.Time
	LastSuccessAt                               time.Time
	LastDuration                                time.Duration
	LastSessionIssuanceErased                   int
	LastRefreshRotationErased                   int
	LastRecoveryCommandReceiptsDeleted          int
	LastSessionCommandReceiptsDeleted           int
	LastVerificationTokensExpired               int
	LastPasswordResetTokensExpired              int
	LastSecurityEmailDeliveriesCancelled        int
	LastPendingRecoveryContactsRevoked          int
	LastRevokedRecoveryContactsErased           int
	LastTerminalSecurityEmailDeliveriesDeleted  int
	LastTerminalVerificationTokensDeleted       int
	LastTerminalPasswordResetTokensDeleted      int
	LastTerminalRecoveryContactsDeleted         int
	LastBatches                                 int
	LastBoundHit                                bool
	ConsecutiveFailures                         uint64
	TotalSessionIssuanceErased                  uint64
	TotalRefreshRotationErased                  uint64
	TotalRecoveryCommandReceiptsDeleted         uint64
	TotalSessionCommandReceiptsDeleted          uint64
	TotalVerificationTokensExpired              uint64
	TotalPasswordResetTokensExpired             uint64
	TotalSecurityEmailDeliveriesCancelled       uint64
	TotalPendingRecoveryContactsRevoked         uint64
	TotalRevokedRecoveryContactsErased          uint64
	TotalTerminalSecurityEmailDeliveriesDeleted uint64
	TotalTerminalVerificationTokensDeleted      uint64
	TotalTerminalPasswordResetTokensDeleted     uint64
	TotalTerminalRecoveryContactsDeleted        uint64
	LastFailureClassificationCode               string
}

// ReceiptJanitor is a lifecycle-owned, single-loop privacy eraser. Run does
// not spawn any goroutine: the shared lifecycle owns the one component loop.
type ReceiptJanitor struct {
	repository ports.ReceiptCleanupRepository
	tx         tx.Manager
	config     ReceiptJanitorConfig

	lifecycleMu sync.Mutex
	state       receiptJanitorState
	runCancel   context.CancelFunc
	stop        chan struct{}
	stopOnce    sync.Once
	done        chan struct{}
	doneOnce    sync.Once

	statsMu sync.RWMutex
	stats   ReceiptJanitorStats
}

func NewReceiptJanitor(
	repository ports.ReceiptCleanupRepository,
	txManager tx.Manager,
	config ReceiptJanitorConfig,
) (*ReceiptJanitor, error) {
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if isNilTokenDependency(repository) || isNilTokenDependency(txManager) ||
		config.Interval <= 0 || config.Interval > maximumReceiptCleanupInterval ||
		config.Timeout <= 0 || config.Timeout > maximumReceiptCleanupTimeout ||
		config.Timeout > config.Interval || config.BatchSize < ports.MinimumReceiptCleanupBatchSize ||
		config.BatchSize > ports.MaximumReceiptCleanupBatchSize || config.MaxBatches <= 0 ||
		config.MaxBatches > maximumReceiptCleanupBatches || config.TerminalRetention <= 0 ||
		config.TerminalRetention > maximumReceiptRetention || config.Clock == nil {
		return nil, ErrInvalidReceiptJanitor
	}
	return &ReceiptJanitor{
		repository: repository,
		tx:         txManager,
		config:     config,
		state:      receiptJanitorNew,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}, nil
}

func (*ReceiptJanitor) Name() string { return "identity-receipt-janitor" }

func (janitor *ReceiptJanitor) Run(ctx context.Context) error {
	if janitor == nil || ctx == nil {
		return ErrInvalidReceiptJanitor
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	janitor.lifecycleMu.Lock()
	if janitor.state != receiptJanitorNew {
		janitor.lifecycleMu.Unlock()
		return ErrInvalidReceiptJanitor
	}
	runCtx, cancel := context.WithCancel(ctx)
	janitor.runCancel = cancel
	janitor.state = receiptJanitorRunning
	janitor.setRunning(true)
	janitor.lifecycleMu.Unlock()

	componentCtx := correlation.EnsureContext(runCtx)
	logger.FromContext(componentCtx).Info(
		"identity receipt janitor started",
		zap.Duration("receipt_cleanup_interval", janitor.config.Interval),
		zap.Int("receipt_cleanup_batch_size", janitor.config.BatchSize),
		zap.Int("receipt_cleanup_max_batches", janitor.config.MaxBatches),
	)
	defer func() {
		cancel()
		janitor.lifecycleMu.Lock()
		janitor.state = receiptJanitorStopped
		janitor.setRunning(false)
		janitor.doneOnce.Do(func() { close(janitor.done) })
		janitor.lifecycleMu.Unlock()
		logger.FromContext(componentCtx).Info("identity receipt janitor stopped")
	}()

	janitor.cleanup(componentCtx)
	ticker := time.NewTicker(janitor.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-runCtx.Done():
			return nil
		case <-janitor.stop:
			return nil
		case <-ticker.C:
			janitor.cleanup(componentCtx)
		}
	}
}

func (janitor *ReceiptJanitor) Shutdown(ctx context.Context) error {
	if janitor == nil || ctx == nil {
		return ErrInvalidReceiptJanitor
	}
	janitor.lifecycleMu.Lock()
	switch janitor.state {
	case receiptJanitorNew:
		janitor.state = receiptJanitorStopped
		janitor.setRunning(false)
		janitor.requestStop()
		janitor.doneOnce.Do(func() { close(janitor.done) })
	case receiptJanitorRunning:
		janitor.state = receiptJanitorStopping
		if janitor.runCancel != nil {
			janitor.runCancel()
		}
		janitor.requestStop()
	case receiptJanitorStopping, receiptJanitorStopped:
	default:
		janitor.lifecycleMu.Unlock()
		return ErrInvalidReceiptJanitor
	}
	done := janitor.done
	janitor.lifecycleMu.Unlock()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (janitor *ReceiptJanitor) cleanup(parent context.Context) {
	if parent == nil || parent.Err() != nil {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(
		correlation.EnsureContext(parent),
		janitor.config.Timeout,
	)
	defer cancel()

	attemptAt := janitor.config.Clock().UTC()
	started := time.Now()
	result := ports.ReceiptCleanupResult{}
	batches := 0
	boundHit := false
	var cleanupErr error
	for batches < janitor.config.MaxBatches {
		if err := cleanupCtx.Err(); err != nil {
			cleanupErr = err
			break
		}
		batch := ports.ReceiptCleanupResult{}
		err := janitor.tx.DoInTx(cleanupCtx, func(transactionContext context.Context) error {
			var repositoryErr error
			batch, repositoryErr = janitor.repository.EraseExpired(
				transactionContext,
				ports.ReceiptCleanupRequest{
					Now:            attemptAt,
					TerminalBefore: attemptAt.Add(-janitor.config.TerminalRetention),
					BatchSize:      janitor.config.BatchSize,
				},
			)
			return repositoryErr
		})
		if err != nil {
			cleanupErr = err
			break
		}
		batches++
		result.Add(batch)
		if !batch.MayHaveMore {
			break
		}
		if batches == janitor.config.MaxBatches {
			boundHit = true
		}
	}
	duration := time.Since(started)
	if duration < 0 {
		duration = 0
	}
	if cleanupErr != nil {
		// Lifecycle cancellation is expected and must not poison health during a
		// normal shutdown. A janitor-owned timeout remains an operational error.
		if parent.Err() != nil || errors.Is(cleanupErr, context.Canceled) && cleanupCtx.Err() == context.Canceled {
			return
		}
		classification := receiptCleanupErrorClass(cleanupErr)
		janitor.recordCleanup(attemptAt, duration, result, batches, boundHit, classification)
		fields := receiptCleanupLogFields(result, batches, boundHit, duration)
		fields = append(fields, zap.String("error_class", classification))
		logger.FromContext(cleanupCtx).Warn(
			"identity receipt cleanup failed",
			fields...,
		)
		return
	}

	janitor.recordCleanup(attemptAt, duration, result, batches, boundHit, "")
	log := logger.FromContext(cleanupCtx)
	fields := receiptCleanupLogFields(result, batches, boundHit, duration)
	if result.Total() > 0 || boundHit {
		log.Info("identity receipt cleanup completed", fields...)
		return
	}
	log.Debug("identity receipt cleanup completed", fields...)
}

func receiptCleanupLogFields(
	result ports.ReceiptCleanupResult,
	batches int,
	boundHit bool,
	duration time.Duration,
) []zap.Field {
	return []zap.Field{
		zap.Int("receipt_cleanup_batches", batches),
		zap.Int("session_issuance_receipts_erased", result.SessionIssuanceErased),
		zap.Int("refresh_rotation_receipts_erased", result.RefreshRotationErased),
		zap.Int("recovery_command_receipts_deleted", result.RecoveryCommandReceiptsDeleted),
		zap.Int("session_command_receipts_deleted", result.SessionCommandReceiptsDeleted),
		zap.Int("verification_tokens_expired", result.VerificationTokensExpired),
		zap.Int("password_reset_tokens_expired", result.PasswordResetTokensExpired),
		zap.Int("security_email_deliveries_cancelled", result.SecurityEmailDeliveriesCancelled),
		zap.Int("pending_recovery_contacts_revoked", result.PendingRecoveryContactsRevoked),
		zap.Int("revoked_recovery_contacts_erased", result.RevokedRecoveryContactsErased),
		zap.Int("terminal_security_email_deliveries_deleted", result.TerminalSecurityEmailDeliveriesDeleted),
		zap.Int("terminal_verification_tokens_deleted", result.TerminalVerificationTokensDeleted),
		zap.Int("terminal_password_reset_tokens_deleted", result.TerminalPasswordResetTokensDeleted),
		zap.Int("terminal_recovery_contacts_deleted", result.TerminalRecoveryContactsDeleted),
		zap.Bool("receipt_cleanup_bound_hit", boundHit),
		zap.Duration("receipt_cleanup_duration", duration),
	}
}

func (janitor *ReceiptJanitor) recordCleanup(
	attemptAt time.Time,
	duration time.Duration,
	result ports.ReceiptCleanupResult,
	batches int,
	boundHit bool,
	failureClassification string,
) {
	janitor.statsMu.Lock()
	defer janitor.statsMu.Unlock()
	janitor.stats.LastAttemptAt = attemptAt
	janitor.stats.LastDuration = duration
	janitor.stats.LastSessionIssuanceErased = result.SessionIssuanceErased
	janitor.stats.LastRefreshRotationErased = result.RefreshRotationErased
	janitor.stats.LastRecoveryCommandReceiptsDeleted = result.RecoveryCommandReceiptsDeleted
	janitor.stats.LastSessionCommandReceiptsDeleted = result.SessionCommandReceiptsDeleted
	janitor.stats.LastVerificationTokensExpired = result.VerificationTokensExpired
	janitor.stats.LastPasswordResetTokensExpired = result.PasswordResetTokensExpired
	janitor.stats.LastSecurityEmailDeliveriesCancelled = result.SecurityEmailDeliveriesCancelled
	janitor.stats.LastPendingRecoveryContactsRevoked = result.PendingRecoveryContactsRevoked
	janitor.stats.LastRevokedRecoveryContactsErased = result.RevokedRecoveryContactsErased
	janitor.stats.LastTerminalSecurityEmailDeliveriesDeleted = result.TerminalSecurityEmailDeliveriesDeleted
	janitor.stats.LastTerminalVerificationTokensDeleted = result.TerminalVerificationTokensDeleted
	janitor.stats.LastTerminalPasswordResetTokensDeleted = result.TerminalPasswordResetTokensDeleted
	janitor.stats.LastTerminalRecoveryContactsDeleted = result.TerminalRecoveryContactsDeleted
	janitor.stats.LastBatches = batches
	janitor.stats.LastBoundHit = boundHit
	janitor.stats.TotalSessionIssuanceErased += uint64(max(result.SessionIssuanceErased, 0))
	janitor.stats.TotalRefreshRotationErased += uint64(max(result.RefreshRotationErased, 0))
	janitor.stats.TotalRecoveryCommandReceiptsDeleted += uint64(max(result.RecoveryCommandReceiptsDeleted, 0))
	janitor.stats.TotalSessionCommandReceiptsDeleted += uint64(max(result.SessionCommandReceiptsDeleted, 0))
	janitor.stats.TotalVerificationTokensExpired += uint64(max(result.VerificationTokensExpired, 0))
	janitor.stats.TotalPasswordResetTokensExpired += uint64(max(result.PasswordResetTokensExpired, 0))
	janitor.stats.TotalSecurityEmailDeliveriesCancelled += uint64(max(result.SecurityEmailDeliveriesCancelled, 0))
	janitor.stats.TotalPendingRecoveryContactsRevoked += uint64(max(result.PendingRecoveryContactsRevoked, 0))
	janitor.stats.TotalRevokedRecoveryContactsErased += uint64(max(result.RevokedRecoveryContactsErased, 0))
	janitor.stats.TotalTerminalSecurityEmailDeliveriesDeleted += uint64(max(result.TerminalSecurityEmailDeliveriesDeleted, 0))
	janitor.stats.TotalTerminalVerificationTokensDeleted += uint64(max(result.TerminalVerificationTokensDeleted, 0))
	janitor.stats.TotalTerminalPasswordResetTokensDeleted += uint64(max(result.TerminalPasswordResetTokensDeleted, 0))
	janitor.stats.TotalTerminalRecoveryContactsDeleted += uint64(max(result.TerminalRecoveryContactsDeleted, 0))
	janitor.stats.LastFailureClassificationCode = failureClassification
	if failureClassification != "" {
		janitor.stats.ConsecutiveFailures++
		return
	}
	janitor.stats.LastSuccessAt = attemptAt
	janitor.stats.ConsecutiveFailures = 0
}

func (janitor *ReceiptJanitor) setRunning(running bool) {
	janitor.statsMu.Lock()
	janitor.stats.Running = running
	janitor.statsMu.Unlock()
}

func (janitor *ReceiptJanitor) requestStop() {
	janitor.stopOnce.Do(func() { close(janitor.stop) })
}

func (janitor *ReceiptJanitor) Stats() ReceiptJanitorStats {
	if janitor == nil {
		return ReceiptJanitorStats{}
	}
	janitor.statsMu.RLock()
	defer janitor.statsMu.RUnlock()
	return janitor.stats
}

// Check implements health.Checker. Register it as a non-critical readiness
// check: cleanup failure is visible operationally but must not evict otherwise
// healthy API instances and amplify an incident.
func (janitor *ReceiptJanitor) Check(ctx context.Context) error {
	if janitor == nil || ctx == nil {
		return ErrReceiptJanitorHealth
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	stats := janitor.Stats()
	if !stats.Running || stats.LastSuccessAt.IsZero() || stats.ConsecutiveFailures > 0 {
		return ErrReceiptJanitorHealth
	}
	now := janitor.config.Clock().UTC()
	staleAfter := 3*janitor.config.Interval + janitor.config.Timeout
	age := now.Sub(stats.LastSuccessAt)
	if age < 0 || age > staleAfter {
		return ErrReceiptJanitorHealth
	}
	return nil
}

func receiptCleanupErrorClass(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, ErrInvalidReceiptJanitor):
		return "configuration"
	default:
		return "storage"
	}
}

var _ health.Checker = (*ReceiptJanitor)(nil)

// Compile-time lifecycle shape without coupling this domain to a concrete
// runner implementation.
var _ interface {
	Name() string
	Run(context.Context) error
	Shutdown(context.Context) error
} = (*ReceiptJanitor)(nil)
