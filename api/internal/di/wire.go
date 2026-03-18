package di

import (
	cultivationDi "github.com/huynhanx03/judgify/internal/cultivation/di"
	identityDi "github.com/huynhanx03/judgify/internal/identity/di"
	problemDi "github.com/huynhanx03/judgify/internal/problem/di"
)

// SetupDependencies initializes all domain dependencies and returns the global container.
func SetupDependencies() *Container {
	identityContainer := identityDi.NewIdentityContainer()
	problemContainer := problemDi.NewProblemContainer()
	cultivationContainer := cultivationDi.NewCultivationContainer()

	container := &Container{
		Identity:    identityContainer,
		Problem:     problemContainer,
		Cultivation: cultivationContainer,
	}

	GlobalContainer = container
	return container
}
