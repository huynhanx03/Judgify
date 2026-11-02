package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ports"
)

func TestAuthorizationHealthChecksDesiredAppliedFence(t *testing.T) {
	fixture := newAuthorizationFixture(t)
	require.NoError(t, fixture.service.Check(context.Background()))

	fixture.service.runtime.SetDesiredRevision(fixture.service.runtime.AppliedRevision() + 1)
	require.Error(t, fixture.service.Check(context.Background()))
}

func TestAuthorizationHealthDetectsRemoteDurableRevision(t *testing.T) {
	fixture := newAuthorizationFixture(t)
	require.NoError(t, fixture.service.Check(context.Background()))

	// Simulate another API instance committing a policy mutation against the
	// shared durable authorization fence. This instance has not reconciled yet.
	fixture.repository.mu.Lock()
	fixture.repository.state.DesiredRevision++
	fixture.repository.mu.Unlock()

	require.ErrorIs(t, fixture.service.Check(context.Background()), authcasbin.ErrUnhealthy)
}

func TestAuthorizationHealthBoundsAndSanitizesDurableStateFailure(t *testing.T) {
	fixture := newAuthorizationFixture(t)
	repositoryFailure := errors.New("database details must not cross the health boundary")
	stateCalled := false
	stateHadDeadline := false
	var stateBudget time.Duration
	fixture.service.repository = authorizationHealthStateRepository{
		AuthorizationPolicyRepository: fixture.repository,
		state: func(ctx context.Context) (ports.AuthorizationState, error) {
			stateCalled = true
			deadline, ok := ctx.Deadline()
			stateHadDeadline = ok
			if ok {
				stateBudget = time.Until(deadline)
			}
			return ports.AuthorizationState{}, repositoryFailure
		},
	}

	err := fixture.service.Check(context.Background())
	require.ErrorIs(t, err, authcasbin.ErrUnhealthy)
	require.NotErrorIs(t, err, repositoryFailure)
	require.NotContains(t, err.Error(), repositoryFailure.Error())
	require.True(t, stateCalled)
	require.True(t, stateHadDeadline)
	require.Positive(t, stateBudget)
	require.LessOrEqual(t, stateBudget, time.Second)
}

type authorizationHealthStateRepository struct {
	ports.AuthorizationPolicyRepository
	state func(context.Context) (ports.AuthorizationState, error)
}

func (repository authorizationHealthStateRepository) State(
	ctx context.Context,
) (ports.AuthorizationState, error) {
	return repository.state(ctx)
}
