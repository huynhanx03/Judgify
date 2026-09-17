// Package eventdelivery composes Judgify's database-backed outbox relay with
// idempotent local projections. Transport-neutral relay/router primitives stay
// in go-common; this package owns only application lifecycle and repair.
package eventdelivery

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"
	"github.com/huynhanx03/go-common/pkg/common/lifecycle"
	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/platform/eventing"
)

const (
	maximumReconcileBatch   = 4096
	maximumReconcileBatches = 1024
	maximumReconcilerName   = 64
)

var (
	ErrInvalidRuntime     = errors.New("event delivery: invalid runtime")
	ErrRuntimeUnavailable = errors.New("event delivery: unavailable")
)

type relayLifecycle interface {
	Run(context.Context) error
	Shutdown(context.Context) error
}

type deadReplayer interface {
	ReplayDead(context.Context, eventing.ReplayDeadRequest) (bool, error)
}

// Reconciler repairs one idempotent projection from canonical PostgreSQL
// history. Reconcile returns the number processed in this bounded batch.
type Reconciler struct {
	Name      string
	Reconcile func(context.Context, int) (int, error)
}

// Options bounds periodic repair independently from live outbox delivery.
type Options struct {
	ReconcileInterval time.Duration
	ReconcileBatch    int
	MaxBatchesPerRun  int
}

type runtimeState uint8

const (
	runtimeNew runtimeState = iota
	runtimeStarting
	runtimeRunning
	runtimeStopped
)

// Runtime is the API-owned event-delivery lifecycle. Delivery and projection
// repair are deliberately colocated so a single-process deployment cannot
// accidentally start competing local-broker owners.
type Runtime struct {
	relay       relayLifecycle
	replayer    deadReplayer
	reconcilers []Reconciler
	options     Options
	logger      *zap.Logger

	mu                        sync.RWMutex
	state                     runtimeState
	observedSince             time.Time
	lastReconcileAt           time.Time
	lastSuccessfulReconcileAt time.Time
	lastReconcileErr          error
}

// MetricsSnapshot exposes only process-local reconciliation clocks. It carries
// no event, aggregate, tenant, user, or correlation identifiers.
type MetricsSnapshot struct {
	ObservedSince             time.Time
	LastReconcileAt           time.Time
	LastSuccessfulReconcileAt time.Time
}

func New(
	relay relayLifecycle,
	replayer deadReplayer,
	reconcilers []Reconciler,
	options Options,
	componentLogger *zap.Logger,
) (*Runtime, error) {
	if nilInterface(relay) || nilInterface(replayer) || componentLogger == nil ||
		options.ReconcileInterval <= 0 ||
		options.ReconcileBatch <= 0 || options.ReconcileBatch > maximumReconcileBatch ||
		options.MaxBatchesPerRun <= 0 ||
		options.MaxBatchesPerRun > maximumReconcileBatches ||
		!validReconcilers(reconcilers) {
		return nil, ErrInvalidRuntime
	}
	frozen := make([]Reconciler, len(reconcilers))
	copy(frozen, reconcilers)
	for index := range frozen {
		frozen[index].Name = strings.Clone(frozen[index].Name)
	}
	return &Runtime{
		relay: relay, replayer: replayer, reconcilers: frozen,
		options: options, logger: componentLogger, state: runtimeNew,
		observedSince: time.Now().UTC(),
	}, nil
}

func (*Runtime) Name() string { return "event-delivery" }

func (runtime *Runtime) Run(ctx context.Context) error {
	if runtime == nil || ctx == nil {
		return ErrInvalidRuntime
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	runtime.mu.Lock()
	if runtime.state != runtimeNew {
		runtime.mu.Unlock()
		return ErrInvalidRuntime
	}
	runtime.state = runtimeStarting
	runtime.mu.Unlock()
	defer func() {
		runtime.mu.Lock()
		runtime.state = runtimeStopped
		runtime.mu.Unlock()
	}()

	ctx = logger.WithContext(ctx, runtime.logger)
	if err := runtime.reconcile(ctx); err != nil && ctx.Err() == nil {
		logger.FromContext(ctx).Warn("event projection startup repair incomplete")
	}
	if ctx.Err() != nil {
		return nil
	}
	runtime.mu.Lock()
	runtime.state = runtimeRunning
	runtime.mu.Unlock()

	relayResult := make(chan error, 1)
	go func() { relayResult <- runtime.relay.Run(ctx) }()
	ticker := time.NewTicker(runtime.options.ReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-relayResult:
			if ctx.Err() != nil {
				return nil
			}
			if err == nil {
				return fmt.Errorf("%w: relay exited", ErrRuntimeUnavailable)
			}
			return fmt.Errorf("event delivery relay: %w", err)
		case <-ticker.C:
			if err := runtime.reconcile(ctx); err != nil && ctx.Err() == nil {
				logger.FromContext(ctx).Warn("event projection periodic repair incomplete")
			}
		}
	}
}

func (runtime *Runtime) Shutdown(ctx context.Context) error {
	if runtime == nil || ctx == nil || nilInterface(runtime.relay) {
		return ErrInvalidRuntime
	}
	return runtime.relay.Shutdown(ctx)
}

// ReplayDead exposes the fenced storage primitive without bypassing runtime
// observability. HTTP/CLI operator surfaces can depend on this narrow method.
func (runtime *Runtime) ReplayDead(
	ctx context.Context,
	eventID uuid.UUID,
	expectedAttempts uint32,
) (bool, error) {
	if runtime == nil || ctx == nil || eventID == uuid.Nil || expectedAttempts == 0 ||
		nilInterface(runtime.replayer) {
		return false, ErrInvalidRuntime
	}
	ctx = logger.WithContext(ctx, runtime.logger)
	replayed, err := runtime.replayer.ReplayDead(ctx, eventing.ReplayDeadRequest{
		EventID: eventID, ExpectedAttempts: expectedAttempts,
	})
	eventCtx := logger.WithFields(ctx,
		logger.String("event_id", eventID.String()),
	)
	if err != nil {
		logger.FromContext(eventCtx).Warn("dead outbox event replay failed")
		return false, err
	}
	logger.FromContext(eventCtx).Info(
		"dead outbox event replay evaluated",
		zap.Bool("replayed", replayed),
	)
	return replayed, nil
}

// Check reports lifecycle and latest repair status without exposing raw
// dependency errors through the health transport.
func (runtime *Runtime) Check(ctx context.Context) error {
	if runtime == nil || ctx == nil {
		return ErrRuntimeUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	runtime.mu.RLock()
	state, reconcileErr := runtime.state, runtime.lastReconcileErr
	runtime.mu.RUnlock()
	if state != runtimeRunning || reconcileErr != nil {
		return ErrRuntimeUnavailable
	}
	return nil
}

// Metrics returns a lock-consistent copy suitable for a scrape collector.
func (runtime *Runtime) Metrics() MetricsSnapshot {
	if runtime == nil {
		return MetricsSnapshot{}
	}
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return MetricsSnapshot{
		ObservedSince:             runtime.observedSince,
		LastReconcileAt:           runtime.lastReconcileAt,
		LastSuccessfulReconcileAt: runtime.lastSuccessfulReconcileAt,
	}
}

func (runtime *Runtime) reconcile(ctx context.Context) error {
	var failures []error
	for _, reconciler := range runtime.reconcilers {
		total := 0
		failed := false
		for batch := 0; batch < runtime.options.MaxBatchesPerRun; batch++ {
			processed, err := reconciler.Reconcile(ctx, runtime.options.ReconcileBatch)
			if err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", reconciler.Name, err))
				failed = true
				break
			}
			if processed < 0 || processed > runtime.options.ReconcileBatch {
				failures = append(failures, fmt.Errorf(
					"%s: %w",
					reconciler.Name,
					ErrInvalidRuntime,
				))
				failed = true
				break
			}
			total += processed
			if processed < runtime.options.ReconcileBatch {
				break
			}
		}
		reconcileCtx := logger.WithFields(ctx,
			logger.String("projection", reconciler.Name),
		)
		if failed {
			logger.FromContext(reconcileCtx).Warn("event projection repair failed")
			continue
		}
		if total == 0 {
			logger.FromContext(reconcileCtx).Debug("event projection already consistent")
		} else {
			logger.FromContext(reconcileCtx).Info(
				"event projection repaired",
				zap.Int("projection_count", total),
			)
		}
	}
	err := errors.Join(failures...)
	now := time.Now().UTC()
	runtime.mu.Lock()
	runtime.lastReconcileAt = now
	if err == nil {
		runtime.lastSuccessfulReconcileAt = now
	}
	runtime.lastReconcileErr = err
	runtime.mu.Unlock()
	return err
}

func validReconcilers(reconcilers []Reconciler) bool {
	if len(reconcilers) == 0 || len(reconcilers) > maximumReconcileBatches {
		return false
	}
	names := make(map[string]struct{}, len(reconcilers))
	for _, reconciler := range reconcilers {
		if !validName(reconciler.Name) || reconciler.Reconcile == nil {
			return false
		}
		if _, duplicate := names[reconciler.Name]; duplicate {
			return false
		}
		names[reconciler.Name] = struct{}{}
	}
	return true
}

func validName(value string) bool {
	if value == "" || len(value) > maximumReconcilerName || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return true
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var (
	_ lifecycle.Component  = (*Runtime)(nil)
	_ commonhealth.Checker = (*Runtime)(nil)
)
