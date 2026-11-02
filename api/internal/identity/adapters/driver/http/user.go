package http

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	"github.com/huynhanx03/judgify/internal/application/workflow/authorizationadmin"
	"github.com/huynhanx03/judgify/internal/application/workflow/userinvitation"
	"github.com/huynhanx03/judgify/internal/application/workflow/userlifecycle"
	"github.com/huynhanx03/judgify/internal/authctx"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	"github.com/huynhanx03/judgify/internal/ports"
)

type UserHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*userResponse], error)
	PreviewInvitation(ctx context.Context, req *userInvitationCommandRequest) (*userInvitationPreviewResponse, error)
	ApplyInvitation(ctx context.Context, req *userInvitationCommandRequest) (*userInvitationReceiptResponse, error)
	PreviewInvitationResend(ctx context.Context, req *userInvitationResendCommandRequest) (*userInvitationResendPreviewResponse, error)
	ApplyInvitationResend(ctx context.Context, req *userInvitationResendCommandRequest) (*userInvitationResendReceiptResponse, error)
	PreviewRoles(ctx context.Context, req *userRolesCommandRequest) (*userRolePreviewResponse, error)
	ApplyRoles(ctx context.Context, req *userRolesCommandRequest) (*userRoleMutationReceiptResponse, error)
	PreviewSuspend(ctx context.Context, req *userLifecycleCommandRequest) (*userLifecyclePreviewResponse, error)
	ApplySuspend(ctx context.Context, req *userLifecycleCommandRequest) (*userLifecycleReceiptResponse, error)
	PreviewReactivate(ctx context.Context, req *userLifecycleCommandRequest) (*userLifecyclePreviewResponse, error)
	ApplyReactivate(ctx context.Context, req *userLifecycleCommandRequest) (*userLifecycleReceiptResponse, error)
	PreviewDelete(ctx context.Context, req *userLifecycleCommandRequest) (*userLifecyclePreviewResponse, error)
	ApplyDelete(ctx context.Context, req *userLifecycleCommandRequest) (*userLifecycleReceiptResponse, error)
	GetProfile(ctx context.Context) (*profileAttrsResponse, error)
	GetProfileAttributes(ctx context.Context) (*profileAttributesResponse, error)
	UpdateProfileAttributes(ctx context.Context, req *updateProfileAttributesRequest) (*profileAttributesResponse, error)
}

type userHandler struct {
	userService        ports.UserService
	roleWorkflow       *authorizationadmin.Workflow
	lifecycleWorkflow  *userlifecycle.Workflow
	invitationWorkflow *userinvitation.Workflow
}

func NewUserHandler(
	userService ports.UserService,
	roleWorkflow *authorizationadmin.Workflow,
	lifecycleWorkflow *userlifecycle.Workflow,
	invitationWorkflow *userinvitation.Workflow,
) UserHandler {
	return &userHandler{
		userService:        userService,
		roleWorkflow:       roleWorkflow,
		lifecycleWorkflow:  lifecycleWorkflow,
		invitationWorkflow: invitationWorkflow,
	}
}

func (h *userHandler) PreviewRoles(
	ctx context.Context,
	req *userRolesCommandRequest,
) (*userRolePreviewResponse, error) {
	if h == nil || h.roleWorkflow == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"authorization workflow unavailable",
			nil,
		)
	}
	command, err := toUserRolesCommand(req)
	if err != nil {
		return nil, err
	}
	request, err := userRoleWorkflowRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.roleWorkflow.PreviewUserRoles(ctx, request)
	return toUserRolePreviewResponse(response), mapAuthorizationAdminError(err)
}

func (h *userHandler) ApplyRoles(
	ctx context.Context,
	req *userRolesCommandRequest,
) (*userRoleMutationReceiptResponse, error) {
	if h == nil || h.roleWorkflow == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"authorization workflow unavailable",
			nil,
		)
	}
	command, err := toUserRolesCommand(req)
	if err != nil {
		return nil, err
	}
	request, err := userRoleWorkflowRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.roleWorkflow.ApplyUserRoles(ctx, request)
	return toUserRoleMutationReceiptResponse(response), mapAuthorizationAdminError(err)
}

func userRoleWorkflowRequest(
	ctx context.Context,
	req *dto.UserRolesCommandRequest,
) (authorizationadmin.UserRoleRequest, error) {
	if req == nil {
		return authorizationadmin.UserRoleRequest{}, apperr.New(
			apperr.CodeParamInvalid,
			"invalid authorization command",
			authorizationadmin.ErrInvalidWorkflow,
		)
	}
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	headerID, headerOK := identityCommandID(ctx)
	if !actorOK || !sessionOK {
		return authorizationadmin.UserRoleRequest{}, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	if !headerOK || headerID == uuid.Nil || headerID != req.CommandID {
		return authorizationadmin.UserRoleRequest{}, apperr.New(
			apperr.CodeParamInvalid,
			"one matching canonical idempotency key is required",
			authorizationadmin.ErrInvalidWorkflow,
		)
	}
	return authorizationadmin.UserRoleRequest{
		UserID:            req.UserID,
		CommandID:         req.CommandID,
		ExpectedRevision:  req.ExpectedRevision,
		Reason:            req.Reason,
		Roles:             req.Roles,
		ConfirmationToken: req.ConfirmationToken,
		ActorID:           actorID,
		SessionID:         sessionID,
	}, nil
}

func (h *userHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*userResponse], error) {
	result, err := h.userService.Find(ctx, req)
	return toUserPage(result), err
}

func (h *userHandler) PreviewInvitation(
	ctx context.Context,
	req *userInvitationCommandRequest,
) (*userInvitationPreviewResponse, error) {
	if h == nil || h.invitationWorkflow == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"user invitation workflow unavailable",
			nil,
		)
	}
	request, err := userInvitationWorkflowRequest(ctx, toUserInvitationCommandRequest(req))
	if err != nil {
		return nil, err
	}
	response, err := h.invitationWorkflow.Preview(ctx, request)
	return toUserInvitationPreviewResponse(response), mapUserInvitationError(err)
}

func (h *userHandler) ApplyInvitation(
	ctx context.Context,
	req *userInvitationCommandRequest,
) (*userInvitationReceiptResponse, error) {
	if h == nil || h.invitationWorkflow == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"user invitation workflow unavailable",
			nil,
		)
	}
	request, err := userInvitationWorkflowRequest(ctx, toUserInvitationCommandRequest(req))
	if err != nil {
		return nil, err
	}
	response, err := h.invitationWorkflow.Apply(ctx, request)
	return toUserInvitationReceiptResponse(response), mapUserInvitationError(err)
}

func userInvitationWorkflowRequest(
	ctx context.Context,
	req *dto.UserInvitationCommandRequest,
) (userinvitation.Request, error) {
	if req == nil {
		return userinvitation.Request{}, apperr.New(
			apperr.CodeParamInvalid,
			"invalid user invitation command",
			userinvitation.ErrInvalidWorkflow,
		)
	}
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	headerID, headerOK := identityCommandID(ctx)
	if !actorOK || !sessionOK {
		return userinvitation.Request{}, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	if !headerOK || headerID == uuid.Nil || headerID != req.CommandID {
		return userinvitation.Request{}, apperr.New(
			apperr.CodeParamInvalid,
			"one matching canonical idempotency key is required",
			userinvitation.ErrInvalidWorkflow,
		)
	}
	return userinvitation.Request{
		ProfileSchemaEvidence: req.ProfileSchemaEvidence,
		CommandID:             req.CommandID, Username: req.Username,
		RecoveryEmail: req.RecoveryEmail,
		Roles:         append([]string(nil), req.Roles...), Reason: req.Reason,
		ConfirmationToken: req.ConfirmationToken,
		ActorID:           actorID, SessionID: sessionID,
	}, nil
}

func mapUserInvitationError(err error) error {
	if err == nil {
		return nil
	}
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) {
		return err
	}
	switch {
	case errors.Is(err, admincommand.ErrRecentAuthenticationRequired):
		return apperr.New(
			apperr.CodeForbidden,
			"recent authentication required",
			err,
		).WithDetails(map[string]any{"reason": "reauthentication_required"})
	case errors.Is(err, admincommand.ErrConfirmationExpired),
		errors.Is(err, admincommand.ErrConfirmationMismatch):
		return apperr.New(
			apperr.CodeConflict,
			"user invitation confirmation is stale",
			err,
		).WithDetails(map[string]any{"reason": "confirmation_stale"})
	case errors.Is(err, admincommand.ErrInvalidCommandMeta),
		errors.Is(err, admincommand.ErrInvalidConfirmation),
		errors.Is(err, userinvitation.ErrInvalidWorkflow):
		return apperr.New(
			apperr.CodeParamInvalid,
			"invalid user invitation command",
			err,
		)
	default:
		return err
	}
}

func (h *userHandler) PreviewInvitationResend(
	ctx context.Context,
	req *userInvitationResendCommandRequest,
) (*userInvitationResendPreviewResponse, error) {
	if h == nil || h.invitationWorkflow == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"user invitation workflow unavailable",
			nil,
		)
	}
	command, err := toUserInvitationResendCommandRequest(req)
	if err != nil {
		return nil, apperr.New(apperr.CodeParamInvalid, "invalid invitation resend command", err)
	}
	request, err := userInvitationResendWorkflowRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.invitationWorkflow.PreviewResend(ctx, request)
	return toUserInvitationResendPreviewResponse(response), mapUserInvitationError(err)
}

func (h *userHandler) ApplyInvitationResend(
	ctx context.Context,
	req *userInvitationResendCommandRequest,
) (*userInvitationResendReceiptResponse, error) {
	if h == nil || h.invitationWorkflow == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"user invitation workflow unavailable",
			nil,
		)
	}
	command, err := toUserInvitationResendCommandRequest(req)
	if err != nil {
		return nil, apperr.New(apperr.CodeParamInvalid, "invalid invitation resend command", err)
	}
	request, err := userInvitationResendWorkflowRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.invitationWorkflow.ApplyResend(ctx, request)
	return toUserInvitationResendReceiptResponse(response), mapUserInvitationError(err)
}

func userInvitationResendWorkflowRequest(
	ctx context.Context,
	req *dto.UserInvitationResendCommandRequest,
) (userinvitation.ResendRequest, error) {
	if req == nil {
		return userinvitation.ResendRequest{}, apperr.New(
			apperr.CodeParamInvalid,
			"invalid invitation resend command",
			userinvitation.ErrInvalidWorkflow,
		)
	}
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	headerID, headerOK := identityCommandID(ctx)
	if !actorOK || !sessionOK {
		return userinvitation.ResendRequest{}, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	if !headerOK || headerID == uuid.Nil || headerID != req.CommandID {
		return userinvitation.ResendRequest{}, apperr.New(
			apperr.CodeParamInvalid,
			"one matching canonical idempotency key is required",
			userinvitation.ErrInvalidWorkflow,
		)
	}
	return userinvitation.ResendRequest{
		UserID: req.ID, CommandID: req.CommandID,
		ExpectedUserVersion:       req.ExpectedUserVersion,
		ExpectedInvitationVersion: req.ExpectedInvitationVersion,
		Reason:                    req.Reason, ConfirmationToken: req.ConfirmationToken,
		ActorID: actorID, SessionID: sessionID,
	}, nil
}

func (h *userHandler) PreviewSuspend(
	ctx context.Context,
	req *userLifecycleCommandRequest,
) (*userLifecyclePreviewResponse, error) {
	return h.previewLifecycle(ctx, "suspend", req)
}

func (h *userHandler) ApplySuspend(
	ctx context.Context,
	req *userLifecycleCommandRequest,
) (*userLifecycleReceiptResponse, error) {
	return h.applyLifecycle(ctx, "suspend", req)
}

func (h *userHandler) PreviewReactivate(
	ctx context.Context,
	req *userLifecycleCommandRequest,
) (*userLifecyclePreviewResponse, error) {
	return h.previewLifecycle(ctx, "reactivate", req)
}

func (h *userHandler) ApplyReactivate(
	ctx context.Context,
	req *userLifecycleCommandRequest,
) (*userLifecycleReceiptResponse, error) {
	return h.applyLifecycle(ctx, "reactivate", req)
}

func (h *userHandler) PreviewDelete(
	ctx context.Context,
	req *userLifecycleCommandRequest,
) (*userLifecyclePreviewResponse, error) {
	return h.previewLifecycle(ctx, "delete", req)
}

func (h *userHandler) ApplyDelete(
	ctx context.Context,
	req *userLifecycleCommandRequest,
) (*userLifecycleReceiptResponse, error) {
	return h.applyLifecycle(ctx, "delete", req)
}

func (h *userHandler) previewLifecycle(
	ctx context.Context,
	action string,
	req *userLifecycleCommandRequest,
) (*userLifecyclePreviewResponse, error) {
	if h == nil || h.lifecycleWorkflow == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"user lifecycle workflow unavailable",
			nil,
		)
	}
	command, err := toUserLifecycleCommand(req)
	if err != nil {
		return nil, apperr.New(apperr.CodeParamInvalid, "invalid user lifecycle command", err)
	}
	request, err := userLifecycleWorkflowRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.lifecycleWorkflow.Preview(ctx, action, request)
	return toUserLifecyclePreviewResponse(response), mapUserLifecycleError(err)
}

func (h *userHandler) applyLifecycle(
	ctx context.Context,
	action string,
	req *userLifecycleCommandRequest,
) (*userLifecycleReceiptResponse, error) {
	if h == nil || h.lifecycleWorkflow == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"user lifecycle workflow unavailable",
			nil,
		)
	}
	command, err := toUserLifecycleCommand(req)
	if err != nil {
		return nil, apperr.New(apperr.CodeParamInvalid, "invalid user lifecycle command", err)
	}
	request, err := userLifecycleWorkflowRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.lifecycleWorkflow.Apply(ctx, action, request)
	return toUserLifecycleReceiptResponse(response), mapUserLifecycleError(err)
}

func userLifecycleWorkflowRequest(
	ctx context.Context,
	req *dto.UserLifecycleCommandRequest,
) (userlifecycle.Request, error) {
	if req == nil {
		return userlifecycle.Request{}, apperr.New(
			apperr.CodeParamInvalid,
			"invalid user lifecycle command",
			userlifecycle.ErrInvalidWorkflow,
		)
	}
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	headerID, headerOK := identityCommandID(ctx)
	if !actorOK || !sessionOK {
		return userlifecycle.Request{}, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	if !headerOK || headerID != req.CommandID {
		return userlifecycle.Request{}, apperr.New(
			apperr.CodeParamInvalid,
			"one matching canonical idempotency key is required",
			userlifecycle.ErrInvalidWorkflow,
		)
	}
	return userlifecycle.Request{
		UserID:            req.ID,
		CommandID:         req.CommandID,
		ExpectedVersion:   req.ExpectedVersion,
		Reason:            req.Reason,
		ConfirmationToken: req.ConfirmationToken,
		ActorID:           actorID,
		SessionID:         sessionID,
	}, nil
}

func mapUserLifecycleError(err error) error {
	if err == nil {
		return nil
	}
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) {
		return err
	}
	switch {
	case errors.Is(err, admincommand.ErrRecentAuthenticationRequired):
		return apperr.New(
			apperr.CodeForbidden,
			"recent authentication required",
			err,
		).WithDetails(map[string]any{"reason": "reauthentication_required"})
	case errors.Is(err, admincommand.ErrConfirmationExpired),
		errors.Is(err, admincommand.ErrConfirmationMismatch):
		return apperr.New(
			apperr.CodeConflict,
			"user lifecycle confirmation is stale",
			err,
		).WithDetails(map[string]any{"reason": "confirmation_stale"})
	case errors.Is(err, admincommand.ErrInvalidCommandMeta),
		errors.Is(err, admincommand.ErrInvalidConfirmation),
		errors.Is(err, userlifecycle.ErrInvalidWorkflow):
		return apperr.New(
			apperr.CodeParamInvalid,
			"invalid user lifecycle command",
			err,
		)
	default:
		return err
	}
}

func (h *userHandler) GetProfile(ctx context.Context) (*profileAttrsResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "user id not found in context", nil)
	}

	result, err := h.userService.GetProfile(ctx, userID)
	return toProfileAttrsResponse(result), err
}

func (h *userHandler) GetProfileAttributes(
	ctx context.Context,
) (*profileAttributesResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "user id not found in context", nil)
	}
	result, err := h.userService.GetProfileAttributes(ctx, userID)
	return toProfileAttributesResponse(result), err
}

func (h *userHandler) UpdateProfileAttributes(
	ctx context.Context,
	req *updateProfileAttributesRequest,
) (*profileAttributesResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "user id not found in context", nil)
	}
	request, err := toUpdateProfileAttributesRequest(req)
	if err != nil {
		return nil, err
	}
	result, err := h.userService.UpdateProfileAttributes(ctx, userID, request)
	return toProfileAttributesResponse(result), err
}
