// Package bootstrap owns finite process initialization and shared resources.
// It performs no role selection, signal handling, or background starts.
package bootstrap

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/huynhanx03/go-common/pkg/common/cache"
	"github.com/huynhanx03/go-common/pkg/common/cache/ristretto"
	"github.com/huynhanx03/go-common/pkg/common/lifecycle"
	pkgEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/authctx"
	internalEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/infrastructure/logging"
	"github.com/huynhanx03/judgify/pkg/settings"
)

var ErrInvalidProcess = errors.New("bootstrap: invalid process")
var ErrMigrationRunnerNotConfigured = errors.New("bootstrap: migration runner is not configured")
var ErrMigrationRunnerAlreadyConfigured = errors.New("bootstrap: migration runner is already configured")

const failedCompositionCleanupTimeout = 10 * time.Second
const maxDatabaseObservers = 8

// Builder composes role-specific components without starting them.
type Builder func(context.Context, *Process) ([]lifecycle.Component, error)

// MigrationRunner applies the checked-in migration chain and returns. Runtime
// bootstrap never falls back to Ent auto-DDL.
type MigrationRunner interface {
	Apply(context.Context, settings.Config) error
}

// Process is the single resource graph shared by every role in a process.
// Resource acquisition is idempotent so the development-only all role does
// not create duplicate database pools or caches.
type Process struct {
	Config settings.Config
	Logger *commonlogger.LoggerZap

	mu                sync.RWMutex
	database          *internalEnt.EntClient
	cache             cache.LocalCache[string, any]
	keys              bool
	closed            bool
	composed          bool
	migrations        MigrationRunner
	databaseObservers []pkgEnt.StatementObserver
}

// Builder binds one or more inert composition functions to this process. The
// returned closure is suitable for runtime.Builders and always funnels through
// the same resource ownership path.
func (process *Process) Builder(builders ...Builder) func(context.Context) ([]lifecycle.Component, error) {
	bound := append([]Builder(nil), builders...)
	return func(ctx context.Context) ([]lifecycle.Component, error) {
		if ctx != nil && process != nil && process.Logger != nil && process.Logger.Logger != nil {
			ctx = commonlogger.WithContext(ctx, process.Logger.Logger)
		}
		return process.Compose(ctx, bound...)
	}
}

// PrepareForRole validates the complete role-specific configuration before
// creating a logger, opening a database connection, or constructing any
// dependency. This keeps startup fail-closed and prevents a worker or migrator
// from requiring unrelated API secrets.
func PrepareForRole(profile Profile) (*Process, error) {
	config, err := loadConfig(context.Background(), profile, OSFileSecretSource{})
	if err != nil {
		return nil, err
	}
	initializedLogger, err := logging.New(config)
	if err != nil {
		return nil, err
	}

	process := &Process{Config: config, Logger: initializedLogger}
	return process, nil
}

// Compose acquires shared resources through one Process and prepends their
// lifecycle owners before role components. On any build failure it closes all
// resources acquired so far.
func (process *Process) Compose(ctx context.Context, builders ...Builder) ([]lifecycle.Component, error) {
	if process == nil || ctx == nil || len(builders) == 0 {
		return nil, ErrInvalidProcess
	}
	components := make([]lifecycle.Component, 0, len(builders)+4)
	for _, builder := range builders {
		if builder == nil {
			return nil, process.failBuild(ctx, ErrInvalidProcess, components)
		}
		built, err := builder(ctx, process)
		if err != nil {
			return nil, process.failBuild(ctx, err, components)
		}
		if len(built) == 0 {
			return nil, process.failBuild(ctx, ErrInvalidProcess, components)
		}
		components = append(components, built...)
	}
	resources, err := process.resourceComponents()
	if err != nil {
		return nil, process.failBuild(ctx, err, components)
	}
	return process.bindLifecycleContexts(append(resources, components...)), nil
}

func (process *Process) bindLifecycleContexts(
	components []lifecycle.Component,
) []lifecycle.Component {
	if process == nil || process.Logger == nil || process.Logger.Logger == nil {
		return components
	}
	bound := make([]lifecycle.Component, len(components))
	for index, component := range components {
		if component == nil {
			continue
		}
		bound[index] = contextualComponent{
			component: component,
			logger:    process.Logger,
		}
	}
	return bound
}

// OpenDatabase opens or returns the one process-local Ent client.
func (process *Process) OpenDatabase() (*internalEnt.EntClient, error) {
	if process == nil || process.Logger == nil || process.Logger.Logger == nil {
		return nil, ErrInvalidProcess
	}
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.closed {
		return nil, ErrInvalidProcess
	}
	if process.database != nil {
		return process.database, nil
	}

	baseDriver, err := pkgEnt.NewDriver(process.Config.Database)
	if err != nil {
		return nil, fmt.Errorf("open database driver: %w", err)
	}
	driver := pkgEnt.WrapObservedLogging(baseDriver, 0, process.observeDatabase)
	client := generate.NewClient(generate.Driver(driver))
	pkgEnt.RegisterErrorPredicates(pkgEnt.ErrorPredicates{
		IsNotFound:        generate.IsNotFound,
		IsValidationError: generate.IsValidationError,
		IsConstraintError: generate.IsConstraintError,
		IsNotLoaded:       generate.IsNotLoaded,
		IsNotSingular:     generate.IsNotSingular,
	})
	pkgEnt.SetActorResolver(func(ctx context.Context) (string, bool) {
		actorID, ok := authctx.UserID(ctx)
		if !ok {
			return "", false
		}
		return actorID.String(), true
	})
	process.database = internalEnt.WrapClientWithDriver(client, driver, process.Logger.Logger)
	process.Logger.Named("database").Info("database connected")
	return process.database, nil
}

// RegisterDatabaseObserver adds a process-local bounded metadata observer. It
// is valid both before and after OpenDatabase because the driver dispatches to
// the current observer snapshot on every completed call. Statement text and
// arguments never cross this boundary.
func (process *Process) RegisterDatabaseObserver(observer pkgEnt.StatementObserver) error {
	if process == nil || observer == nil {
		return ErrInvalidProcess
	}
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.closed || len(process.databaseObservers) >= maxDatabaseObservers {
		return ErrInvalidProcess
	}
	process.databaseObservers = append(process.databaseObservers, observer)
	return nil
}

func (process *Process) observeDatabase(
	ctx context.Context,
	observation pkgEnt.StatementObservation,
) {
	if process == nil {
		return
	}
	process.mu.RLock()
	observers := append([]pkgEnt.StatementObserver(nil), process.databaseObservers...)
	process.mu.RUnlock()
	for _, observer := range observers {
		callDatabaseObserver(observer, ctx, observation)
	}
}

func callDatabaseObserver(
	observer pkgEnt.StatementObserver,
	ctx context.Context,
	observation pkgEnt.StatementObservation,
) {
	defer func() { _ = recover() }()
	observer(ctx, observation)
}

// OpenCache opens or returns the one process-local cache.
func (process *Process) OpenCache() (cache.LocalCache[string, any], error) {
	if process == nil {
		return nil, ErrInvalidProcess
	}
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.closed {
		return nil, ErrInvalidProcess
	}
	if process.cache != nil {
		return process.cache, nil
	}
	local, err := ristretto.New[string, any](
		ristretto.WithMaxCost(10_000),
		ristretto.WithNumCounters(100_000),
	)
	if err != nil {
		return nil, fmt.Errorf("open local cache: %w", err)
	}
	process.cache = local
	return process.cache, nil
}

// LoadKeys reads and validates JWT keys once for API roles.
func (process *Process) LoadKeys() error {
	if process == nil {
		return ErrInvalidProcess
	}
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.closed {
		return ErrInvalidProcess
	}
	if process.keys {
		return nil
	}
	privateBytes, err := os.ReadFile(process.Config.JWT.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("read JWT private key: %w", err)
	}
	privateKey, err := parseRSAPrivateKey(privateBytes)
	if err != nil {
		return fmt.Errorf("parse JWT private key: %w", err)
	}
	publicBytes, err := os.ReadFile(process.Config.JWT.PublicKeyPath)
	if err != nil {
		return fmt.Errorf("read JWT public key: %w", err)
	}
	publicKey, err := parseRSAPublicKey(publicBytes)
	if err != nil {
		return fmt.Errorf("parse JWT public key: %w", err)
	}
	process.Config.JWT.PrivateKey = privateKey
	process.Config.JWT.PublicKey = publicKey
	process.keys = true
	return nil
}

// SetMigrationRunner installs the checked-in SQL migration adapter before the
// migrate role is selected. It does not execute the adapter.
func (process *Process) SetMigrationRunner(runner MigrationRunner) error {
	if process == nil || runner == nil {
		return ErrInvalidProcess
	}
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.closed || process.composed {
		return ErrInvalidProcess
	}
	if process.migrations != nil {
		return ErrMigrationRunnerAlreadyConfigured
	}
	process.migrations = runner
	return nil
}

// Migrate invokes only the configured checked-in SQL migration runner. It is
// finite and never constructs a long-running role or uses Ent auto-DDL.
func (process *Process) Migrate(ctx context.Context) error {
	if process == nil || ctx == nil {
		return ErrInvalidProcess
	}
	process.mu.Lock()
	runner := process.migrations
	config := process.Config
	closed := process.closed
	process.mu.Unlock()
	if closed {
		return ErrInvalidProcess
	}
	if runner == nil {
		return ErrMigrationRunnerNotConfigured
	}
	if err := runner.Apply(ctx, config); err != nil {
		return fmt.Errorf("apply checked-in migrations: %w", err)
	}
	return nil
}

// Close closes resources in reverse dependency order. It is idempotent.
func (process *Process) Close() error {
	if process == nil {
		return nil
	}
	process.mu.Lock()
	if process.closed {
		process.mu.Unlock()
		return nil
	}
	process.closed = true
	local := process.cache
	database := process.database
	process.cache = nil
	process.database = nil
	process.mu.Unlock()

	if local != nil {
		local.Close()
	}
	var err error
	if database != nil {
		err = database.Close()
	}
	return err
}

// Sync flushes the process logger after the lifecycle has stopped.
func (process *Process) Sync() error {
	if process == nil || process.Logger == nil {
		return nil
	}
	return process.Logger.Sync()
}

func (process *Process) resourceComponents() ([]lifecycle.Component, error) {
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.closed || process.composed {
		return nil, ErrInvalidProcess
	}
	process.composed = true
	components := make([]lifecycle.Component, 0, 2)
	if process.database != nil {
		components = append(components, &resourceComponent{
			name: "database",
			close: func() error {
				process.mu.Lock()
				database := process.database
				process.database = nil
				process.mu.Unlock()
				if database == nil {
					return nil
				}
				return database.Close()
			},
		})
	}
	if process.cache != nil {
		components = append(components, &resourceComponent{
			name: "local-cache",
			close: func() error {
				process.mu.Lock()
				local := process.cache
				process.cache = nil
				process.mu.Unlock()
				if local != nil {
					local.Close()
				}
				return nil
			},
		})
	}
	return components, nil
}

func (process *Process) failBuild(ctx context.Context, cause error, components []lifecycle.Component) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failedCompositionCleanupTimeout)
	defer cancel()
	failures := []error{cause}
	for index := len(components) - 1; index >= 0; index-- {
		component := components[index]
		if component == nil {
			continue
		}
		if err := component.Shutdown(cleanupCtx); err != nil {
			failures = append(failures, fmt.Errorf("cleanup %s: %w", component.Name(), err))
		}
	}
	failures = append(failures, process.Close())
	return errors.Join(failures...)
}

type resourceComponent struct {
	name      string
	close     func() error
	closeOnce sync.Once
	closeErr  error
}

type contextualComponent struct {
	component lifecycle.Component
	logger    *commonlogger.LoggerZap
}

func (component contextualComponent) Name() string {
	return component.component.Name()
}

func (component contextualComponent) Run(ctx context.Context) error {
	return component.component.Run(component.bind(ctx))
}

func (component contextualComponent) Shutdown(ctx context.Context) error {
	return component.component.Shutdown(component.bind(ctx))
}

func (component contextualComponent) bind(ctx context.Context) context.Context {
	if ctx == nil || component.logger == nil || component.logger.Logger == nil {
		return ctx
	}
	return commonlogger.WithContext(ctx, component.logger.Logger)
}

func (component *resourceComponent) Name() string { return component.name }

func (*resourceComponent) Run(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidProcess
	}
	<-ctx.Done()
	return nil
}

func (component *resourceComponent) Shutdown(context.Context) error {
	component.closeOnce.Do(func() { component.closeErr = component.close() })
	return component.closeErr
}

func parseRSAPrivateKey(key []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(key)
	if block == nil {
		return nil, errors.New("bootstrap: failed to parse PEM block for private key")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	rsaKey, ok := priv.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("bootstrap: private key is not RSA")
	}
	return rsaKey, nil
}

func parseRSAPublicKey(key []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(key)
	if block == nil {
		return nil, errors.New("bootstrap: failed to parse PEM block for public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if rsaPub, parseErr := x509.ParsePKCS1PublicKey(block.Bytes); parseErr == nil {
			return rsaPub, nil
		}
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("bootstrap: public key is not RSA")
	}
	return rsaPub, nil
}
