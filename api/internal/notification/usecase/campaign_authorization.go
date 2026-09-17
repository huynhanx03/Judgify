package usecase

import (
	"context"
	"errors"

	"github.com/huynhanx03/go-common/pkg/security/authorization"

	"github.com/huynhanx03/judgify/internal/authctx"
	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

var ErrInvalidCampaignCapabilityChecker = errors.New(
	"notification campaign: invalid capability checker",
)

// CampaignCapabilityChecker reuses the canonical authenticated principal and
// Casbin-backed authorizer. Audience selection is domain data, so this check
// belongs after the request/revision has been loaded instead of in a
// transport middleware that cannot see the selected audience.
type CampaignCapabilityChecker struct {
	authorizer authorization.Authorizer
}

func NewCampaignCapabilityChecker(
	authorizer authorization.Authorizer,
) (*CampaignCapabilityChecker, error) {
	if authorizer == nil {
		return nil, ErrInvalidCampaignCapabilityChecker
	}
	return &CampaignCapabilityChecker{authorizer: authorizer}, nil
}

var _ ports.CampaignCapabilityChecker = (*CampaignCapabilityChecker)(nil)

func (checker *CampaignCapabilityChecker) RequireAudienceCapability(
	ctx context.Context,
	audienceKind string,
) error {
	if checker == nil || checker.authorizer == nil || ctx == nil {
		return ports.ErrCampaignPermissionDenied
	}
	action := permissions.PermissionScopeCompose
	switch audienceKind {
	case contract.AudienceExplicitUsers:
	case contract.AudienceRole:
		action = permissions.PermissionScopeBroadcastRole
	case contract.AudienceAllActive:
		action = permissions.PermissionScopeBroadcastAllActive
	default:
		return ports.ErrCampaignPermissionDenied
	}
	principal, ok := authctx.Principal(ctx)
	if !ok {
		return ports.ErrCampaignPermissionDenied
	}
	decision, err := checker.authorizer.Authorize(ctx, authorization.Request{
		Principal: principal,
		Resource:  permissions.ResourceKeyNotification,
		Action:    action,
	})
	if err != nil {
		return err
	}
	if !decision.Allowed {
		return ports.ErrCampaignPermissionDenied
	}
	return nil
}
