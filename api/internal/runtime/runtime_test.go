package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huynhanx03/go-common/pkg/common/lifecycle"
	"github.com/huynhanx03/go-common/pkg/correlation"
)

type recordingComponent struct {
	name     string
	started  chan<- string
	shutdown func(context.Context) error
}

type contextRecordingComponent struct {
	observed chan<- string
	cancel   context.CancelFunc
}

func (contextRecordingComponent) Name() string { return "context-recorder" }

func (component contextRecordingComponent) Run(ctx context.Context) error {
	component.observed <- correlation.FromContext(ctx)
	component.cancel()
	<-ctx.Done()
	return nil
}

func (contextRecordingComponent) Shutdown(context.Context) error { return nil }

func (component recordingComponent) Name() string { return component.name }

func (component recordingComponent) Run(ctx context.Context) error {
	if component.started != nil {
		component.started <- component.name
	}
	<-ctx.Done()
	return nil
}

func (component recordingComponent) Shutdown(ctx context.Context) error {
	if component.shutdown == nil {
		return nil
	}
	return component.shutdown(ctx)
}

func TestRunRejectsUnknownRoleAndReleaseAllBeforeBuilding(t *testing.T) {
	t.Parallel()

	buildCalls := 0
	config := Config{
		Mode: "release",
		Builders: Builders{
			All: func(context.Context) ([]lifecycle.Component, error) {
				buildCalls++
				return nil, nil
			},
		},
	}

	if err := Run(context.Background(), Role("unknown"), config); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("unknown role error = %v, want ErrInvalidRole", err)
	}
	if err := Run(context.Background(), RoleAll, config); !errors.Is(err, ErrAllDisabled) {
		t.Fatalf("release all error = %v, want ErrAllDisabled", err)
	}
	config.Mode = "prod"
	if err := Run(context.Background(), RoleAll, config); !errors.Is(err, ErrAllDisabled) {
		t.Fatalf("prod all error = %v, want ErrAllDisabled", err)
	}
	for _, mode := range []string{"", "staging", "typo"} {
		config.Mode = mode
		if err := Run(context.Background(), RoleAll, config); !errors.Is(err, ErrAllDisabled) {
			t.Fatalf("%q all error = %v, want ErrAllDisabled", mode, err)
		}
	}
	config.Mode = "dev"
	config.Builders.All = func(context.Context) ([]lifecycle.Component, error) {
		buildCalls++
		return nil, errors.New("development builder reached")
	}
	if err := Run(context.Background(), RoleAll, config); err == nil {
		t.Fatal("dev all did not invoke its builder")
	}
	if buildCalls != 1 {
		t.Fatalf("non-development all invoked builder; calls = %d", buildCalls)
	}
}

func TestRunMigrateIsFiniteAndDoesNotBuildLongRunningComponents(t *testing.T) {
	t.Parallel()

	migrated := false
	componentBuilds := 0
	config := Config{Builders: Builders{
		API: func(context.Context) ([]lifecycle.Component, error) {
			componentBuilds++
			return nil, nil
		},
		Migrate: func(context.Context) error {
			migrated = true
			return nil
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := Run(ctx, RoleMigrate, config); err != nil {
		t.Fatalf("Run migrate: %v", err)
	}
	if !migrated || componentBuilds != 0 {
		t.Fatalf("migrated=%t component builds=%d", migrated, componentBuilds)
	}
}

func TestRunUsesLifecycleForReverseBoundedShutdown(t *testing.T) {
	t.Parallel()

	started := make(chan string, 2)
	var mu sync.Mutex
	shutdownOrder := make([]string, 0, 2)
	appendShutdown := func(name string) func(context.Context) error {
		return func(context.Context) error {
			mu.Lock()
			shutdownOrder = append(shutdownOrder, name)
			mu.Unlock()
			return nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- Run(ctx, RoleAPI, Config{
			ShutdownTimeout:          time.Second,
			ComponentShutdownTimeout: 100 * time.Millisecond,
			Builders: Builders{API: func(context.Context) ([]lifecycle.Component, error) {
				return []lifecycle.Component{
					recordingComponent{name: "database", started: started, shutdown: appendShutdown("database")},
					recordingComponent{name: "http", started: started, shutdown: appendShutdown("http")},
				}, nil
			}},
		})
	}()

	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("components did not start")
		}
	}
	cancel()
	if err := <-result; err != nil {
		t.Fatalf("Run after cancellation: %v", err)
	}

	mu.Lock()
	got := strings.Join(shutdownOrder, ",")
	mu.Unlock()
	if got != "http,database" {
		t.Fatalf("shutdown order = %q, want http,database", got)
	}
}

func TestRunKeepsOneProcessCorrelationIDAcrossCompositionAndLifecycle(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	observed := make(chan string, 2)
	result := make(chan error, 1)
	go func() {
		result <- Run(ctx, RoleAPI, Config{
			ShutdownTimeout:          time.Second,
			ComponentShutdownTimeout: time.Second,
			Builders: Builders{API: func(buildContext context.Context) ([]lifecycle.Component, error) {
				observed <- correlation.FromContext(buildContext)
				return []lifecycle.Component{contextRecordingComponent{
					observed: observed,
					cancel:   cancel,
				}}, nil
			}},
		})
	}()

	compositionCID := <-observed
	componentCID := <-observed
	if err := correlation.Validate(compositionCID); err != nil {
		t.Fatalf("composition cid = %q: %v", compositionCID, err)
	}
	if componentCID != compositionCID {
		t.Fatalf("component cid = %q, composition cid = %q", componentCID, compositionCID)
	}
	if err := <-result; err != nil {
		t.Fatalf("Run after cancellation: %v", err)
	}
}

func TestRunReturnsBuilderAndComponentFailures(t *testing.T) {
	t.Parallel()

	buildFailure := errors.New("compose failed")
	err := Run(context.Background(), RoleAPI, Config{Builders: Builders{
		API: func(context.Context) ([]lifecycle.Component, error) { return nil, buildFailure },
	}})
	if !errors.Is(err, buildFailure) {
		t.Fatalf("builder error = %v, want %v", err, buildFailure)
	}

	runFailure := errors.New("listener failed")
	err = Run(context.Background(), RoleAPI, Config{Builders: Builders{
		API: func(context.Context) ([]lifecycle.Component, error) {
			return []lifecycle.Component{failingComponent{err: runFailure}}, nil
		},
	}})
	if !errors.Is(err, runFailure) {
		t.Fatalf("component error = %v, want %v", err, runFailure)
	}
}

func TestRunValidatesConfigurationBeforeBuilding(t *testing.T) {
	t.Parallel()

	buildCalls := 0
	builder := func(context.Context) ([]lifecycle.Component, error) {
		buildCalls++
		return []lifecycle.Component{recordingComponent{name: "api"}}, nil
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Run(canceled, RoleAPI, Config{Builders: Builders{API: builder}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context error = %v, want context.Canceled", err)
	}
	// Held in a variable rather than written inline so the nil-context guard
	// stays covered without tripping the linter's literal-nil check.
	var missingContext context.Context
	if err := Run(missingContext, RoleAPI, Config{
		Builders: Builders{API: builder},
	}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("nil context error = %v, want ErrInvalidConfig", err)
	}
	if err := Run(context.Background(), RoleAPI, Config{
		ShutdownTimeout: -time.Second,
		Builders:        Builders{API: builder},
	}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("negative timeout error = %v, want ErrInvalidConfig", err)
	}
	if err := Run(context.Background(), RoleAPI, Config{
		ComponentShutdownTimeout: -time.Second,
		Builders:                 Builders{API: builder},
	}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("negative component timeout error = %v, want ErrInvalidConfig", err)
	}
	if buildCalls != 0 {
		t.Fatalf("invalid configuration invoked builder %d times", buildCalls)
	}
}

func TestRunRejectsEmptyLongRunningRole(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), RoleAPI, Config{Builders: Builders{
		API: func(context.Context) ([]lifecycle.Component, error) { return nil, nil },
	}})
	if !errors.Is(err, ErrNoComponents) {
		t.Fatalf("empty role error = %v, want ErrNoComponents", err)
	}
}

func TestRunCleansRegisteredComponentsWhenRegistrationFails(t *testing.T) {
	t.Parallel()

	shutdown := make(chan struct{}, 1)
	err := Run(context.Background(), RoleAPI, Config{Builders: Builders{
		API: func(context.Context) ([]lifecycle.Component, error) {
			return []lifecycle.Component{
				recordingComponent{name: "duplicate", shutdown: func(context.Context) error {
					shutdown <- struct{}{}
					return nil
				}},
				recordingComponent{name: "duplicate"},
			}, nil
		},
	}})
	if !errors.Is(err, lifecycle.ErrDuplicateComponent) {
		t.Fatalf("registration error = %v, want duplicate component", err)
	}
	select {
	case <-shutdown:
	case <-time.After(time.Second):
		t.Fatal("registered component was not shut down")
	}
}

type failingComponent struct{ err error }

func (failingComponent) Name() string                        { return "failing" }
func (component failingComponent) Run(context.Context) error { return component.err }
func (failingComponent) Shutdown(context.Context) error      { return nil }
