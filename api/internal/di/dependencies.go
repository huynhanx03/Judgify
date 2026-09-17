package di

import (
	"fmt"

	"github.com/huynhanx03/go-common/pkg/common/cache"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/pkg/settings"
)

// Dependencies is the immutable API composition boundary. Resource ownership
// remains with bootstrap.Process; domain containers only borrow these explicit
// references for the lifetime of the composed API role.
type Dependencies struct {
	Database *internalent.EntClient
	Cache    cache.LocalCache[string, any]
	Config   settings.Config
	Logger   *commonlogger.LoggerZap
}

func (dependencies Dependencies) validate() error {
	if dependencies.Database == nil || dependencies.Database.Client() == nil ||
		dependencies.Cache == nil || dependencies.Logger == nil ||
		dependencies.Logger.Logger == nil {
		return fmt.Errorf("setup dependencies: invalid process resources")
	}
	return nil
}
