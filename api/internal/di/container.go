package di

import (
	identityDi "github.com/huynhanx03/judgify/internal/identity/di"
	problemDi "github.com/huynhanx03/judgify/internal/problem/di"
)

// Container holds all domain-specific dependency containers.
type Container struct {
	Identity *identityDi.IdentityContainer
	Problem  *problemDi.ProblemContainer
}

// GlobalContainer is the global instance of Container.
var GlobalContainer *Container
