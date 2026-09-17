package usecase

import (
	"context"
	"errors"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/health"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	maximumSecurityEmailPollInterval  = 24 * time.Hour
	maximumSecurityEmailTimeout       = 5 * time.Minute
	maximumSecurityEmailConcurrency   = 32
	maximumSecurityEmailDispatchBatch = 100
	maximumSecurityEmailAttempts      = 100
	maximumSecurityEmailRetry         = 24 * time.Hour
	securityEmailLeaseSafetyMargin    = time.Second
)

var (
	ErrInvalidSecurityEmailDispatcher = errors.New("identity: invalid security email dispatcher")
	ErrSecurityEmailDispatcherHealth  = errors.New("identity: security email dispatcher unhealthy")
)

type securityEmailDispatcherState uint8

const (
	securityEmailDispatcherNew securityEmailDispatcherState = iota
	securityEmailDispatcherRunning
	securityEmailDispatcherStopping
	securityEmailDispatcherStopped
)

type SecurityEmailDispatcherConfig struct {
	Owner           string
	PollInterval    time.Duration
	ClaimTimeout    time.Duration
	DeliveryTimeout time.Duration
	FinalizeTimeout time.Duration
	LeaseDuration   time.Duration
	BatchSize       int
	Concurrency     int
	MaximumAttempts uint32
	RetryInitial    time.Duration
	RetryMaximum    time.Duration
	RetryMultiplier float64
	Clock           func() time.Time
}

type SecurityEmailDispatcherStats struct {
	Running             bool
	LastCycleAt         time.Time
	LastSuccessAt       time.Time
	LastDuration        time.Duration
	LastClaimed         int
	ConsecutiveFailures uint64
	LastFailureClass    string
	Claimed             uint64
	Sent                uint64
	Retried             uint64
	Dead                uint64
	Cancelled           uint64
}

type SecurityEmailDispatcher struct {
	repository ports.SecurityEmailDeliveryRepository
	delivery   ports.SecurityEmailDelivery
	codec      *SecurityEmailPayloadCodec
	tx         tx.Manager
	config     SecurityEmailDispatcherConfig

	lifecycleMu sync.Mutex
	state       securityEmailDispatcherState
	runCancel   context.CancelFunc
	stop        chan struct{}
	stopOnce    sync.Once
	done        chan struct{}
	doneOnce    sync.Once

	statsMu sync.RWMutex
	stats   SecurityEmailDispatcherStats
}

func NewSecurityEmailDispatcher(
	repository ports.SecurityEmailDeliveryRepository,
	delivery ports.SecurityEmailDelivery,
	codec *SecurityEmailPayloadCodec,
	txManager tx.Manager,
	config SecurityEmailDispatcherConfig,
) (*SecurityEmailDispatcher, error) {
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if isNilTokenDependency(repository) || isNilTokenDependency(delivery) || codec == nil ||
		isNilTokenDependency(txManager) || !validSecurityEmailDispatcherConfig(config) {
		return nil, ErrInvalidSecurityEmailDispatcher
	}
	return &SecurityEmailDispatcher{
		repository: repository, delivery: delivery, codec: codec, tx: txManager, config: config,
		state: securityEmailDispatcherNew,
		stop:  make(chan struct{}), done: make(chan struct{}),
	}, nil
}

func (*SecurityEmailDispatcher) Name() string { return "identity-security-email-dispatcher" }

func (dispatcher *SecurityEmailDispatcher) Run(ctx context.Context) error {
	if dispatcher == nil || ctx == nil {
		return ErrInvalidSecurityEmailDispatcher
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dispatcher.lifecycleMu.Lock()
	if dispatcher.state != securityEmailDispatcherNew {
		dispatcher.lifecycleMu.Unlock()
		return ErrInvalidSecurityEmailDispatcher
	}
	runCtx, cancel := context.WithCancel(ctx)
	dispatcher.runCancel = cancel
	dispatcher.state = securityEmailDispatcherRunning
	dispatcher.setRunning(true)
	dispatcher.lifecycleMu.Unlock()

	componentCtx := correlation.EnsureContext(runCtx)
	logger.FromContext(componentCtx).Info(
		"security email dispatcher started",
		zap.Duration("poll_interval", dispatcher.config.PollInterval),
		zap.Int("batch_size", dispatcher.config.BatchSize),
		zap.Int("concurrency", dispatcher.config.Concurrency),
	)
	defer func() {
		cancel()
		dispatcher.lifecycleMu.Lock()
		dispatcher.state = securityEmailDispatcherStopped
		dispatcher.setRunning(false)
		dispatcher.doneOnce.Do(func() { close(dispatcher.done) })
		dispatcher.lifecycleMu.Unlock()
		logger.FromContext(componentCtx).Info("security email dispatcher stopped")
	}()

	dispatcher.dispatchCycle(componentCtx)
	ticker := time.NewTicker(dispatcher.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-runCtx.Done():
			return nil
		case <-dispatcher.stop:
			return nil
		case <-ticker.C:
			dispatcher.dispatchCycle(componentCtx)
		}
	}
}

func (dispatcher *SecurityEmailDispatcher) Shutdown(ctx context.Context) error {
	if dispatcher == nil || ctx == nil {
		return ErrInvalidSecurityEmailDispatcher
	}
	dispatcher.lifecycleMu.Lock()
	switch dispatcher.state {
	case securityEmailDispatcherNew:
		dispatcher.state = securityEmailDispatcherStopped
		dispatcher.setRunning(false)
		dispatcher.requestStop()
		dispatcher.doneOnce.Do(func() { close(dispatcher.done) })
	case securityEmailDispatcherRunning:
		dispatcher.state = securityEmailDispatcherStopping
		if dispatcher.runCancel != nil {
			dispatcher.runCancel()
		}
		dispatcher.requestStop()
	case securityEmailDispatcherStopping, securityEmailDispatcherStopped:
	default:
		dispatcher.lifecycleMu.Unlock()
		return ErrInvalidSecurityEmailDispatcher
	}
	done := dispatcher.done
	dispatcher.lifecycleMu.Unlock()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (dispatcher *SecurityEmailDispatcher) dispatchCycle(parent context.Context) {
	if dispatcher == nil || parent == nil || parent.Err() != nil {
		return
	}
	cycleAt := dispatcher.config.Clock().UTC()
	started := time.Now()
	claimCtx, cancel := context.WithTimeout(
		correlation.WithContext(parent, correlation.New()),
		dispatcher.config.ClaimTimeout,
	)
	var leases []ports.SecurityEmailLease
	err := dispatcher.tx.DoInTx(claimCtx, func(transactionContext context.Context) error {
		var claimErr error
		leases, claimErr = dispatcher.repository.Claim(transactionContext, ports.SecurityEmailClaim{
			Owner: dispatcher.config.Owner, Now: cycleAt,
			LeaseDuration: dispatcher.config.LeaseDuration, Limit: dispatcher.config.BatchSize,
		})
		return claimErr
	})
	cancel()
	if err != nil {
		if parent.Err() != nil {
			return
		}
		dispatcher.recordCycle(cycleAt, time.Since(started), 0, "storage")
		logger.FromContext(parent).Warn(
			"security email claim failed",
			zap.String("error_class", "storage"),
		)
		return
	}
	dispatcher.addClaimed(len(leases))
	failures := dispatcher.dispatchBatch(parent, leases)
	duration := time.Since(started)
	if duration < 0 {
		duration = 0
	}
	if failures > 0 {
		dispatcher.recordCycle(cycleAt, duration, len(leases), "storage")
		return
	}
	dispatcher.recordCycle(cycleAt, duration, len(leases), "")
}

func (dispatcher *SecurityEmailDispatcher) dispatchBatch(
	parent context.Context,
	leases []ports.SecurityEmailLease,
) int {
	if len(leases) == 0 {
		return 0
	}
	workerCount := min(dispatcher.config.Concurrency, len(leases))
	jobs := make(chan ports.SecurityEmailLease)
	results := make(chan error, len(leases))
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for lease := range jobs {
				workCtx := correlation.WithContext(parent, correlation.New())
				results <- dispatcher.dispatchLease(workCtx, lease)
			}
		}()
	}
	for _, lease := range leases {
		jobs <- lease
	}
	close(jobs)
	workers.Wait()
	close(results)
	failures := 0
	for err := range results {
		if err == nil || errors.Is(err, ports.ErrSecurityEmailLeaseLost) ||
			parent.Err() != nil && errors.Is(err, context.Canceled) {
			continue
		}
		failures++
	}
	return failures
}

func (dispatcher *SecurityEmailDispatcher) dispatchLease(
	ctx context.Context,
	lease ports.SecurityEmailLease,
) (dispatchErr error) {
	ctx = correlation.EnsureContext(ctx)
	ctx = logger.WithFields(
		ctx,
		logger.String("delivery_id", lease.ID.String()),
		logger.String("security_email_kind", string(lease.Kind)),
		logger.String("attempt", strconv.FormatUint(uint64(lease.Attempt), 10)),
	)
	defer func() {
		if dispatchErr == nil || errors.Is(dispatchErr, ports.ErrSecurityEmailLeaseLost) ||
			ctx.Err() != nil && errors.Is(dispatchErr, context.Canceled) {
			return
		}
		logger.FromContext(ctx).Error(
			"security email dispatch failed",
			zap.String("error_class", securityEmailDispatchFailureClass(dispatchErr)),
		)
	}()

	now := dispatcher.config.Clock().UTC()
	if lease.Attempt > dispatcher.config.MaximumAttempts {
		return dispatcher.markDead(ctx, lease, now, "attempts_exhausted")
	}
	var credentialID uuid.UUID
	switch lease.Kind {
	case ports.SecurityEmailKindPasswordReset:
		if lease.PasswordResetTokenID == uuid.Nil || lease.VerificationTokenID != uuid.Nil {
			return dispatcher.markDead(ctx, lease, now, "payload_invalid")
		}
		credentialID = lease.PasswordResetTokenID
	case ports.SecurityEmailKindRecoveryContactVerification,
		ports.SecurityEmailKindAccountInvitation:
		if lease.VerificationTokenID == uuid.Nil || lease.PasswordResetTokenID != uuid.Nil {
			return dispatcher.markDead(ctx, lease, now, "payload_invalid")
		}
		credentialID = lease.VerificationTokenID
	default:
		return dispatcher.markDead(ctx, lease, now, "payload_unsupported")
	}
	prepared, err := dispatcher.prepareForSend(ctx, lease, credentialID, now)
	if err != nil {
		return err
	}
	if !prepared {
		dispatcher.incrementCancelled()
		logger.FromContext(ctx).Info("stale security email cancelled")
		return nil
	}
	metadata := SecurityEmailPayloadMetadata{
		DeliveryID: lease.ID, Kind: lease.Kind,
		ProviderIdempotencyKey: lease.ProviderIdempotencyKey,
		ExpiresAt:              lease.PayloadExpiresAt,
	}
	var deliver func(context.Context) error
	switch lease.Kind {
	case ports.SecurityEmailKindPasswordReset:
		metadata.CredentialID = lease.PasswordResetTokenID
		message, err := dispatcher.codec.OpenPasswordReset(
			metadata, lease.KeyID, lease.PayloadCiphertext, now,
		)
		if err != nil {
			return dispatcher.markDead(ctx, lease, now, securityEmailPayloadErrorClass(err))
		}
		deliver = func(deliveryContext context.Context) error {
			return dispatcher.delivery.Deliver(deliveryContext, message)
		}
	case ports.SecurityEmailKindRecoveryContactVerification:
		metadata.CredentialID = lease.VerificationTokenID
		message, err := dispatcher.codec.OpenRecoveryContactVerification(
			metadata, lease.KeyID, lease.PayloadCiphertext, now,
		)
		if err != nil {
			return dispatcher.markDead(ctx, lease, now, securityEmailPayloadErrorClass(err))
		}
		deliver = func(deliveryContext context.Context) error {
			return dispatcher.delivery.DeliverVerification(deliveryContext, message)
		}
	case ports.SecurityEmailKindAccountInvitation:
		metadata.CredentialID = lease.VerificationTokenID
		message, err := dispatcher.codec.OpenAccountInvitation(
			metadata, lease.KeyID, lease.PayloadCiphertext, now,
		)
		if err != nil {
			return dispatcher.markDead(ctx, lease, now, securityEmailPayloadErrorClass(err))
		}
		deliver = func(deliveryContext context.Context) error {
			return dispatcher.delivery.DeliverInvitation(deliveryContext, message)
		}
	}
	deliveryCtx, cancel := context.WithTimeout(ctx, dispatcher.config.DeliveryTimeout)
	deliveryStarted := time.Now()
	deliveryErr := deliver(deliveryCtx)
	deliveryDuration := time.Since(deliveryStarted)
	completedAt := dispatcher.config.Clock().UTC()
	if deliveryDuration < 0 {
		deliveryDuration = 0
	}
	cancel()
	if deliveryErr == nil {
		logger.FromContext(ctx).Debug(
			"security email provider attempt completed",
			zap.Duration("provider_latency", deliveryDuration),
			zap.String("result", "accepted"),
		)
		return dispatcher.markSent(ctx, lease, completedAt)
	}
	retryable, class := securityEmailProviderFailurePolicy(deliveryErr)
	logger.FromContext(ctx).Debug(
		"security email provider attempt completed",
		zap.Duration("provider_latency", deliveryDuration),
		zap.String("result", class),
	)
	if !retryable || lease.Attempt >= dispatcher.config.MaximumAttempts {
		if lease.Attempt >= dispatcher.config.MaximumAttempts && retryable {
			class = "attempts_exhausted"
		}
		return dispatcher.markDead(ctx, lease, completedAt, class)
	}
	delay := dispatcher.retryDelay(lease.Attempt)
	nextAttemptAt := completedAt.Add(delay)
	if !nextAttemptAt.Before(lease.PayloadExpiresAt) {
		return dispatcher.markDead(ctx, lease, completedAt, "payload_expired")
	}
	return dispatcher.markRetry(ctx, lease, completedAt, nextAttemptAt, class)
}

func (dispatcher *SecurityEmailDispatcher) prepareForSend(
	ctx context.Context,
	lease ports.SecurityEmailLease,
	credentialID uuid.UUID,
	now time.Time,
) (bool, error) {
	prepareContext, cancel := context.WithTimeout(ctx, dispatcher.config.FinalizeTimeout)
	defer cancel()
	prepared := false
	err := dispatcher.tx.DoInTx(prepareContext, func(transactionContext context.Context) error {
		var prepareErr error
		prepared, prepareErr = dispatcher.repository.PrepareForSend(
			transactionContext,
			ports.SecurityEmailPrepareCommand{
				ID: lease.ID, Kind: lease.Kind, CredentialID: credentialID,
				Owner: lease.Owner, LeaseToken: lease.LeaseToken,
				Attempt: lease.Attempt, Now: now,
			},
		)
		return prepareErr
	})
	return prepared, err
}

func (dispatcher *SecurityEmailDispatcher) markSent(
	ctx context.Context,
	lease ports.SecurityEmailLease,
	at time.Time,
) error {
	err := dispatcher.finalize(ctx, func(transactionContext context.Context) error {
		return dispatcher.repository.MarkSent(transactionContext, securityEmailOutcome(lease, at, ""))
	})
	if err == nil {
		dispatcher.incrementSent()
		logger.FromContext(ctx).Info("security email delivered")
	}
	return err
}

func (dispatcher *SecurityEmailDispatcher) markRetry(
	ctx context.Context,
	lease ports.SecurityEmailLease,
	at, nextAttemptAt time.Time,
	class string,
) error {
	outcome := securityEmailOutcome(lease, at, class)
	outcome.NextAttemptAt = nextAttemptAt
	err := dispatcher.finalize(ctx, func(transactionContext context.Context) error {
		return dispatcher.repository.MarkRetry(transactionContext, outcome)
	})
	if err == nil {
		dispatcher.incrementRetried()
		logger.FromContext(ctx).Info(
			"security email retry scheduled",
			zap.String("error_class", class),
			zap.Time("next_attempt_at", nextAttemptAt),
		)
	}
	return err
}

func (dispatcher *SecurityEmailDispatcher) markDead(
	ctx context.Context,
	lease ports.SecurityEmailLease,
	at time.Time,
	class string,
) error {
	err := dispatcher.finalize(ctx, func(transactionContext context.Context) error {
		return dispatcher.repository.MarkDead(transactionContext, securityEmailOutcome(lease, at, class))
	})
	if err == nil {
		dispatcher.incrementDead()
		logger.FromContext(ctx).Warn(
			"security email delivery terminal",
			zap.String("error_class", class),
		)
	}
	return err
}

func (dispatcher *SecurityEmailDispatcher) finalize(
	parent context.Context,
	operation func(context.Context) error,
) error {
	if parent == nil || operation == nil {
		return ErrInvalidSecurityEmailDispatcher
	}
	finalizeCtx, cancel := context.WithTimeout(
		context.WithoutCancel(parent),
		dispatcher.config.FinalizeTimeout,
	)
	defer cancel()
	return dispatcher.tx.DoInTx(finalizeCtx, operation)
}

func securityEmailOutcome(
	lease ports.SecurityEmailLease,
	at time.Time,
	class string,
) ports.SecurityEmailLeaseOutcome {
	return ports.SecurityEmailLeaseOutcome{
		ID: lease.ID, Owner: lease.Owner, LeaseToken: lease.LeaseToken,
		Attempt: lease.Attempt, At: at.UTC(), ErrorClass: class,
	}
}

func (dispatcher *SecurityEmailDispatcher) retryDelay(attempt uint32) time.Duration {
	exponent := max(int(attempt)-1, 0)
	delay := float64(dispatcher.config.RetryInitial)
	for index := 0; index < exponent && delay < float64(dispatcher.config.RetryMaximum); index++ {
		delay *= dispatcher.config.RetryMultiplier
		if math.IsInf(delay, 1) || delay >= float64(dispatcher.config.RetryMaximum) {
			return dispatcher.config.RetryMaximum
		}
	}
	if delay > float64(dispatcher.config.RetryMaximum) {
		return dispatcher.config.RetryMaximum
	}
	return time.Duration(delay)
}

func securityEmailPayloadErrorClass(err error) string {
	switch {
	case errors.Is(err, ErrSecurityEmailPayloadExpired):
		return "payload_expired"
	case errors.Is(err, ErrSecurityEmailPayloadAuthentication):
		return "payload_authentication"
	case errors.Is(err, ErrSecurityEmailPayloadUnsupported):
		return "payload_unsupported"
	default:
		return "payload_invalid"
	}
}

func securityEmailProviderFailurePolicy(err error) (bool, string) {
	var classified ports.PasswordResetDeliveryFailure
	if errors.As(err, &classified) {
		class := classified.ErrorClass()
		if !validSecurityEmailErrorClass(class) {
			class = "provider_failure"
		}
		return classified.Retryable(), class
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true, "provider_timeout"
	}
	return true, "provider_unavailable"
}

func securityEmailDispatchFailureClass(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "storage_timeout"
	case errors.Is(err, context.Canceled):
		return "storage_cancelled"
	default:
		return "storage"
	}
}

func validSecurityEmailErrorClass(value string) bool {
	return validProviderIdempotencyKey(value) && len(value) <= 64
}

func validSecurityEmailDispatcherConfig(config SecurityEmailDispatcherConfig) bool {
	return validProviderIdempotencyKey(config.Owner) &&
		config.PollInterval > 0 && config.PollInterval <= maximumSecurityEmailPollInterval &&
		config.ClaimTimeout > 0 && config.ClaimTimeout <= maximumSecurityEmailTimeout &&
		config.DeliveryTimeout > 0 && config.DeliveryTimeout <= maximumSecurityEmailTimeout &&
		config.FinalizeTimeout > 0 && config.FinalizeTimeout <= maximumSecurityEmailTimeout &&
		config.LeaseDuration >= config.ClaimTimeout+config.DeliveryTimeout+config.FinalizeTimeout+
			securityEmailLeaseSafetyMargin &&
		config.LeaseDuration <= maximumSecurityEmailTimeout &&
		config.BatchSize > 0 && config.BatchSize <= maximumSecurityEmailDispatchBatch &&
		config.Concurrency > 0 && config.Concurrency <= maximumSecurityEmailConcurrency &&
		config.BatchSize <= config.Concurrency &&
		config.MaximumAttempts > 0 && config.MaximumAttempts <= maximumSecurityEmailAttempts &&
		config.RetryInitial > 0 && config.RetryInitial <= config.RetryMaximum &&
		config.RetryMaximum <= maximumSecurityEmailRetry &&
		config.RetryMultiplier >= 1 && config.RetryMultiplier <= 10 && config.Clock != nil
}

func (dispatcher *SecurityEmailDispatcher) requestStop() {
	dispatcher.stopOnce.Do(func() { close(dispatcher.stop) })
}

func (dispatcher *SecurityEmailDispatcher) setRunning(running bool) {
	dispatcher.statsMu.Lock()
	dispatcher.stats.Running = running
	dispatcher.statsMu.Unlock()
}

func (dispatcher *SecurityEmailDispatcher) recordCycle(
	at time.Time,
	duration time.Duration,
	claimed int,
	failureClass string,
) {
	dispatcher.statsMu.Lock()
	defer dispatcher.statsMu.Unlock()
	dispatcher.stats.LastCycleAt = at
	dispatcher.stats.LastDuration = duration
	dispatcher.stats.LastClaimed = claimed
	dispatcher.stats.LastFailureClass = failureClass
	if failureClass != "" {
		dispatcher.stats.ConsecutiveFailures++
		return
	}
	dispatcher.stats.LastSuccessAt = at
	dispatcher.stats.ConsecutiveFailures = 0
}

func (dispatcher *SecurityEmailDispatcher) addClaimed(count int) {
	dispatcher.statsMu.Lock()
	dispatcher.stats.Claimed += uint64(max(count, 0))
	dispatcher.statsMu.Unlock()
}

func (dispatcher *SecurityEmailDispatcher) incrementSent() {
	dispatcher.statsMu.Lock()
	dispatcher.stats.Sent++
	dispatcher.statsMu.Unlock()
}

func (dispatcher *SecurityEmailDispatcher) incrementRetried() {
	dispatcher.statsMu.Lock()
	dispatcher.stats.Retried++
	dispatcher.statsMu.Unlock()
}

func (dispatcher *SecurityEmailDispatcher) incrementDead() {
	dispatcher.statsMu.Lock()
	dispatcher.stats.Dead++
	dispatcher.statsMu.Unlock()
}

func (dispatcher *SecurityEmailDispatcher) incrementCancelled() {
	dispatcher.statsMu.Lock()
	dispatcher.stats.Cancelled++
	dispatcher.statsMu.Unlock()
}

func (dispatcher *SecurityEmailDispatcher) Stats() SecurityEmailDispatcherStats {
	if dispatcher == nil {
		return SecurityEmailDispatcherStats{}
	}
	dispatcher.statsMu.RLock()
	defer dispatcher.statsMu.RUnlock()
	return dispatcher.stats
}

func (dispatcher *SecurityEmailDispatcher) Check(ctx context.Context) error {
	if dispatcher == nil || ctx == nil {
		return ErrSecurityEmailDispatcherHealth
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	stats := dispatcher.Stats()
	if !stats.Running || stats.LastSuccessAt.IsZero() || stats.ConsecutiveFailures > 0 {
		return ErrSecurityEmailDispatcherHealth
	}
	age := dispatcher.config.Clock().UTC().Sub(stats.LastSuccessAt)
	staleAfter := 3*dispatcher.config.PollInterval + dispatcher.config.ClaimTimeout +
		dispatcher.config.DeliveryTimeout + dispatcher.config.FinalizeTimeout
	if age < 0 || age > staleAfter {
		return ErrSecurityEmailDispatcherHealth
	}
	return nil
}

var _ health.Checker = (*SecurityEmailDispatcher)(nil)

var _ interface {
	Name() string
	Run(context.Context) error
	Shutdown(context.Context) error
} = (*SecurityEmailDispatcher)(nil)
