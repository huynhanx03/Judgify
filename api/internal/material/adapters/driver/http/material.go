package http

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	"github.com/huynhanx03/judgify/internal/application/workflow/materiallifecycle"
	"github.com/huynhanx03/judgify/internal/authctx"
	contract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

// MaterialHandler defines the material HTTP handler interface.
type MaterialHandler interface {
	FindPublic(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*MaterialResponse], error)
	GetPublic(ctx context.Context, req *materialIDRequest) (*MaterialResponse, error)
	FindAdmin(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*MaterialResponse], error)
	GetAdmin(ctx context.Context, req *materialIDRequest) (*MaterialResponse, error)
	Create(ctx context.Context, req *createMaterialRequest) (*MaterialResponse, error)
	Update(ctx context.Context, req *updateMaterialRequest) (*MaterialResponse, error)
	Delete(ctx context.Context, req *materialIDRequest) (*MaterialResponse, error)
	Search(ctx context.Context, req *materialSearchRequest) (*materialSearchResponse, error)
	SearchAdmin(ctx context.Context, req *adminMaterialSearchRequest) (*adminMaterialSearchResponse, error)
	ResolveSlug(ctx context.Context, req *GetMaterialBySlugRequest) (*materialSlugResponse, error)
	Related(ctx context.Context, req *RelatedMaterialRequest) ([]*MaterialResponse, error)
	ListRevisions(ctx context.Context, req *ListMaterialRevisionsRequest) (*materialRevisionPage, error)
	GetRevision(ctx context.Context, req *GetMaterialRevisionRequest) (*materialRevisionResponse, error)
	RecordView(ctx context.Context, req *RecordMaterialViewRequest) (*RecordMaterialViewResponse, error)
	CreateDraft(ctx context.Context, req *createMaterialRequest) (*materialMutationReceipt, error)
	SaveDraft(ctx context.Context, req *updateMaterialRequest) (*materialMutationReceipt, error)
	PreviewSubmitReview(ctx context.Context, req *archiveMaterialRequest) (*materialLifecyclePreviewResponse, error)
	ApplySubmitReview(ctx context.Context, req *archiveMaterialRequest) (*materialLifecycleReceipt, error)
	PreviewReturnToDraft(ctx context.Context, req *archiveMaterialRequest) (*materialLifecyclePreviewResponse, error)
	ApplyReturnToDraft(ctx context.Context, req *archiveMaterialRequest) (*materialLifecycleReceipt, error)
	PreviewPublish(ctx context.Context, req *publishMaterialRequest) (*materialLifecyclePreviewResponse, error)
	ApplyPublish(ctx context.Context, req *publishMaterialRequest) (*materialLifecycleReceipt, error)
	PreviewArchive(ctx context.Context, req *archiveMaterialRequest) (*materialLifecyclePreviewResponse, error)
	ApplyArchive(ctx context.Context, req *archiveMaterialRequest) (*materialLifecycleReceipt, error)
	PreviewRollback(ctx context.Context, req *rollbackMaterialRequest) (*materialLifecyclePreviewResponse, error)
	ApplyRollback(ctx context.Context, req *rollbackMaterialRequest) (*materialLifecycleReceipt, error)
	Rename(ctx context.Context, req *renameMaterialRequest) (*materialMutationReceipt, error)
}

type materialHandler struct {
	materialService ports.MaterialService
	editorial       ports.MaterialEditorialService
	query           ports.MaterialQueryService
	views           ports.MaterialViewService
	lifecycle       *materiallifecycle.Workflow
}

// NewMaterialHandler creates a new MaterialHandler.
func NewMaterialHandler(materialService ports.MaterialService) MaterialHandler {
	return &materialHandler{materialService: materialService}
}

func NewProductionMaterialHandler(
	legacy ports.MaterialService,
	editorial ports.MaterialEditorialService,
	query ports.MaterialQueryService,
	views ports.MaterialViewService,
	lifecycle *materiallifecycle.Workflow,
) MaterialHandler {
	return &materialHandler{
		materialService: legacy, editorial: editorial, query: query, views: views,
		lifecycle: lifecycle,
	}
}

func (h *materialHandler) FindPublic(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*MaterialResponse], error) {
	result, err := h.materialService.FindPublic(ctx, req)
	return toMaterialPage(result), err
}

func (h *materialHandler) GetPublic(ctx context.Context, req *materialIDRequest) (*MaterialResponse, error) {
	id, err := toMaterialID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.materialService.GetPublic(ctx, id)
	return toMaterialResponse(result), err
}

func (h *materialHandler) FindAdmin(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*MaterialResponse], error) {
	result, err := h.materialService.FindAdmin(ctx, req)
	return toMaterialPage(result), err
}

func (h *materialHandler) GetAdmin(ctx context.Context, req *materialIDRequest) (*MaterialResponse, error) {
	id, err := toMaterialID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.query.GetAdmin(ctx, id)
	return toMaterialResponse(result), err
}

func (h *materialHandler) Create(ctx context.Context, req *createMaterialRequest) (*MaterialResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil)
	}
	command, err := toCreateMaterialCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.materialService.Create(ctx, userID, command)
	return toMaterialResponse(result), err
}

func (h *materialHandler) Update(ctx context.Context, req *updateMaterialRequest) (*MaterialResponse, error) {
	id, command, err := toUpdateMaterialCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.materialService.Update(ctx, id, command)
	return toMaterialResponse(result), err
}

func (h *materialHandler) Delete(ctx context.Context, req *materialIDRequest) (*MaterialResponse, error) {
	id, err := toMaterialID(req)
	if err != nil {
		return nil, err
	}
	if err := h.materialService.Delete(ctx, id); err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *materialHandler) Search(ctx context.Context, req *materialSearchRequest) (*materialSearchResponse, error) {
	_, authenticated := authctx.UserID(ctx)
	query, err := toMaterialSearchQuery(req, authenticated)
	if err != nil {
		return nil, err
	}
	result, err := h.query.Search(ctx, query)
	return toMaterialSearchResponse(result), err
}

func (h *materialHandler) SearchAdmin(ctx context.Context, req *adminMaterialSearchRequest) (*adminMaterialSearchResponse, error) {
	query, err := toAdminMaterialSearchQuery(req)
	if err != nil {
		return nil, err
	}
	result, err := h.query.SearchAdmin(ctx, query)
	return toAdminMaterialSearchResponse(result), err
}

func (h *materialHandler) ResolveSlug(ctx context.Context, req *GetMaterialBySlugRequest) (*materialSlugResponse, error) {
	_, authenticated := authctx.UserID(ctx)
	result, err := h.query.Resolve(ctx, req.Slug, authenticated)
	return toMaterialSlugResponse(result), err
}

func (h *materialHandler) Related(ctx context.Context, req *RelatedMaterialRequest) ([]*MaterialResponse, error) {
	_, authenticated := authctx.UserID(ctx)
	result, err := h.query.Related(ctx, req.Slug, authenticated, req.Limit)
	return toMaterialResponses(result), err
}

func (h *materialHandler) ListRevisions(ctx context.Context, req *ListMaterialRevisionsRequest) (*materialRevisionPage, error) {
	id, err := parseMaterialID(req.ID)
	if err != nil {
		return nil, err
	}
	result, err := h.query.ListRevisions(ctx, id, req.Cursor, req.Limit)
	return toMaterialRevisionPage(result), err
}

func (h *materialHandler) GetRevision(ctx context.Context, req *GetMaterialRevisionRequest) (*materialRevisionResponse, error) {
	id, revisionID, err := toMaterialRevisionIDs(req)
	if err != nil {
		return nil, err
	}
	result, err := h.query.GetRevision(ctx, id, revisionID)
	return toMaterialRevisionResponse(result), err
}

func (h *materialHandler) RecordView(ctx context.Context, req *RecordMaterialViewRequest) (*RecordMaterialViewResponse, error) {
	id, err := toMaterialViewID(req)
	if err != nil {
		return nil, err
	}
	counted, err := h.views.Record(ctx, id)
	if err != nil {
		return nil, err
	}
	return &RecordMaterialViewResponse{Counted: counted}, nil
}

func (h *materialHandler) CreateDraft(ctx context.Context, req *createMaterialRequest) (*materialMutationReceipt, error) {
	authorID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil)
	}
	command, err := toCreateMaterialCommand(req)
	if err != nil {
		return nil, err
	}
	if err := matchingMaterialCommandID(ctx, command.CommandID); err != nil {
		return nil, err
	}
	materialID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	visibility := entity.MaterialVisibility(command.Visibility)
	if visibility == "" {
		visibility = entity.MaterialPublic
	}
	response, err := h.editorial.CreateDraft(ctx, ports.CreateDraftCommand{
		CommandID: command.CommandID, MaterialID: materialID, Slug: command.Slug, Title: command.Title,
		Description: command.Description, Content: []byte(command.Content),
		CategoryID: command.CategoryID, DifficultyID: command.DifficultyID,
		TagIDs: command.TagIDs, AuthorID: authorID, Visibility: visibility,
		ChangeSummary: command.ChangeSummary,
	})
	if err == nil {
		logMaterialMutationReceipt(ctx, response)
	}
	return toMaterialMutationReceipt(response), mapMaterialMutationError(err)
}

func (h *materialHandler) SaveDraft(ctx context.Context, req *updateMaterialRequest) (*materialMutationReceipt, error) {
	materialID, command, err := toUpdateMaterialCommand(req)
	if err != nil {
		return nil, err
	}
	authorID, ok := authctx.UserID(ctx)
	if !ok || command.Title == nil || command.Description == nil || command.Content == nil ||
		command.CategoryID == nil || command.DifficultyID == nil || command.TagIDs == nil {
		return nil, entity.ErrMaterialInvalidCommand
	}
	if err := matchingMaterialCommandID(ctx, command.CommandID); err != nil {
		return nil, err
	}
	visibility := entity.MaterialPublic
	if command.Visibility != nil {
		visibility = entity.MaterialVisibility(*command.Visibility)
	}
	response, err := h.editorial.SaveDraft(ctx, ports.MaterialSaveDraftCommand{
		CommandID: command.CommandID, MaterialID: materialID, Title: *command.Title, Description: *command.Description,
		Content: []byte(*command.Content), CategoryID: *command.CategoryID,
		DifficultyID: *command.DifficultyID, TagIDs: *command.TagIDs,
		AuthorID: authorID, Visibility: visibility,
		ExpectedVersion: command.ExpectedVersion, ChangeSummary: command.ChangeSummary,
	})
	if err == nil {
		logMaterialMutationReceipt(ctx, response)
	}
	return toMaterialMutationReceipt(response), mapMaterialMutationError(err)
}

func (h *materialHandler) PreviewPublish(
	ctx context.Context,
	req *publishMaterialRequest,
) (*materialLifecyclePreviewResponse, error) {
	request, err := toPublishMaterialLifecycleRequest(req)
	if err != nil {
		return nil, err
	}
	request.ConfirmationToken = ""
	return h.previewLifecycle(ctx, request)
}

func (h *materialHandler) PreviewSubmitReview(ctx context.Context, req *archiveMaterialRequest) (*materialLifecyclePreviewResponse, error) {
	request, err := toArchiveMaterialLifecycleRequest(ports.MaterialLifecycleSubmitReview, req)
	if err != nil {
		return nil, err
	}
	request.ConfirmationToken = ""
	return h.previewLifecycle(ctx, request)
}

func (h *materialHandler) ApplySubmitReview(ctx context.Context, req *archiveMaterialRequest) (*materialLifecycleReceipt, error) {
	request, err := toArchiveMaterialLifecycleRequest(ports.MaterialLifecycleSubmitReview, req)
	if err != nil {
		return nil, err
	}
	return h.applyLifecycle(ctx, request)
}

func (h *materialHandler) PreviewReturnToDraft(ctx context.Context, req *archiveMaterialRequest) (*materialLifecyclePreviewResponse, error) {
	request, err := toArchiveMaterialLifecycleRequest(ports.MaterialLifecycleReturnToDraft, req)
	if err != nil {
		return nil, err
	}
	request.ConfirmationToken = ""
	return h.previewLifecycle(ctx, request)
}

func (h *materialHandler) ApplyReturnToDraft(ctx context.Context, req *archiveMaterialRequest) (*materialLifecycleReceipt, error) {
	request, err := toArchiveMaterialLifecycleRequest(ports.MaterialLifecycleReturnToDraft, req)
	if err != nil {
		return nil, err
	}
	return h.applyLifecycle(ctx, request)
}

func (h *materialHandler) ApplyPublish(
	ctx context.Context,
	req *publishMaterialRequest,
) (*materialLifecycleReceipt, error) {
	request, err := toPublishMaterialLifecycleRequest(req)
	if err != nil {
		return nil, err
	}
	return h.applyLifecycle(ctx, request)
}

func (h *materialHandler) PreviewArchive(
	ctx context.Context,
	req *archiveMaterialRequest,
) (*materialLifecyclePreviewResponse, error) {
	request, err := toArchiveMaterialLifecycleRequest(ports.MaterialLifecycleArchive, req)
	if err != nil {
		return nil, err
	}
	request.ConfirmationToken = ""
	return h.previewLifecycle(ctx, request)
}

func (h *materialHandler) ApplyArchive(
	ctx context.Context,
	req *archiveMaterialRequest,
) (*materialLifecycleReceipt, error) {
	request, err := toArchiveMaterialLifecycleRequest(ports.MaterialLifecycleArchive, req)
	if err != nil {
		return nil, err
	}
	return h.applyLifecycle(ctx, request)
}

func (h *materialHandler) PreviewRollback(
	ctx context.Context,
	req *rollbackMaterialRequest,
) (*materialLifecyclePreviewResponse, error) {
	request, err := toRollbackMaterialLifecycleRequest(req)
	if err != nil {
		return nil, err
	}
	request.ConfirmationToken = ""
	return h.previewLifecycle(ctx, request)
}

func (h *materialHandler) ApplyRollback(
	ctx context.Context,
	req *rollbackMaterialRequest,
) (*materialLifecycleReceipt, error) {
	request, err := toRollbackMaterialLifecycleRequest(req)
	if err != nil {
		return nil, err
	}
	return h.applyLifecycle(ctx, request)
}

func lifecycleRequest(
	action ports.MaterialLifecycleAction,
	materialID uuid.UUID,
	targetRevisionID uuid.UUID,
	commandID uuid.UUID,
	expectedVersion int64,
	reason string,
	confirmation string,
) materiallifecycle.Request {
	return materiallifecycle.Request{
		Action:            action,
		MaterialID:        materialID,
		TargetRevisionID:  targetRevisionID,
		CommandID:         commandID,
		ExpectedVersion:   expectedVersion,
		Reason:            reason,
		ConfirmationToken: confirmation,
	}
}

func (h *materialHandler) previewLifecycle(
	ctx context.Context,
	request materiallifecycle.Request,
) (*materialLifecyclePreviewResponse, error) {
	request, err := materialLifecyclePrincipal(ctx, request)
	if err != nil {
		return nil, err
	}
	if err := matchingMaterialCommandID(ctx, request.CommandID); err != nil {
		return nil, err
	}
	response, err := h.lifecycle.Preview(ctx, request)
	if err == nil && response != nil {
		logger.FromContext(ctx).Debug(
			"material lifecycle review issued",
			zap.String("material_id", response.MaterialID.String()),
			zap.String("action", response.Action),
			zap.Int64("expected_version", response.ExpectedVersion),
		)
	}
	return toMaterialLifecyclePreviewResponse(response), mapMaterialLifecycleError(err)
}

func (h *materialHandler) applyLifecycle(
	ctx context.Context,
	request materiallifecycle.Request,
) (*materialLifecycleReceipt, error) {
	request, err := materialLifecyclePrincipal(ctx, request)
	if err != nil {
		return nil, err
	}
	if err := matchingMaterialCommandID(ctx, request.CommandID); err != nil {
		return nil, err
	}
	response, err := h.lifecycle.Apply(ctx, request)
	if err == nil {
		logMaterialLifecycleReceipt(ctx, response)
	}
	return toMaterialLifecycleReceipt(response), mapMaterialLifecycleError(err)
}

func materialLifecyclePrincipal(
	ctx context.Context,
	request materiallifecycle.Request,
) (materiallifecycle.Request, error) {
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !actorOK || !sessionOK {
		return materiallifecycle.Request{}, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	request.ActorID = actorID
	request.SessionID = sessionID
	return request, nil
}

func matchingMaterialCommandID(ctx context.Context, body uuid.UUID) error {
	header, ok := ctx.Value(materialCommandIDContextKey{}).(uuid.UUID)
	if !ok || header == uuid.Nil || header != body {
		return apperr.New(
			apperr.CodeParamInvalid,
			"one matching canonical idempotency key is required",
			entity.ErrMaterialInvalidCommand,
		)
	}
	return nil
}

func mapMaterialLifecycleError(err error) error {
	if err == nil {
		return nil
	}
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) {
		return err
	}
	switch {
	case errors.Is(err, entity.ErrMaterialCommandConflict):
		return apperr.New(
			apperr.CodeConflict,
			"material command identity was already used for another request",
			err,
		).WithDetails(map[string]any{"reason": "idempotency_conflict"})
	case errors.Is(err, admincommand.ErrRecentAuthenticationRequired):
		return apperr.New(
			apperr.CodeForbidden,
			"recent authentication required",
			err,
		).WithDetails(map[string]any{"reason": "recent_authentication_required"})
	case errors.Is(err, admincommand.ErrConfirmationExpired):
		return apperr.New(
			apperr.CodeConflict,
			"material review expired",
			err,
		).WithDetails(map[string]any{"reason": "confirmation_expired"})
	case errors.Is(err, admincommand.ErrConfirmationMismatch):
		return apperr.New(
			apperr.CodeConflict,
			"material review no longer matches this command",
			err,
		).WithDetails(map[string]any{"reason": "confirmation_mismatch"})
	case errors.Is(err, entity.ErrMaterialConflict),
		errors.Is(err, entity.ErrMaterialInvalidLifecycle),
		errors.Is(err, entity.ErrMaterialArtifactRejected):
		return apperr.New(
			apperr.CodeConflict,
			"material changed; create a new review",
			err,
		).WithDetails(map[string]any{"reason": "material_changed"})
	case errors.Is(err, entity.ErrMaterialNotFound):
		return apperr.New(apperr.CodeNotFound, "material not found", err)
	case errors.Is(err, admincommand.ErrInvalidConfirmation),
		errors.Is(err, admincommand.ErrInvalidCommandMeta),
		errors.Is(err, materiallifecycle.ErrInvalidWorkflow),
		errors.Is(err, entity.ErrMaterialInvalidCommand):
		return apperr.New(
			apperr.CodeParamInvalid,
			"invalid material lifecycle command",
			err,
		)
	default:
		return err
	}
}

func mapMaterialMutationError(err error) error {
	if err == nil {
		return nil
	}
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) {
		return err
	}
	switch {
	case errors.Is(err, entity.ErrMaterialCommandConflict):
		return apperr.New(
			apperr.CodeConflict,
			"material command identity was already used for another request",
			err,
		).WithDetails(map[string]any{"reason": "idempotency_conflict"})
	case errors.Is(err, entity.ErrMaterialSlugConflict):
		return apperr.New(
			apperr.CodeConflict,
			"material slug is unavailable",
			err,
		).WithDetails(map[string]any{"reason": "slug_unavailable"})
	case errors.Is(err, entity.ErrMaterialConflict),
		errors.Is(err, entity.ErrMaterialArtifactRejected):
		return apperr.New(
			apperr.CodeConflict,
			"material command conflicts with current state",
			err,
		).WithDetails(map[string]any{"reason": "material_changed"})
	case errors.Is(err, entity.ErrMaterialNotFound):
		return apperr.New(apperr.CodeNotFound, "material not found", err)
	case errors.Is(err, entity.ErrMaterialInvalidCommand):
		return apperr.New(
			apperr.CodeParamInvalid,
			"invalid material command",
			err,
		)
	default:
		return err
	}
}

func (h *materialHandler) Rename(ctx context.Context, req *renameMaterialRequest) (*materialMutationReceipt, error) {
	actorID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil)
	}
	command, err := toRenameMaterialCommand(req, actorID)
	if err != nil {
		return nil, err
	}
	if err := matchingMaterialCommandID(ctx, command.CommandID); err != nil {
		return nil, err
	}
	response, err := h.editorial.Rename(ctx, command)
	if err == nil {
		logMaterialMutationReceipt(ctx, response)
	}
	return toMaterialMutationReceipt(response), mapMaterialMutationError(err)
}

func logMaterialMutationReceipt(
	ctx context.Context,
	receipt *contract.MaterialMutationReceipt,
) {
	if receipt == nil {
		return
	}
	logger.FromContext(ctx).Info(
		"material mutation committed",
		zap.String("material_id", receipt.MaterialID.String()),
		zap.String("action", receipt.Action),
		zap.Int64("version", receipt.Version),
		zap.Bool("idempotent_replay", receipt.IdempotentReplay),
	)
}

func logMaterialLifecycleReceipt(
	ctx context.Context,
	receipt *contract.MaterialLifecycleReceipt,
) {
	if receipt == nil {
		return
	}
	logger.FromContext(ctx).Info(
		"material lifecycle command committed",
		zap.String("material_id", receipt.MaterialID.String()),
		zap.String("action", receipt.Action),
		zap.Int64("version", receipt.Version),
		zap.Bool("idempotent_replay", receipt.IdempotentReplay),
	)
}
