// Package processhealth composes role-aware probes from go-common's bounded
// health primitives. Product roles own dependency selection; this package
// standardizes admission gates, safe metric observation, and private probe
// serving without importing a domain.
package processhealth

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"

	platformmetrics "github.com/huynhanx03/judgify/internal/platform/observability"
)

var ErrInvalidProcessHealth = errors.New("process health: invalid configuration")

type Role string

const (
	RoleAPI    Role = "api"
	RoleWorker Role = "judge-worker"
)

type Dependency struct {
	Name      string
	Checker   commonhealth.Checker
	Probes    commonhealth.Probe
	Critical  bool
	Timeout   time.Duration
	ErrorCode string
}

// Control is the lifecycle-owned process state shared by /live and /ready.
// Run opens the admission gate only after its event loop is live. Registered
// dependencies still keep the aggregate readiness probe fail-closed while
// their own components initialize. Drain closes admission synchronously.
type Control struct {
	role      Role
	liveness  commonhealth.Gate
	readiness commonhealth.Gate

	mu      sync.Mutex
	running bool
	done    chan struct{}
	once    sync.Once
}

func New(
	role Role,
	dependencies []Dependency,
	metrics *platformmetrics.Metrics,
) (*commonhealth.Registry, *Control, error) {
	if role != RoleAPI && role != RoleWorker {
		return nil, nil, ErrInvalidProcessHealth
	}
	registry, err := commonhealth.New(commonhealth.Options{})
	if err != nil {
		return nil, nil, err
	}
	control := &Control{role: role, done: make(chan struct{})}
	registrations := []Dependency{
		{
			Name: "process-event-loop", Checker: &control.liveness,
			Probes: commonhealth.ProbeLiveness, Critical: true,
			Timeout: time.Second, ErrorCode: "process_event_loop_unavailable",
		},
		{
			Name: "admission-gate", Checker: &control.readiness,
			Probes: commonhealth.ProbeReadiness, Critical: true,
			Timeout: time.Second, ErrorCode: "process_draining_or_initializing",
		},
	}
	registrations = append(registrations, dependencies...)
	for _, dependency := range registrations {
		if !validDependency(dependency) {
			return nil, nil, ErrInvalidProcessHealth
		}
		checker := dependency.Checker
		if metrics != nil {
			checker = observedChecker{
				next: checker, metrics: metrics, role: string(role),
				probe: probeName(dependency.Probes), check: dependency.Name,
				critical: dependency.Critical, timeout: dependency.Timeout,
			}
		}
		if err := registry.Register(dependency.Name, checker, commonhealth.CheckOptions{
			Probes: dependency.Probes, Critical: dependency.Critical,
			Timeout: dependency.Timeout, ErrorCode: dependency.ErrorCode,
		}); err != nil {
			return nil, nil, fmt.Errorf("register %s health: %w", dependency.Name, err)
		}
	}
	return registry, control, nil
}

func (control *Control) Name() string {
	if control == nil {
		return "process-health"
	}
	return string(control.role) + "-process-health"
}

func (control *Control) Run(ctx context.Context) error {
	if control == nil || ctx == nil {
		return ErrInvalidProcessHealth
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	control.mu.Lock()
	if control.running {
		control.mu.Unlock()
		return ErrInvalidProcessHealth
	}
	control.running = true
	control.liveness.Open()
	control.readiness.Open()
	control.mu.Unlock()

	<-ctx.Done()
	control.BeginDrain("lifecycle_cancelled")
	control.liveness.Close("lifecycle_cancelled")
	control.once.Do(func() { close(control.done) })
	return nil
}

// MarkReady opens admission only while the process event loop is live.
func (control *Control) MarkReady() error {
	if control == nil {
		return ErrInvalidProcessHealth
	}
	control.mu.Lock()
	defer control.mu.Unlock()
	if !control.running || !control.liveness.IsOpen() {
		return ErrInvalidProcessHealth
	}
	control.readiness.Open()
	return nil
}

// BeginDrain synchronously closes admission.
func (control *Control) BeginDrain(reason string) {
	if control == nil {
		return
	}
	control.readiness.Close(reason)
}

func (control *Control) Shutdown(ctx context.Context) error {
	if control == nil || ctx == nil {
		return ErrInvalidProcessHealth
	}
	control.BeginDrain("shutdown")
	control.liveness.Close("shutdown")
	control.once.Do(func() { close(control.done) })
	select {
	case <-control.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type observedChecker struct {
	next     commonhealth.Checker
	metrics  *platformmetrics.Metrics
	role     string
	probe    string
	check    string
	critical bool
	timeout  time.Duration
}

func (checker observedChecker) Check(ctx context.Context) error {
	if nilChecker(checker.next) || checker.metrics == nil || ctx == nil {
		return ErrInvalidProcessHealth
	}
	startedAt := time.Now()
	err := checker.next.Check(ctx)
	outcome := "healthy"
	if err != nil {
		if checker.critical {
			outcome = "unhealthy"
		} else {
			outcome = "degraded"
		}
		if errors.Is(err, context.DeadlineExceeded) ||
			(checker.timeout > 0 && time.Since(startedAt) >= checker.timeout) {
			outcome = "timeout"
		}
	}
	checker.metrics.RecordHealth(
		context.WithoutCancel(ctx),
		checker.role,
		checker.probe,
		checker.check,
		outcome,
	)
	return err
}

func validDependency(dependency Dependency) bool {
	if nilChecker(dependency.Checker) || dependency.Probes == 0 ||
		dependency.Timeout <= 0 || dependency.Timeout > 30*time.Second ||
		!safeToken(dependency.Name) || !safeToken(dependency.ErrorCode) {
		return false
	}
	return true
}

func nilChecker(checker commonhealth.Checker) bool {
	if checker == nil {
		return true
	}
	value := reflect.ValueOf(checker)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func safeToken(value string) bool {
	if len(value) == 0 || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func probeName(probe commonhealth.Probe) string {
	switch probe {
	case commonhealth.ProbeLiveness:
		return "live"
	case commonhealth.ProbeReadiness:
		return "ready"
	default:
		// A dependency registered on both is still a fixed bounded value.
		return "ready"
	}
}

var _ commonhealth.Checker = observedChecker{}
