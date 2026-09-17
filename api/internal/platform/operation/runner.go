package operation

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"
)

var ErrRunnerUnavailable = errors.New("operation runner: unavailable")

const (
	defaultLeaseDuration     = 45 * time.Second
	defaultBatchTimeout      = 30 * time.Second
	defaultIdleWait          = 250 * time.Millisecond
	defaultRetryBaseDelay    = 500 * time.Millisecond
	defaultRetryMaximumDelay = 30 * time.Second
	defaultMaximumAttempts   = 8
	minimumLeaseDuration     = 5 * time.Second
	maximumLeaseDuration     = 5 * time.Minute
	maximumRetryDelay        = 24 * time.Hour
	maximumAttempts          = 100
)

// BatchError is the only executor error classification persisted by the
// control plane. Error text and wrapped causes remain logs-only and are never
// copied into operation rows.
type BatchError struct {
	code       string
	retryable  bool
	retryAfter time.Duration
	cause      error
}

func (failure *BatchError) Error() string {
	if failure == nil {
		return "operation batch failed"
	}
	return "operation batch failed: " + failure.code
}

func (failure *BatchError) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.cause
}

func Retryable(code string, retryAfter time.Duration, cause error) error {
	if !validIdentifier(code, MaxFailureCodeBytes) ||
		retryAfter < 0 || retryAfter > maximumRetryDelay {
		return ErrInvalidOperation
	}
	return &BatchError{
		code: code, retryable: true, retryAfter: retryAfter, cause: cause,
	}
}

func Terminal(code string, cause error) error {
	if !validIdentifier(code, MaxFailureCodeBytes) {
		return ErrInvalidOperation
	}
	return &BatchError{code: code, cause: cause}
}

type RunnerOptions struct {
	Owner             string
	LeaseDuration     time.Duration
	BatchTimeout      time.Duration
	IdleWait          time.Duration
	RetryBaseDelay    time.Duration
	RetryMaximumDelay time.Duration
	MaximumAttempts   uint32
	Now               func() time.Time
}

type Runner struct {
	repository        Repository
	transactions      UnitOfWork
	registry          *Registry
	owner             string
	leaseDuration     time.Duration
	batchTimeout      time.Duration
	idleWait          time.Duration
	retryBaseDelay    time.Duration
	retryMaximumDelay time.Duration
	maximumAttempts   uint32
	now               func() time.Time

	mu      sync.RWMutex
	running bool
}

func NewRunner(
	repository Repository,
	transactions UnitOfWork,
	registry *Registry,
	options RunnerOptions,
) (*Runner, error) {
	if repository == nil || transactions == nil || registry == nil ||
		!validIdentifier(options.Owner, 128) {
		return nil, ErrInvalidOperation
	}
	if options.LeaseDuration == 0 {
		options.LeaseDuration = defaultLeaseDuration
	}
	if options.BatchTimeout == 0 {
		options.BatchTimeout = defaultBatchTimeout
	}
	if options.IdleWait == 0 {
		options.IdleWait = defaultIdleWait
	}
	if options.RetryBaseDelay == 0 {
		options.RetryBaseDelay = defaultRetryBaseDelay
	}
	if options.RetryMaximumDelay == 0 {
		options.RetryMaximumDelay = defaultRetryMaximumDelay
	}
	if options.MaximumAttempts == 0 {
		options.MaximumAttempts = defaultMaximumAttempts
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if options.LeaseDuration < minimumLeaseDuration ||
		options.LeaseDuration > maximumLeaseDuration ||
		options.BatchTimeout <= 0 ||
		options.BatchTimeout >= options.LeaseDuration ||
		options.IdleWait <= 0 || options.IdleWait > time.Minute ||
		options.RetryBaseDelay <= 0 ||
		options.RetryMaximumDelay < options.RetryBaseDelay ||
		options.RetryMaximumDelay > maximumRetryDelay ||
		options.MaximumAttempts < 1 ||
		options.MaximumAttempts > maximumAttempts {
		return nil, ErrInvalidOperation
	}
	if err := registry.Freeze(); err != nil {
		return nil, err
	}
	return &Runner{
		repository: repository, transactions: transactions, registry: registry,
		owner: options.Owner, leaseDuration: options.LeaseDuration,
		batchTimeout: options.BatchTimeout, idleWait: options.IdleWait,
		retryBaseDelay:    options.RetryBaseDelay,
		retryMaximumDelay: options.RetryMaximumDelay,
		maximumAttempts:   options.MaximumAttempts,
		now:               options.Now,
	}, nil
}

func (*Runner) Name() string { return "operation-runner" }

func (runner *Runner) Run(ctx context.Context) error {
	if runner == nil || ctx == nil {
		return ErrInvalidOperation
	}
	var consecutiveErrors uint32
	runner.mu.Lock()
	if runner.running {
		runner.mu.Unlock()
		return ErrRunnerUnavailable
	}
	runner.running = true
	runner.mu.Unlock()
	defer func() {
		runner.mu.Lock()
		runner.running = false
		runner.mu.Unlock()
	}()
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		claimed, err := runner.RunOnce(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			consecutiveErrors++
			logger.FromContext(ctx).Error(
				"operation runner iteration failed",
				zap.String("cause_type", fmt.Sprintf("%T", err)),
				zap.Uint32("consecutive_errors", consecutiveErrors),
			)
			if !waitOperationContext(
				ctx,
				runner.exponentialDelay(consecutiveErrors),
			) {
				return nil
			}
			continue
		}
		consecutiveErrors = 0
		if claimed {
			continue
		}
		if !waitOperationContext(ctx, runner.idleWait) {
			return nil
		}
	}
}

func (*Runner) Shutdown(context.Context) error { return nil }

// Kinds returns the runner's bounded code-owned operation catalog.
func (runner *Runner) Kinds() []string {
	if runner == nil || runner.registry == nil {
		return nil
	}
	return runner.registry.Kinds()
}

// Check reports lifecycle admission only. Database availability is already a
// separate critical probe, so this checker does not duplicate SQL traffic.
func (runner *Runner) Check(ctx context.Context) error {
	if runner == nil || ctx == nil {
		return ErrRunnerUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	runner.mu.RLock()
	running := runner.running
	runner.mu.RUnlock()
	if !running {
		return ErrRunnerUnavailable
	}
	return nil
}

// RunOnce claims and processes at most one bounded batch.
func (runner *Runner) RunOnce(ctx context.Context) (bool, error) {
	if runner == nil || runner.repository == nil || runner.transactions == nil ||
		runner.registry == nil || ctx == nil || ctx.Err() != nil {
		return false, ErrInvalidOperation
	}
	current, found, err := runner.repository.Claim(ctx, ClaimCommand{
		Owner: runner.owner, LeaseDuration: runner.leaseDuration,
	})
	if err != nil || !found {
		return found, err
	}
	if err := Validate(current); err != nil || current.Lease == nil {
		return true, errors.Join(ErrInvalidOperation, err)
	}
	operationContext := correlation.WithContext(ctx, current.CorrelationID)
	operationContext = logger.WithFields(
		operationContext,
		logger.String("operation_id", current.ID.String()),
		logger.String("operation_kind", current.Kind),
		logger.String("batch_sequence", strconv.FormatInt(current.NextBatchSequence, 10)),
		logger.String("attempt_count", strconv.FormatUint(uint64(current.AttemptCount), 10)),
	)
	lease := *current.Lease
	executor, exists := runner.registry.Lookup(current.Kind)
	if !exists {
		if failErr := runner.repository.MarkFailed(
			operationContext,
			lease,
			"executor_not_registered",
		); failErr != nil {
			return true, errors.Join(ErrExecutorNotFound, failErr)
		}
		logger.FromContext(operationContext).Error(
			"operation failed because executor is not registered",
		)
		return true, nil
	}

	startedAt := runner.now().UTC()
	logger.FromContext(operationContext).Debug(
		"operation batch claimed",
		zap.Time("lease_until", lease.Until),
	)
	batchContext, cancel := context.WithTimeout(
		operationContext,
		runner.batchTimeout,
	)
	defer cancel()
	var committed Operation
	err = runner.transactions.Within(batchContext, func(txCtx context.Context) error {
		locked, lockErr := runner.repository.LockClaim(txCtx, lease)
		if lockErr != nil {
			return lockErr
		}
		if locked.Kind != current.Kind || locked.NextBatchSequence != current.NextBatchSequence ||
			locked.BatchSize != current.BatchSize {
			return ErrLeaseLost
		}
		settled, controlled, controlErr := runner.repository.SettleRequestedControl(
			txCtx,
			lease,
		)
		if controlErr != nil {
			return controlErr
		}
		if controlled {
			committed = settled
			return nil
		}
		result, executeErr := executeOperationBatch(executor, txCtx, BatchInput{
			OperationID: locked.ID,
			Sequence:    locked.NextBatchSequence,
			Request:     append([]byte(nil), locked.Request...),
			Checkpoint:  append([]byte(nil), locked.Checkpoint...),
			Limit:       locked.BatchSize,
		})
		if executeErr != nil {
			return executeErr
		}
		if validateErr := ValidateBatchResult(locked, result); validateErr != nil {
			return validateErr
		}
		committed, executeErr = runner.repository.CommitBatch(
			txCtx, lease, result, startedAt,
		)
		return executeErr
	})
	if err == nil {
		if committed.Status == StatusPaused || committed.Status == StatusCancelled {
			logger.FromContext(operationContext).Info(
				"operation requested control settled before batch execution",
				zap.String("status", string(committed.Status)),
			)
			return true, nil
		}
		logger.FromContext(operationContext).Debug(
			"operation batch committed",
			zap.Int64("processed_count", committed.Processed),
			zap.Int64("total_count", committed.Total),
			zap.String("status", string(committed.Status)),
		)
		if committed.Status == StatusCompleted {
			logger.FromContext(operationContext).Info(
				"operation completed",
				zap.Int64("succeeded_count", committed.Succeeded),
				zap.Int64("failed_count", committed.Failed),
			)
		}
		return true, nil
	}
	if ctx.Err() != nil {
		// The durable lease is deliberately left for expiry/reclaim. A shutdown
		// must never issue a new write through an already-cancelled context.
		return true, nil
	}
	return true, runner.handleBatchFailure(
		operationContext,
		current,
		lease,
		err,
	)
}

func (runner *Runner) handleBatchFailure(
	ctx context.Context,
	current Operation,
	lease Lease,
	cause error,
) error {
	var classified *BatchError
	if errors.As(cause, &classified) && classified != nil &&
		validIdentifier(classified.code, MaxFailureCodeBytes) {
		if classified.retryable {
			return runner.scheduleRetry(
				ctx,
				current,
				lease,
				classified.code,
				classified.retryAfter,
				cause,
			)
		}
		if err := runner.repository.MarkFailed(ctx, lease, classified.code); err != nil {
			return errors.Join(cause, err)
		}
		logger.FromContext(ctx).Error(
			"operation reached terminal failure",
			zap.String("error_code", classified.code),
		)
		runner.logFailureDetail(ctx, cause)
		return nil
	}

	if errors.Is(cause, context.DeadlineExceeded) {
		return runner.scheduleRetry(
			ctx,
			current,
			lease,
			"batch_timeout",
			0,
			cause,
		)
	}
	if errors.Is(cause, ErrInvalidBatchResult) ||
		errors.Is(cause, ErrInvalidRequest) ||
		errors.Is(cause, ErrInvalidExecutor) {
		if err := runner.repository.MarkFailed(
			ctx,
			lease,
			"invalid_executor_result",
		); err != nil {
			return errors.Join(cause, err)
		}
		logger.FromContext(ctx).Error(
			"operation executor violated its contract",
			zap.String("error_code", "invalid_executor_result"),
			zap.String("cause_type", fmt.Sprintf("%T", cause)),
		)
		runner.logFailureDetail(ctx, cause)
		return nil
	}
	return runner.scheduleRetry(
		ctx,
		current,
		lease,
		"batch_execution_failed",
		0,
		cause,
	)
}

func (runner *Runner) scheduleRetry(
	ctx context.Context,
	current Operation,
	lease Lease,
	code string,
	requestedDelay time.Duration,
	cause error,
) error {
	if current.AttemptCount >= runner.maximumAttempts {
		if err := runner.repository.MarkFailed(ctx, lease, code); err != nil {
			return errors.Join(cause, err)
		}
		logger.FromContext(ctx).Error(
			"operation retry budget exhausted",
			zap.String("error_code", code),
		)
		runner.logFailureDetail(ctx, cause)
		return nil
	}
	delay := requestedDelay
	if delay <= 0 {
		delay = runner.exponentialDelay(current.AttemptCount)
	}
	if delay > runner.retryMaximumDelay {
		delay = runner.retryMaximumDelay
	}
	availableAt := runner.now().UTC().Add(delay)
	if err := runner.repository.ReleaseRetry(ctx, lease, RetryDirective{
		Code: code, AvailableAt: availableAt,
	}); err != nil {
		return errors.Join(cause, err)
	}
	logger.FromContext(ctx).Warn(
		"operation batch scheduled for retry",
		zap.String("error_code", code),
		zap.Time("available_at", availableAt),
	)
	runner.logFailureDetail(ctx, cause)
	return nil
}

func (runner *Runner) exponentialDelay(attempt uint32) time.Duration {
	if runner == nil || runner.retryBaseDelay <= 0 ||
		runner.retryMaximumDelay < runner.retryBaseDelay {
		return defaultRetryBaseDelay
	}
	delay := runner.retryBaseDelay
	for step := uint32(1); step < attempt; step++ {
		if delay >= runner.retryMaximumDelay/2 {
			return runner.retryMaximumDelay
		}
		delay *= 2
	}
	if delay > runner.retryMaximumDelay {
		return runner.retryMaximumDelay
	}
	return delay
}

func (runner *Runner) logFailureDetail(ctx context.Context, cause error) {
	logger.FromContext(ctx).Debug(
		"operation batch failure detail",
		zap.String("cause_type", fmt.Sprintf("%T", cause)),
	)
	var panicFailure *operationExecutorPanic
	if errors.As(cause, &panicFailure) && panicFailure != nil {
		logger.FromContext(ctx).Debug(
			"operation executor panic stack",
			zap.ByteString("stack", panicFailure.stack),
		)
	}
}

type operationExecutorPanic struct {
	stack []byte
}

func (*operationExecutorPanic) Error() string {
	return "operation executor panicked"
}

func executeOperationBatch(
	executor Executor,
	ctx context.Context,
	input BatchInput,
) (result BatchResult, err error) {
	defer func() {
		if recover() != nil {
			result = BatchResult{}
			err = Terminal(
				"executor_panic",
				&operationExecutorPanic{stack: append([]byte(nil), debug.Stack()...)},
			)
		}
	}()
	return executor.ExecuteBatch(ctx, input)
}

func waitOperationContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
