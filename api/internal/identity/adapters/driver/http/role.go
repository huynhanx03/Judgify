package http

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	"github.com/huynhanx03/judgify/internal/application/workflow/authorizationadmin"
	"github.com/huynhanx03/judgify/internal/application/workflow/rolemetadata"
	"github.com/huynhanx03/judgify/internal/authctx"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	"github.com/huynhanx03/judgify/internal/ports"
)

func roleMetadataRequest(
	ctx context.Context,
	req *dto.UpdateRoleRequest,
) (rolemetadata.Request, error) {
	if req == nil {
		return rolemetadata.Request{}, apperr.New(
			apperr.CodeParamInvalid,
			"invalid role command",
			rolemetadata.ErrInvalidWorkflow,
		)
	}
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	headerID, headerOK := identityCommandID(ctx)
	if !actorOK || !sessionOK || actorID == uuid.Nil || sessionID == uuid.Nil {
		return rolemetadata.Request{}, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	if !headerOK || headerID == uuid.Nil || headerID != req.CommandID {
		return rolemetadata.Request{}, apperr.New(
			apperr.CodeParamInvalid,
			"one matching canonical idempotency key is required",
			rolemetadata.ErrInvalidWorkflow,
		)
	}
	return rolemetadata.Request{
		RoleID:            req.ID,
		CommandID:         req.CommandID,
		ExpectedVersion:   req.ExpectedVersion,
		Reason:            req.Reason,
		Name:              req.Name,
		Description:       req.Description,
		ConfirmationToken: req.ConfirmationToken,
		ActorID:           actorID,
		SessionID:         sessionID,
	}, nil
}

func roleCreateCommand(
	ctx context.Context,
	req *dto.CreateRoleRequest,
) (*dto.CreateRoleRequest, [sha256.Size]byte, error) {
	if req == nil {
		return nil, [sha256.Size]byte{}, apperr.New(
			apperr.CodeParamInvalid,
			"invalid role command",
			nil,
		)
	}
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	headerID, headerOK := identityCommandID(ctx)
	if !actorOK || !sessionOK || actorID == uuid.Nil || sessionID == uuid.Nil {
		return nil, [sha256.Size]byte{}, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	if !headerOK || headerID == uuid.Nil || headerID != req.CommandID {
		return nil, [sha256.Size]byte{}, apperr.New(
			apperr.CodeParamInvalid,
			"one matching canonical idempotency key is required",
			nil,
		)
	}

	req.Reason = strings.TrimSpace(req.Reason)
	document, err := json.Marshal(struct {
		ActorID         uuid.UUID `json:"actor_id"`
		SessionID       uuid.UUID `json:"session_id"`
		CommandID       uuid.UUID `json:"command_id"`
		ExpectedVersion int64     `json:"expected_version"`
		Reason          string    `json:"reason"`
		Key             string    `json:"key"`
		Name            string    `json:"name"`
		Description     string    `json:"description"`
	}{
		ActorID:         actorID,
		SessionID:       sessionID,
		CommandID:       req.CommandID,
		ExpectedVersion: req.ExpectedVersion,
		Reason:          req.Reason,
		Key:             req.Key,
		Name:            req.Name,
		Description:     req.Description,
	})
	if err != nil {
		return nil, [sha256.Size]byte{}, apperr.New(
			apperr.CodeParamInvalid,
			"invalid role command",
			err,
		)
	}
	hash, err := admincommand.ConfirmationRequestHash(
		"identity.role.metadata.create.v1",
		document,
	)
	if err != nil {
		return nil, [sha256.Size]byte{}, apperr.New(
			apperr.CodeParamInvalid,
			"invalid role command",
			err,
		)
	}
	return req, hash, nil
}

// RoleHandler defines the role HTTP handler interface.
type RoleHandler interface {
	FindAll(ctx context.Context) ([]*roleResponse, error)
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*roleResponse], error)
	Get(ctx context.Context, req *getRoleRequest) (*roleResponse, error)
	Create(ctx context.Context, req *createRoleRequest) (*roleMutationReceiptResponse, error)
	PreviewMetadata(ctx context.Context, req *roleCommandRequest) (*roleMetadataPreviewResponse, error)
	ApplyMetadata(ctx context.Context, req *roleCommandRequest) (*roleMutationReceiptResponse, error)
	PreviewArchive(ctx context.Context, req *roleCommandRequest) (*roleMetadataPreviewResponse, error)
	ApplyArchive(ctx context.Context, req *roleCommandRequest) (*roleMutationReceiptResponse, error)
	GetPolicies(ctx context.Context, req *getRoleRequest) (*rolePolicySnapshotResponse, error)
	SetPolicies(ctx context.Context, req *rolePoliciesUpdateRequest) (*rolePolicySnapshotResponse, error)
	PreviewPolicies(ctx context.Context, req *rolePoliciesCommandRequest) (*rolePolicyPreviewResponse, error)
	ApplyPolicies(ctx context.Context, req *rolePoliciesCommandRequest) (*rolePolicyMutationReceiptResponse, error)
}

type roleHandler struct {
	handler.BaseHandler
	roleService      ports.RoleService
	policyWorkflow   *authorizationadmin.Workflow
	metadataWorkflow *rolemetadata.Workflow
}

// NewRoleHandler creates a new RoleHandler instance.
func NewRoleHandler(
	roleService ports.RoleService,
	policyWorkflow *authorizationadmin.Workflow,
	metadataWorkflow *rolemetadata.Workflow,
) RoleHandler {
	return &roleHandler{
		roleService:      roleService,
		policyWorkflow:   policyWorkflow,
		metadataWorkflow: metadataWorkflow,
	}
}

func (h *roleHandler) PreviewPolicies(
	ctx context.Context,
	req *rolePoliciesCommandRequest,
) (*rolePolicyPreviewResponse, error) {
	if h == nil || h.policyWorkflow == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authorization workflow unavailable", nil)
	}
	command, err := toRolePoliciesCommand(req)
	if err != nil {
		return nil, err
	}
	request, err := rolePolicyWorkflowRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.policyWorkflow.PreviewRolePolicies(ctx, request)
	return toRolePolicyPreviewResponse(response), mapAuthorizationAdminError(err)
}

func (h *roleHandler) ApplyPolicies(
	ctx context.Context,
	req *rolePoliciesCommandRequest,
) (*rolePolicyMutationReceiptResponse, error) {
	if h == nil || h.policyWorkflow == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authorization workflow unavailable", nil)
	}
	command, err := toRolePoliciesCommand(req)
	if err != nil {
		return nil, err
	}
	request, err := rolePolicyWorkflowRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.policyWorkflow.ApplyRolePolicies(ctx, request)
	return toRolePolicyMutationReceiptResponse(response), mapAuthorizationAdminError(err)
}

func rolePolicyWorkflowRequest(
	ctx context.Context,
	req *dto.RolePoliciesCommandRequest,
) (authorizationadmin.RolePolicyRequest, error) {
	if req == nil {
		return authorizationadmin.RolePolicyRequest{}, apperr.New(
			apperr.CodeParamInvalid,
			"invalid authorization command",
			authorizationadmin.ErrInvalidWorkflow,
		)
	}
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	headerID, headerOK := identityCommandID(ctx)
	if !actorOK || !sessionOK {
		return authorizationadmin.RolePolicyRequest{}, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	if !headerOK || headerID == uuid.Nil || headerID != req.CommandID {
		return authorizationadmin.RolePolicyRequest{}, apperr.New(
			apperr.CodeParamInvalid,
			"one matching canonical idempotency key is required",
			authorizationadmin.ErrInvalidWorkflow,
		)
	}
	return authorizationadmin.RolePolicyRequest{
		RoleID:            req.ID,
		CommandID:         req.CommandID,
		ExpectedRevision:  req.ExpectedRevision,
		Reason:            req.Reason,
		Rules:             req.Rules,
		ConfirmationToken: req.ConfirmationToken,
		ActorID:           actorID,
		SessionID:         sessionID,
	}, nil
}

func mapAuthorizationAdminError(err error) error {
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
			"authorization confirmation is stale",
			err,
		).WithDetails(map[string]any{"reason": "confirmation_stale"})
	case errors.Is(err, admincommand.ErrInvalidCommandMeta),
		errors.Is(err, admincommand.ErrInvalidConfirmation),
		errors.Is(err, authorizationadmin.ErrInvalidWorkflow):
		return apperr.New(
			apperr.CodeParamInvalid,
			"invalid authorization command",
			err,
		)
	default:
		return err
	}
}

func mapRoleMetadataError(err error) error {
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
			"role metadata confirmation is stale",
			err,
		).WithDetails(map[string]any{"reason": "confirmation_stale"})
	case errors.Is(err, admincommand.ErrInvalidCommandMeta),
		errors.Is(err, admincommand.ErrInvalidConfirmation),
		errors.Is(err, rolemetadata.ErrInvalidWorkflow):
		return apperr.New(
			apperr.CodeParamInvalid,
			"invalid role metadata command",
			err,
		)
	default:
		return err
	}
}

// FindAll retrieves all roles without pagination.
func (h *roleHandler) FindAll(ctx context.Context) ([]*roleResponse, error) {
	result, err := h.roleService.FindAll(ctx)
	return toRoleResponses(result), err
}

// Find retrieves roles with pagination.
func (h *roleHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*roleResponse], error) {
	result, err := h.roleService.Find(ctx, req)
	return toRolePage(result), err
}

// Get retrieves a role by ID.
func (h *roleHandler) Get(ctx context.Context, req *getRoleRequest) (*roleResponse, error) {
	request, err := toGetRoleRequest(req)
	if err != nil {
		return nil, err
	}
	result, err := h.roleService.Get(ctx, request.ID)
	return toRoleResponse(result), err
}

// Create creates a new role.
func (h *roleHandler) Create(ctx context.Context, req *createRoleRequest) (*roleMutationReceiptResponse, error) {
	command, err := toCreateRoleRequest(req)
	if err != nil {
		return nil, err
	}
	request, hash, err := roleCreateCommand(ctx, command)
	if err != nil {
		return nil, err
	}
	result, err := h.roleService.Create(ctx, request, hash)
	return toRoleMutationReceiptResponse(result), err
}

func (h *roleHandler) PreviewMetadata(
	ctx context.Context,
	req *roleCommandRequest,
) (*roleMetadataPreviewResponse, error) {
	if h == nil || h.metadataWorkflow == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "role metadata workflow unavailable", nil)
	}
	command, err := toUpdateRoleRequest(req)
	if err != nil {
		return nil, err
	}
	request, err := roleMetadataRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.metadataWorkflow.PreviewUpdate(ctx, request)
	return toRoleMetadataPreviewResponse(response), mapRoleMetadataError(err)
}

func (h *roleHandler) ApplyMetadata(
	ctx context.Context,
	req *roleCommandRequest,
) (*roleMutationReceiptResponse, error) {
	if h == nil || h.metadataWorkflow == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "role metadata workflow unavailable", nil)
	}
	command, err := toUpdateRoleRequest(req)
	if err != nil {
		return nil, err
	}
	request, err := roleMetadataRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.metadataWorkflow.ApplyUpdate(ctx, request)
	return toRoleMutationReceiptResponse(response), mapRoleMetadataError(err)
}

func (h *roleHandler) PreviewArchive(
	ctx context.Context,
	req *roleCommandRequest,
) (*roleMetadataPreviewResponse, error) {
	if h == nil || h.metadataWorkflow == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "role metadata workflow unavailable", nil)
	}
	command, err := toUpdateRoleRequest(req)
	if err != nil {
		return nil, err
	}
	request, err := roleMetadataRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.metadataWorkflow.PreviewArchive(ctx, request)
	return toRoleMetadataPreviewResponse(response), mapRoleMetadataError(err)
}

func (h *roleHandler) ApplyArchive(
	ctx context.Context,
	req *roleCommandRequest,
) (*roleMutationReceiptResponse, error) {
	if h == nil || h.metadataWorkflow == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "role metadata workflow unavailable", nil)
	}
	command, err := toUpdateRoleRequest(req)
	if err != nil {
		return nil, err
	}
	request, err := roleMetadataRequest(ctx, command)
	if err != nil {
		return nil, err
	}
	response, err := h.metadataWorkflow.ApplyArchive(ctx, request)
	return toRoleMutationReceiptResponse(response), mapRoleMetadataError(err)
}

func (h *roleHandler) GetPolicies(ctx context.Context, req *getRoleRequest) (*rolePolicySnapshotResponse, error) {
	request, err := toGetRoleRequest(req)
	if err != nil {
		return nil, err
	}
	result, err := h.roleService.GetPolicies(ctx, request.ID)
	return toRolePolicySnapshotResponse(result), err
}

func (h *roleHandler) SetPolicies(ctx context.Context, req *rolePoliciesUpdateRequest) (*rolePolicySnapshotResponse, error) {
	request, err := toRolePoliciesUpdate(req)
	if err != nil {
		return nil, err
	}
	result, err := h.roleService.SetPolicies(ctx, request.ID, request)
	return toRolePolicySnapshotResponse(result), err
}
