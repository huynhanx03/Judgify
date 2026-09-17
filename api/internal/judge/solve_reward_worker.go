package judge

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/encoding/json"
	"github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/go-common/pkg/mq/forge"
	outboxforge "github.com/huynhanx03/go-common/pkg/mq/outbox/forge"

	"github.com/huynhanx03/judgify/internal/ports"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

const (
	defaultSolveRewardLaneCount    = 8
	defaultSolveRewardLaneCapacity = 64
	defaultSolveRewardBatchSize    = 32
	defaultSolveRewardErrorBackoff = 100 * time.Millisecond
	maximumSolveRewardEventBytes   = 256 << 10
)

var ErrSolveRewardReconciliationIncomplete = errors.New(
	"solve reward worker: reconciliation budget exhausted",
)

type SolveRewardWorkerOptions struct {
	ReconcileInterval   time.Duration
	ReconcileBatch      int
	MaxReconcileBatches int
}

type solveRewardTask struct {
	event submissioncontract.ProblemSolveStatusChangedV1
	done  chan error
}

// SolveRewardWorker is the Forge delivery adapter for Cultivation progression.
// Each solve ID hashes to one FIFO lane, preserving aggregate order while
// independent solves continue concurrently.
type SolveRewardWorker struct {
	consumer  *forge.Consumer
	projector ports.SolveRewardProjector
	lanes     []chan solveRewardTask
	logger    *zap.Logger
	options   SolveRewardWorkerOptions

	mu       sync.Mutex
	started  bool
	live     bool
	stopped  bool
	runCtx   context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	doneOnce sync.Once
	workers  sync.WaitGroup
	done     chan struct{}
}

func NewSolveRewardWorker(
	consumer *forge.Consumer,
	projector ports.SolveRewardProjector,
	logger *zap.Logger,
	options SolveRewardWorkerOptions,
) (*SolveRewardWorker, error) {
	if consumer == nil || isNilSolveRewardProjector(projector) || logger == nil ||
		options.ReconcileInterval <= 0 ||
		options.ReconcileInterval > 24*time.Hour ||
		options.ReconcileBatch <= 0 || options.ReconcileBatch > 4096 ||
		options.MaxReconcileBatches <= 0 ||
		options.MaxReconcileBatches > 1024 {
		return nil, errors.New("solve reward worker: invalid dependencies")
	}
	lanes := make([]chan solveRewardTask, defaultSolveRewardLaneCount)
	for index := range lanes {
		lanes[index] = make(chan solveRewardTask, defaultSolveRewardLaneCapacity)
	}
	return &SolveRewardWorker{
		consumer: consumer, projector: projector, lanes: lanes,
		logger: logger, options: options, done: make(chan struct{}),
	}, nil
}

func (worker *SolveRewardWorker) Start(ctx context.Context) error {
	if worker == nil || ctx == nil {
		return errors.New("solve reward worker: invalid start")
	}
	worker.mu.Lock()
	if worker.started || worker.stopped {
		worker.mu.Unlock()
		return errors.New("solve reward worker: already started")
	}
	runCtx, cancel := context.WithCancel(logger.WithContext(ctx, worker.logger))
	worker.started, worker.runCtx, worker.cancel = true, runCtx, cancel
	worker.mu.Unlock()

	processed, err := reconcileSolveRewards(
		runCtx,
		worker.projector,
		worker.options,
	)
	if err != nil {
		cancel()
		worker.mu.Lock()
		worker.stopped = true
		worker.mu.Unlock()
		worker.finish()
		return fmt.Errorf("startup solve reward reconciliation: %w", err)
	}
	logger.FromContext(runCtx).Info(
		"startup solve reward reconciliation converged",
		zap.Int("transition_count", processed),
	)
	worker.mu.Lock()
	if worker.stopped || runCtx.Err() != nil {
		worker.mu.Unlock()
		cancel()
		worker.finish()
		return errors.Join(
			errors.New("solve reward worker: stopped during startup"),
			runCtx.Err(),
		)
	}
	worker.live = true
	worker.mu.Unlock()

	for _, lane := range worker.lanes {
		worker.workers.Add(1)
		go worker.runLane(runCtx, lane)
	}
	worker.workers.Add(2)
	go worker.runDeliveryLoop(runCtx)
	go worker.runReconciliationLoop(runCtx)
	go func() {
		worker.workers.Wait()
		worker.finish()
	}()
	return nil
}

func (worker *SolveRewardWorker) runDeliveryLoop(ctx context.Context) {
	defer worker.workers.Done()
	for {
		if ctx.Err() != nil {
			return
		}
		consumed, err := consumeForgeBatch(
			ctx, worker.consumer, defaultSolveRewardBatchSize,
			func(data []byte, target any) error {
				return json.UnmarshalStrictLimit(data, target, maximumSolveRewardEventBytes)
			},
			worker.dispatch,
			func(delivery forge.Delivery, err error) {
				logCtx := solveRewardDeliveryContext(ctx, worker.logger, delivery)
				logger.FromContext(logCtx).Error(
					"invalid solve reward payload",
					zap.Error(err),
					zap.Uint64("forge_offset", delivery.Offset),
					zap.Int("payload_bytes", len(delivery.Value)),
				)
			},
		)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.FromContext(ctx).Warn("solve reward poll failed", zap.Error(err))
			if !waitSolveRewardRetry(ctx, defaultSolveRewardErrorBackoff) {
				return
			}
			continue
		}
		if consumed == 0 {
			continue
		}
	}
}

func solveRewardDeliveryContext(
	ctx context.Context,
	baseLogger *zap.Logger,
	delivery forge.Delivery,
) context.Context {
	ctx = logger.WithContext(ctx, baseLogger)
	for _, header := range delivery.Headers {
		if string(header.Key) == outboxforge.HeaderCorrelationID {
			ctx = correlation.WithContext(ctx, string(header.Value))
			break
		}
	}
	return correlation.EnsureContext(ctx)
}

func waitSolveRewardRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (worker *SolveRewardWorker) runLane(ctx context.Context, lane <-chan solveRewardTask) {
	defer worker.workers.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-lane:
			result := worker.apply(task.event)
			task.done <- result
			close(task.done)
		}
	}
}

func (worker *SolveRewardWorker) runReconciliationLoop(ctx context.Context) {
	defer worker.workers.Done()
	ticker := time.NewTicker(worker.options.ReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			worker.reconcile(ctx)
		}
	}
}

func (worker *SolveRewardWorker) reconcile(ctx context.Context) {
	total, err := reconcileSolveRewards(ctx, worker.projector, worker.options)
	if err != nil {
		fields := []zap.Field{zap.Error(err), zap.Int("processed_count", total)}
		if ports.IsPermanentSolveReward(err) {
			logger.FromContext(ctx).Error("solve reward reconciliation rejected corrupt evidence", fields...)
		} else if ctx.Err() == nil {
			logger.FromContext(ctx).Warn("solve reward reconciliation failed", fields...)
		}
		return
	}
	if total > 0 {
		logger.FromContext(ctx).Info("solve reward reconciliation applied", zap.Int("transition_count", total))
	} else {
		logger.FromContext(ctx).Debug("solve reward reconciliation found no applicable transitions")
	}
}

func reconcileSolveRewards(
	ctx context.Context,
	projector ports.SolveRewardProjector,
	options SolveRewardWorkerOptions,
) (int, error) {
	if ctx == nil || isNilSolveRewardProjector(projector) ||
		options.ReconcileBatch <= 0 ||
		options.MaxReconcileBatches <= 0 {
		return 0, errors.New("solve reward worker: invalid reconciliation")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	total := 0
	for batch := 0; batch < options.MaxReconcileBatches; batch++ {
		processed, err := projector.ReconcileSolveRewards(
			ctx,
			options.ReconcileBatch,
		)
		if err != nil {
			return total, err
		}
		if processed < 0 || processed > options.ReconcileBatch {
			return total, errors.New(
				"solve reward worker: projector violated reconciliation contract",
			)
		}
		total += processed
		if processed < options.ReconcileBatch {
			return total, nil
		}
	}
	return total, ErrSolveRewardReconciliationIncomplete
}

func isNilSolveRewardProjector(projector ports.SolveRewardProjector) bool {
	if projector == nil {
		return true
	}
	value := reflect.ValueOf(projector)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (worker *SolveRewardWorker) dispatch(
	ctx context.Context,
	event submissioncontract.ProblemSolveStatusChangedV1,
) (<-chan error, error) {
	done := make(chan error, 1)
	lane := worker.lanes[solveRewardLaneIndex(event, len(worker.lanes))]
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case lane <- solveRewardTask{event: event, done: done}:
		return done, nil
	}
}

func solveRewardLaneIndex(
	event submissioncontract.ProblemSolveStatusChangedV1,
	laneCount int,
) int {
	if laneCount <= 1 {
		return 0
	}
	return int(binary.BigEndian.Uint64(event.SolveID[:8]) % uint64(laneCount))
}

func (worker *SolveRewardWorker) apply(
	event submissioncontract.ProblemSolveStatusChangedV1,
) error {
	worker.mu.Lock()
	base := worker.runCtx
	worker.mu.Unlock()
	if base == nil {
		return errors.New("solve reward worker: missing run context")
	}
	ctx := correlation.EnsureContext(correlation.WithContext(base, event.CorrelationID))
	ctx = logger.WithFields(
		ctx,
		logger.String("event_id", event.EventID.String()),
		logger.String("solve_id", event.SolveID.String()),
		logger.String("transition_id", event.TransitionID.String()),
	)
	result, err := worker.projector.ApplySolveTransition(ctx, event)
	if err != nil {
		fields := []zap.Field{
			zap.Error(err),
			zap.Int64("aggregate_version", event.To.Version),
			zap.Bool("permanent", ports.IsPermanentSolveReward(err)),
		}
		logger.FromContext(ctx).Error("solve reward projection failed", fields...)
		return err
	}
	fields := []zap.Field{
		zap.String("outcome", string(result.Outcome)),
		zap.Bool("transition_applied", result.TransitionApplied),
		zap.Int64("aggregate_version", event.To.Version),
		zap.Int64("experience_delta", result.TotalExperienceDelta),
		zap.Int("element_delta_count", result.ElementDeltaCount),
	}
	if result.Outcome == ports.SolveRewardGapDeferred {
		logger.FromContext(ctx).Warn("solve reward transition deferred for reconciliation", fields...)
	} else {
		logger.FromContext(ctx).Debug("solve reward transition processed", fields...)
	}
	return nil
}

func (worker *SolveRewardWorker) Shutdown(ctx context.Context) error {
	if worker == nil || ctx == nil {
		return errors.New("solve reward worker: invalid shutdown")
	}
	worker.stopOnce.Do(func() {
		worker.mu.Lock()
		worker.stopped = true
		cancel, live := worker.cancel, worker.live
		worker.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if !live {
			worker.finish()
		}
	})
	select {
	case <-worker.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (worker *SolveRewardWorker) Check(ctx context.Context) error {
	if worker == nil || ctx == nil {
		return errors.New("solve reward worker: unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	worker.mu.Lock()
	started, live, stopped, runCtx :=
		worker.started, worker.live, worker.stopped, worker.runCtx
	worker.mu.Unlock()
	if !started || !live || stopped || runCtx == nil || runCtx.Err() != nil {
		return errors.New("solve reward worker: unavailable")
	}
	return nil
}

func (worker *SolveRewardWorker) finish() {
	if worker == nil {
		return
	}
	worker.doneOnce.Do(func() { close(worker.done) })
}
