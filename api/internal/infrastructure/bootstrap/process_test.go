package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/huynhanx03/go-common/pkg/common/lifecycle"
	pkgEnt "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/pkg/settings"
)

type inertComponent struct{}

func (inertComponent) Name() string                   { return "inert" }
func (inertComponent) Run(context.Context) error      { return nil }
func (inertComponent) Shutdown(context.Context) error { return nil }

func TestMigrateFailsClosedWithoutCheckedInSQLRunner(t *testing.T) {
	t.Parallel()

	process := &Process{}
	if err := process.Migrate(context.Background()); !errors.Is(err, ErrMigrationRunnerNotConfigured) {
		t.Fatalf("Migrate() = %v, want ErrMigrationRunnerNotConfigured", err)
	}
}

func TestMigrateInvokesFiniteInjectedRunner(t *testing.T) {
	t.Parallel()

	called := 0
	process := &Process{}
	if err := process.SetMigrationRunner(migrationRunnerFunc(func(context.Context, settings.Config) error {
		called++
		return nil
	})); err != nil {
		t.Fatalf("SetMigrationRunner: %v", err)
	}
	if err := process.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if called != 1 {
		t.Fatalf("migration runner calls = %d, want 1", called)
	}
}

func TestMigrationRunnerInstallationIsSingleAssignment(t *testing.T) {
	t.Parallel()

	process := &Process{}
	runner := migrationRunnerFunc(func(context.Context, settings.Config) error { return nil })
	if err := process.SetMigrationRunner(runner); err != nil {
		t.Fatalf("first SetMigrationRunner: %v", err)
	}
	if err := process.SetMigrationRunner(runner); !errors.Is(err, ErrMigrationRunnerAlreadyConfigured) {
		t.Fatalf("second SetMigrationRunner = %v, want ErrMigrationRunnerAlreadyConfigured", err)
	}
}

func TestBoundBuilderCopiesCompositionAndRejectsEmptyResult(t *testing.T) {
	t.Parallel()

	builders := []Builder{func(context.Context, *Process) ([]lifecycle.Component, error) {
		return []lifecycle.Component{inertComponent{}}, nil
	}}
	process := &Process{}
	bound := process.Builder(builders...)
	builders[0] = nil
	components, err := bound(context.Background())
	if err != nil || len(components) != 1 {
		t.Fatalf("bound builder = (%d, %v), want one component", len(components), err)
	}

	empty := &Process{}
	_, err = empty.Compose(context.Background(), func(context.Context, *Process) ([]lifecycle.Component, error) {
		return nil, nil
	})
	if !errors.Is(err, ErrInvalidProcess) {
		t.Fatalf("empty composition error = %v, want ErrInvalidProcess", err)
	}
}

func TestDatabaseObserversCanJoinBeforeOrAfterDatabaseAcquisition(t *testing.T) {
	t.Parallel()

	process := &Process{}
	called := make([]pkgEnt.StatementClass, 0, 2)
	observer := func(_ context.Context, observation pkgEnt.StatementObservation) {
		called = append(called, observation.Class)
	}
	if err := process.RegisterDatabaseObserver(observer); err != nil {
		t.Fatalf("RegisterDatabaseObserver: %v", err)
	}
	process.observeDatabase(context.Background(), pkgEnt.StatementObservation{
		Class:    pkgEnt.StatementClassRead,
		Duration: time.Millisecond,
	})
	if err := process.RegisterDatabaseObserver(observer); err != nil {
		t.Fatalf("second RegisterDatabaseObserver: %v", err)
	}
	process.observeDatabase(context.Background(), pkgEnt.StatementObservation{
		Class:    pkgEnt.StatementClassWrite,
		Duration: time.Millisecond,
	})

	want := []pkgEnt.StatementClass{
		pkgEnt.StatementClassRead,
		pkgEnt.StatementClassWrite,
		pkgEnt.StatementClassWrite,
	}
	if len(called) != len(want) {
		t.Fatalf("observer calls = %v, want %v", called, want)
	}
	for index := range want {
		if called[index] != want[index] {
			t.Fatalf("observer calls = %v, want %v", called, want)
		}
	}
}

type migrationRunnerFunc func(context.Context, settings.Config) error

func (run migrationRunnerFunc) Apply(ctx context.Context, config settings.Config) error {
	return run(ctx, config)
}
