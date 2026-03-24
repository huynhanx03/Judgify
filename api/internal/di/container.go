package di

import (
	cultivationDi "github.com/huynhanx03/judgify/internal/cultivation/di"
	identityDi "github.com/huynhanx03/judgify/internal/identity/di"
	problemDi "github.com/huynhanx03/judgify/internal/problem/di"
	submissionDi "github.com/huynhanx03/judgify/internal/submission/di"
	"github.com/huynhanx03/judgify/pkg/mq/forge"
)

// Container holds all domain-specific dependency containers.
type Container struct {
	Identity    *identityDi.IdentityContainer
	Problem     *problemDi.ProblemContainer
	Cultivation *cultivationDi.CultivationContainer
	Submission  *submissionDi.SubmissionContainer
	Broker      *forge.Broker
}

// GlobalContainer is the global instance of Container.
var GlobalContainer *Container
