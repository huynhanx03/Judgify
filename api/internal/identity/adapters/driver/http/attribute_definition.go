package http

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	"github.com/huynhanx03/judgify/internal/application/workflow/attributeadmin"
	"github.com/huynhanx03/judgify/internal/authctx"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	"github.com/huynhanx03/judgify/internal/ports"
)

// AttributeDefinitionHandler defines the attribute definition HTTP handler interface.
type AttributeDefinitionHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*attributeDefinitionResponse], error)
	Get(ctx context.Context, req *getAttributeDefinitionRequest) (*attributeDefinitionResponse, error)
	ListRevisions(ctx context.Context, req *listAttributeDefinitionRevisionsRequest) (*attributeDefinitionRevisionPageResponse, error)
	Create(ctx context.Context, req *createAttributeDefinitionRequest) (*attributeDefinitionResponse, error)
	Update(ctx context.Context, req *updateAttributeDefinitionRequest) (*attributeDefinitionResponse, error)
	PreviewActivation(ctx context.Context, req *attributeLifecycleCommandRequest) (*attributeLifecyclePreviewResponse, error)
	ApplyActivation(ctx context.Context, req *attributeLifecycleCommandRequest) (*attributeLifecycleReceiptResponse, error)
	PreviewArchive(ctx context.Context, req *attributeLifecycleCommandRequest) (*attributeLifecyclePreviewResponse, error)
	ApplyArchive(ctx context.Context, req *attributeLifecycleCommandRequest) (*attributeLifecycleReceiptResponse, error)
}

type attributeDefinitionHandler struct {
	handler.BaseHandler
	attrDefService ports.AttributeDefinitionService
	workflow       *attributeadmin.Workflow
}

// NewAttributeDefinitionHandler creates a new AttributeDefinitionHandler instance.
func NewAttributeDefinitionHandler(
	attrDefService ports.AttributeDefinitionService,
	workflow *attributeadmin.Workflow,
) AttributeDefinitionHandler {
	return &attributeDefinitionHandler{
		attrDefService: attrDefService,
		workflow:       workflow,
	}
}

// Find retrieves attribute definitions with pagination.
func (h *attributeDefinitionHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*attributeDefinitionResponse], error) {
	result, err := h.attrDefService.Find(ctx, req)
	return toAttributeDefinitionPage(result), err
}

// Get retrieves an attribute definition by ID.
func (h *attributeDefinitionHandler) Get(ctx context.Context, req *getAttributeDefinitionRequest) (*attributeDefinitionResponse, error) {
	request, err := toGetAttributeDefinitionRequest(req)
	if err != nil {
		return nil, err
	}
	result, err := h.attrDefService.Get(ctx, request.ID)
	return toAttributeDefinitionResponse(result), err
}

func (h *attributeDefinitionHandler) ListRevisions(
	ctx context.Context,
	req *listAttributeDefinitionRevisionsRequest,
) (*attributeDefinitionRevisionPageResponse, error) {
	request, err := toListAttributeDefinitionRevisionsRequest(req)
	if err != nil {
		return nil, apperr.New(apperr.CodeParamInvalid, "attribute revision query is required", nil)
	}
	result, err := h.attrDefService.ListRevisions(ctx, request.ID, request.Cursor, request.Limit)
	return toAttributeDefinitionRevisionPageResponse(result), err
}

// Create creates a new attribute definition.
func (h *attributeDefinitionHandler) Create(ctx context.Context, req *createAttributeDefinitionRequest) (*attributeDefinitionResponse, error) {
	request, err := toCreateAttributeDefinitionRequest(req)
	if err != nil || request == nil {
		return nil, apperr.New(apperr.CodeParamInvalid, "attribute definition command is required", nil)
	}
	if err := requireAttributeCommandID(ctx, request.CommandID); err != nil {
		return nil, err
	}
	result, err := h.attrDefService.Create(ctx, request)
	return toAttributeDefinitionResponse(result), err
}

// Update updates an existing attribute definition.
func (h *attributeDefinitionHandler) Update(ctx context.Context, req *updateAttributeDefinitionRequest) (*attributeDefinitionResponse, error) {
	request, err := toUpdateAttributeDefinitionRequest(req)
	if err != nil || request == nil {
		return nil, apperr.New(apperr.CodeParamInvalid, "attribute definition command is required", nil)
	}
	if err := requireAttributeCommandID(ctx, request.CommandID); err != nil {
		return nil, err
	}
	result, err := h.attrDefService.Update(ctx, request.ID, request)
	return toAttributeDefinitionResponse(result), err
}

func requireAttributeCommandID(ctx context.Context, bodyID uuid.UUID) error {
	headerID, ok := identityCommandID(ctx)
	if !ok || bodyID == uuid.Nil || headerID != bodyID {
		return apperr.New(
			apperr.CodeParamInvalid,
			"one matching canonical idempotency key is required",
			nil,
		)
	}
	return nil
}

func (h *attributeDefinitionHandler) PreviewActivation(
	ctx context.Context,
	req *attributeLifecycleCommandRequest,
) (*attributeLifecyclePreviewResponse, error) {
	return h.previewLifecycle(ctx, req, ports.AttributeLifecycleActivate)
}

func (h *attributeDefinitionHandler) ApplyActivation(
	ctx context.Context,
	req *attributeLifecycleCommandRequest,
) (*attributeLifecycleReceiptResponse, error) {
	return h.applyLifecycle(ctx, req, ports.AttributeLifecycleActivate)
}

func (h *attributeDefinitionHandler) PreviewArchive(
	ctx context.Context,
	req *attributeLifecycleCommandRequest,
) (*attributeLifecyclePreviewResponse, error) {
	return h.previewLifecycle(ctx, req, ports.AttributeLifecycleArchive)
}

func (h *attributeDefinitionHandler) ApplyArchive(
	ctx context.Context,
	req *attributeLifecycleCommandRequest,
) (*attributeLifecycleReceiptResponse, error) {
	return h.applyLifecycle(ctx, req, ports.AttributeLifecycleArchive)
}

func (h *attributeDefinitionHandler) previewLifecycle(
	ctx context.Context,
	req *attributeLifecycleCommandRequest,
	action ports.AttributeLifecycleAction,
) (*attributeLifecyclePreviewResponse, error) {
	if h == nil || h.workflow == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "attribute lifecycle unavailable", nil)
	}
	command, err := toAttributeLifecycleCommandRequest(req)
	if err != nil {
		return nil, err
	}
	request, err := attributeLifecycleWorkflowRequest(ctx, command, action)
	if err != nil {
		return nil, err
	}
	result, err := h.workflow.Preview(ctx, request)
	return toAttributeLifecyclePreviewResponse(result), mapAttributeAdminError(err)
}

func (h *attributeDefinitionHandler) applyLifecycle(
	ctx context.Context,
	req *attributeLifecycleCommandRequest,
	action ports.AttributeLifecycleAction,
) (*attributeLifecycleReceiptResponse, error) {
	if h == nil || h.workflow == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "attribute lifecycle unavailable", nil)
	}
	command, err := toAttributeLifecycleCommandRequest(req)
	if err != nil {
		return nil, err
	}
	request, err := attributeLifecycleWorkflowRequest(ctx, command, action)
	if err != nil {
		return nil, err
	}
	result, err := h.workflow.Apply(ctx, request)
	return toAttributeLifecycleReceiptResponse(result), mapAttributeAdminError(err)
}

func attributeLifecycleWorkflowRequest(
	ctx context.Context,
	req *dto.AttributeLifecycleCommandRequest,
	action ports.AttributeLifecycleAction,
) (attributeadmin.Request, error) {
	if req == nil || !action.Valid() {
		return attributeadmin.Request{}, apperr.New(
			apperr.CodeParamInvalid,
			"invalid attribute lifecycle command",
			attributeadmin.ErrInvalidWorkflow,
		)
	}
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !actorOK || !sessionOK {
		return attributeadmin.Request{}, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	if err := requireAttributeCommandID(ctx, req.CommandID); err != nil {
		return attributeadmin.Request{}, err
	}
	if action == ports.AttributeLifecycleArchive && req.CandidateRevisionID != uuid.Nil {
		return attributeadmin.Request{}, apperr.New(
			apperr.CodeParamInvalid,
			"archive command must not select a revision",
			attributeadmin.ErrInvalidWorkflow,
		)
	}
	return attributeadmin.Request{
		Action:              action,
		DefinitionID:        req.ID,
		CandidateRevisionID: req.CandidateRevisionID,
		CommandID:           req.CommandID,
		ExpectedVersion:     req.ExpectedVersion,
		Reason:              req.Reason,
		ReviewHash:          req.ReviewedPreviewHash,
		ConfirmationToken:   req.ConfirmationToken,
		ActorID:             actorID,
		SessionID:           sessionID,
	}, nil
}

func mapAttributeAdminError(err error) error {
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
			"attribute confirmation is stale",
			err,
		).WithDetails(map[string]any{"reason": "confirmation_stale"})
	case errors.Is(err, admincommand.ErrInvalidCommandMeta),
		errors.Is(err, admincommand.ErrInvalidConfirmation),
		errors.Is(err, attributeadmin.ErrInvalidWorkflow):
		return apperr.New(
			apperr.CodeParamInvalid,
			"invalid attribute lifecycle command",
			err,
		)
	default:
		return err
	}
}
