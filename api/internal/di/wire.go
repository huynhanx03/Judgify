package di

import (
	identityDi "github.com/huynhanx03/judgify/internal/identity/di"
	problemDi "github.com/huynhanx03/judgify/internal/problem/di"
)

// SetupDependencies initializes all domain dependencies and returns the global container.
func SetupDependencies() *Container {
	identityContainer := identityDi.NewIdentityContainer()
	problemContainer := problemDi.NewProblemContainer()

	container := &Container{
		Identity: identityContainer,
		Problem:  problemContainer,
	}

	GlobalContainer = container
	return container
}
