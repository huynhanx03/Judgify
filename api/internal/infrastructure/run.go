package infrastructure

import (
	"github.com/huynhanx03/judgify/internal/di"
)

// Run starts the Identity service.
func Run() error {
	LoadConfig()
	SetupLogger()
	SetupEnt()
	SetupCache()
	SetupKeys()
	di.SetupDependencies()

	Initialized()

	http := NewHTTPServer()
	return http.Run()
}
