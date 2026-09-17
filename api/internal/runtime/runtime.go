// Package runtime selects one explicit process role and runs its components
// under the shared bounded lifecycle contract. It deliberately owns neither
// operating-system signals nor process termination; those belong to main.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/huynhanx03/go-common/pkg/common/lifecycle"
	"github.com/huynhanx03/go-common/pkg/correlation"
)

// Role is the independently deployable responsibility of one process.
type Role string

const (
	RoleAPI         Role = "api"
	RoleJudgeWorker Role = "judge-worker"
	RoleMigrate     Role = "migrate"
	RoleAll         Role = "all"
	maxComponents        = 64

	defaultShutdownTimeout          = 30 * time.Second
	defaultComponentShutdownTimeout = 15 * time.Second
)

var (
	ErrInvalidRole   = errors.New("runtime: invalid role")
	ErrAllDisabled   = errors.New("runtime: all role is disabled outside development")
	ErrNoBuilder     = errors.New("runtime: role builder is not configured")
	ErrNoComponents  = errors.New("runtime: long-running role has no components")
	ErrInvalidConfig = errors.New("runtime: invalid configuration")
)

// ComponentBuilder composes a role without starting background work.
type ComponentBuilder func(context.Context) ([]lifecycle.Component, error)

// Builders contains the mutually exclusive role composition roots. Migrate is
// finite and therefore intentionally returns no lifecycle components.
type Builders struct {
	API         ComponentBuilder
	JudgeWorker ComponentBuilder
	All         ComponentBuilder
	Migrate     func(context.Context) error
}

// Config controls role selection and bounded shutdown.
type Config struct {
	Mode                     string
	ShutdownTimeout          time.Duration
	ComponentShutdownTimeout time.Duration
	Builders                 Builders
}

// NewConfig applies the same production shutdown budgets to every binary.
func NewConfig(mode string, builders Builders) Config {
	return Config{
		Mode:                     mode,
		ShutdownTimeout:          defaultShutdownTimeout,
		ComponentShutdownTimeout: defaultComponentShutdownTimeout,
		Builders:                 builders,
	}
}

// ParseRole validates the externally supplied role without guessing.
func ParseRole(value string) (Role, error) {
	role := Role(strings.TrimSpace(value))
	if !role.valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidRole, value)
	}
	return role, nil
}

// Run builds exactly one role and blocks until its lifecycle finishes.
func Run(ctx context.Context, role Role, config Config) error {
	if !role.valid() {
		return fmt.Errorf("%w: %q", ErrInvalidRole, role)
	}
	if role == RoleAll && !allowsAllRole(config.Mode) {
		return ErrAllDisabled
	}
	if ctx == nil {
		return ErrInvalidConfig
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// One process-scoped CID follows composition, component lifecycles, and
	// shutdown. Message/job/connection handlers replace it with their durable
	// operation CID at the corresponding boundary.
	ctx = correlation.EnsureContext(ctx)
	if role == RoleMigrate {
		if config.Builders.Migrate == nil {
			return fmt.Errorf("%w: %s", ErrNoBuilder, role)
		}
		return config.Builders.Migrate(ctx)
	}

	builder := config.Builders.forRole(role)
	if builder == nil {
		return fmt.Errorf("%w: %s", ErrNoBuilder, role)
	}
	if config.ComponentShutdownTimeout < 0 {
		return fmt.Errorf("%w: component shutdown timeout", ErrInvalidConfig)
	}
	runner, err := lifecycle.New(lifecycle.Options{
		ShutdownTimeout: config.ShutdownTimeout,
		MaxComponents:   maxComponents,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	components, err := builder(ctx)
	if err != nil {
		return fmt.Errorf("compose %s role: %w", role, err)
	}
	if len(components) == 0 {
		return fmt.Errorf("%w: %s", ErrNoComponents, role)
	}
	for _, component := range components {
		if err := runner.Add(component, lifecycle.ComponentOptions{
			Required:        true,
			ShutdownTimeout: config.ComponentShutdownTimeout,
		}); err != nil {
			cleanupErr := runner.Shutdown(context.WithoutCancel(ctx))
			return errors.Join(
				fmt.Errorf("register %s component: %w", role, err),
				cleanupErr,
			)
		}
	}
	if err := runner.Run(ctx); err != nil {
		return fmt.Errorf("run %s role: %w", role, err)
	}
	return nil
}

func allowsAllRole(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "dev", "development", "local", "test":
		return true
	default:
		return false
	}
}

func (role Role) valid() bool {
	switch role {
	case RoleAPI, RoleJudgeWorker, RoleMigrate, RoleAll:
		return true
	default:
		return false
	}
}

func (builders Builders) forRole(role Role) ComponentBuilder {
	switch role {
	case RoleAPI:
		return builders.API
	case RoleJudgeWorker:
		return builders.JudgeWorker
	case RoleAll:
		return builders.All
	default:
		return nil
	}
}
