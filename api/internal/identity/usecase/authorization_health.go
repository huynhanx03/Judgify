package usecase

import (
	"context"
	"time"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"
	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"
)

const authorizationHealthStateTimeout = time.Second

// Check implements commonhealth.Checker. Readiness is fenced against the
// durable revision on every probe so a policy commit made by another process
// cannot leave this process serving stale authorization decisions.
func (service *authorizationService) Check(ctx context.Context) error {
	if service == nil || service.repository == nil || service.runtime == nil || ctx == nil {
		return authcasbin.ErrUnhealthy
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	stateCtx, cancel := context.WithTimeout(ctx, authorizationHealthStateTimeout)
	defer cancel()
	state, err := service.repository.State(stateCtx)
	if err != nil {
		// Repository errors may contain driver or policy details. Health callers
		// receive only the stable, non-sensitive classification.
		return authcasbin.ErrUnhealthy
	}
	durableDesired := state.DesiredRevision
	localDesired := service.runtime.DesiredRevision()
	applied := service.runtime.AppliedRevision()
	if durableDesired <= 0 || durableDesired != localDesired || localDesired != applied {
		return authcasbin.ErrUnhealthy
	}
	return nil
}

var _ commonhealth.Checker = (*authorizationService)(nil)
