package bootstrap

import (
	"context"

	"github.com/huynhanx03/judgify/internal/infrastructure/configuration"
	"github.com/huynhanx03/judgify/pkg/settings"
)

// Configuration belongs to a resource-free package so finite operator
// commands do not pull the Ent/application graph into their binaries. These
// aliases preserve the bootstrap API used by the long-running composition
// roots.
type (
	Profile            = configuration.Profile
	ConfigError        = configuration.ConfigError
	SecretSource       = configuration.SecretSource
	OSFileSecretSource = configuration.OSFileSecretSource
)

const (
	ProfileAPI         = configuration.ProfileAPI
	ProfileJudgeWorker = configuration.ProfileJudgeWorker
	ProfileMigrate     = configuration.ProfileMigrate
	ProfileOperator    = configuration.ProfileOperator
	ProfileAll         = configuration.ProfileAll
)

var (
	ErrInvalidConfiguration = configuration.ErrInvalidConfiguration
	ErrInvalidSecretSource  = configuration.ErrInvalidSecretSource
)

func loadConfig(
	ctx context.Context,
	profile Profile,
	secrets SecretSource,
) (settings.Config, error) {
	return configuration.Load(ctx, profile, secrets)
}
