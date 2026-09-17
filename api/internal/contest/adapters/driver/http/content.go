package http

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/response"
	"github.com/huynhanx03/go-common/pkg/common/http/validation"

	"github.com/huynhanx03/judgify/internal/authctx"
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type ContentHandler interface {
	ListPublicAnnouncements(
		context.Context,
		*contestContentListRequest,
	) (*announcementPageResponse, error)
	ListParticipantAnnouncements(
		context.Context,
		*contestContentListRequest,
	) (*announcementPageResponse, error)
	ListOwnClarifications(
		context.Context,
		*contestContentListRequest,
	) (*clarificationPageResponse, error)
	GetOwnClarification(
		context.Context,
		*contestContentIDRequest,
	) (*clarificationResponse, error)
	ListParticipantClarifications(
		context.Context,
		*contestContentListRequest,
	) (*clarificationPageResponse, error)
	ListAdminAnnouncements(
		context.Context,
		*contestContentListRequest,
	) (*announcementPageResponse, error)
	GetAdminAnnouncement(
		context.Context,
		*contestContentIDRequest,
	) (*announcementResponse, error)
	ListAdminClarifications(
		context.Context,
		*contestContentListRequest,
	) (*clarificationPageResponse, error)
	GetAdminClarification(
		context.Context,
		*contestContentIDRequest,
	) (*clarificationResponse, error)
	ListClarificationRevisions(
		context.Context,
		*contestContentIDRequest,
	) ([]contentRevisionResponse, error)
	ListAnnouncementRevisions(
		context.Context,
		*contestContentIDRequest,
	) ([]contentRevisionResponse, error)

	CreateAnnouncement(*gin.Context)
	EditAnnouncement(*gin.Context)
	PublishAnnouncement(*gin.Context)
	WithdrawAnnouncement(*gin.Context)
	SubmitClarification(*gin.Context)
	AnswerClarification(*gin.Context)
	CloseClarification(*gin.Context)
	WithdrawClarification(*gin.Context)
}

type contentHandler struct {
	content ports.ContentService
}

func NewContentHandler(content ports.ContentService) ContentHandler {
	return &contentHandler{content: content}
}

func (handler *contentHandler) ListPublicAnnouncements(
	ctx context.Context,
	request *contestContentListRequest,
) (*announcementPageResponse, error) {
	result, err := handler.content.ListPublicAnnouncements(ctx, toContestContentListRequest(request))
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toAnnouncementPageResponse(result), nil
}

func (handler *contentHandler) ListParticipantAnnouncements(
	ctx context.Context,
	request *contestContentListRequest,
) (*announcementPageResponse, error) {
	actorID, err := contentRequestActor(ctx)
	if err != nil {
		return nil, err
	}
	result, err := handler.content.ListParticipantAnnouncements(
		ctx,
		actorID,
		toContestContentListRequest(request),
	)
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toAnnouncementPageResponse(result), nil
}

func (handler *contentHandler) ListOwnClarifications(
	ctx context.Context,
	request *contestContentListRequest,
) (*clarificationPageResponse, error) {
	actorID, err := contentRequestActor(ctx)
	if err != nil {
		return nil, err
	}
	result, err := handler.content.ListOwnClarifications(ctx, actorID, toContestContentListRequest(request))
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toClarificationPageResponse(result), nil
}

func (handler *contentHandler) GetOwnClarification(
	ctx context.Context,
	request *contestContentIDRequest,
) (*clarificationResponse, error) {
	actorID, err := contentRequestActor(ctx)
	if err != nil {
		return nil, err
	}
	result, err := handler.content.GetOwnClarification(
		ctx,
		request.ContestID,
		request.ContentID,
		actorID,
	)
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toClarificationResponse(result), nil
}

func (handler *contentHandler) ListParticipantClarifications(
	ctx context.Context,
	request *contestContentListRequest,
) (*clarificationPageResponse, error) {
	actorID, err := contentRequestActor(ctx)
	if err != nil {
		return nil, err
	}
	result, err := handler.content.ListParticipantClarifications(
		ctx,
		actorID,
		toContestContentListRequest(request),
	)
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toClarificationPageResponse(result), nil
}

func (handler *contentHandler) ListAdminAnnouncements(
	ctx context.Context,
	request *contestContentListRequest,
) (*announcementPageResponse, error) {
	result, err := handler.content.ListAdminAnnouncements(ctx, toContestContentListRequest(request))
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toAnnouncementPageResponse(result), nil
}

func (handler *contentHandler) GetAdminAnnouncement(
	ctx context.Context,
	request *contestContentIDRequest,
) (*announcementResponse, error) {
	result, err := handler.content.GetAdminAnnouncement(
		ctx,
		request.ContestID,
		request.ContentID,
	)
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toAnnouncementResponse(result), nil
}

func (handler *contentHandler) ListAdminClarifications(
	ctx context.Context,
	request *contestContentListRequest,
) (*clarificationPageResponse, error) {
	result, err := handler.content.ListAdminClarifications(ctx, toContestContentListRequest(request))
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toClarificationPageResponse(result), nil
}

func (handler *contentHandler) GetAdminClarification(
	ctx context.Context,
	request *contestContentIDRequest,
) (*clarificationResponse, error) {
	result, err := handler.content.GetAdminClarification(
		ctx,
		request.ContestID,
		request.ContentID,
	)
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toClarificationResponse(result), nil
}

func (handler *contentHandler) ListClarificationRevisions(
	ctx context.Context,
	request *contestContentIDRequest,
) ([]contentRevisionResponse, error) {
	if _, err := handler.content.GetAdminClarification(
		ctx,
		request.ContestID,
		request.ContentID,
	); err != nil {
		return nil, mapContestContentError(err)
	}
	result, err := handler.content.ListRevisions(
		ctx,
		entity.ContentOwnerClarification,
		request.ContentID,
	)
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toContentRevisionResponses(result), nil
}

func (handler *contentHandler) ListAnnouncementRevisions(
	ctx context.Context,
	request *contestContentIDRequest,
) ([]contentRevisionResponse, error) {
	if _, err := handler.content.GetAdminAnnouncement(
		ctx,
		request.ContestID,
		request.ContentID,
	); err != nil {
		return nil, mapContestContentError(err)
	}
	result, err := handler.content.ListRevisions(
		ctx,
		entity.ContentOwnerAnnouncement,
		request.ContentID,
	)
	if err != nil {
		return nil, mapContestContentError(err)
	}
	return toContentRevisionResponses(result), nil
}

func (handler *contentHandler) CreateAnnouncement(ctx *gin.Context) {
	request, ok := parseContestContentRequest[createAnnouncementRequest](ctx)
	if !ok {
		return
	}
	actorID, err := contentRequestActor(ctx.Request.Context())
	if err == nil {
		err = parseContentContestID(ctx, &request.ContestID)
	}
	if err == nil {
		request.CommandID, err = contentCommandID(ctx)
	}
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	if !validateContestContentRequest(ctx, *request) {
		return
	}
	result, err := handler.content.CreateAnnouncement(
		ctx.Request.Context(),
		request.ContestID,
		actorID,
		toCreateAnnouncementRequest(request),
	)
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	respondContestContent(ctx, http.StatusCreated, result.Version, toAnnouncementResponse(result))
}

func (handler *contentHandler) EditAnnouncement(ctx *gin.Context) {
	request, ok := parseContestContentRequest[editAnnouncementRequest](ctx)
	if !ok {
		return
	}
	actorID, err := contentRequestActor(ctx.Request.Context())
	if err == nil {
		err = parseContentIDs(
			ctx,
			&request.ContestID,
			&request.AnnouncementID,
		)
	}
	if err == nil {
		request.CommandID, err = contentCommandID(ctx)
	}
	if err == nil {
		request.ExpectedVersion, err = contentExpectedVersion(ctx)
	}
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	if !validateContestContentRequest(ctx, *request) {
		return
	}
	result, err := handler.content.EditAnnouncement(
		ctx.Request.Context(),
		request.ContestID,
		actorID,
		toEditAnnouncementRequest(request),
	)
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	respondContestContent(ctx, http.StatusOK, result.Version, toAnnouncementResponse(result))
}

func (handler *contentHandler) PublishAnnouncement(ctx *gin.Context) {
	handler.announcementCommand(ctx, true)
}

func (handler *contentHandler) WithdrawAnnouncement(ctx *gin.Context) {
	handler.announcementCommand(ctx, false)
}

func (handler *contentHandler) announcementCommand(
	ctx *gin.Context,
	publish bool,
) {
	request, ok := parseContestContentRequest[announcementCommandRequest](ctx)
	if !ok {
		return
	}
	actorID, err := contentRequestActor(ctx.Request.Context())
	if err == nil {
		err = parseContentIDs(
			ctx,
			&request.ContestID,
			&request.AnnouncementID,
		)
	}
	if err == nil {
		request.CommandID, err = contentCommandID(ctx)
	}
	if err == nil {
		request.ExpectedVersion, err = contentExpectedVersion(ctx)
	}
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	if !validateContestContentRequest(ctx, *request) {
		return
	}

	var result *contract.AnnouncementResponse
	if publish {
		result, err = handler.content.PublishAnnouncement(
			ctx.Request.Context(),
			request.ContestID,
			actorID,
			toAnnouncementCommandRequest(request),
		)
	} else {
		result, err = handler.content.WithdrawAnnouncement(
			ctx.Request.Context(),
			request.ContestID,
			actorID,
			toAnnouncementCommandRequest(request),
		)
	}
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	respondContestContent(ctx, http.StatusOK, result.Version, toAnnouncementResponse(result))
}

func (handler *contentHandler) SubmitClarification(ctx *gin.Context) {
	request, ok := parseContestContentRequest[submitClarificationRequest](ctx)
	if !ok {
		return
	}
	actorID, err := contentRequestActor(ctx.Request.Context())
	if err == nil {
		err = parseContentContestID(ctx, &request.ContestID)
	}
	if err == nil {
		request.CommandID, err = contentCommandID(ctx)
	}
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	if !validateContestContentRequest(ctx, *request) {
		return
	}
	result, err := handler.content.SubmitClarification(
		ctx.Request.Context(),
		request.ContestID,
		actorID,
		toSubmitClarificationRequest(request),
	)
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	respondContestContent(ctx, http.StatusCreated, result.Version, toClarificationResponse(result))
}

func (handler *contentHandler) AnswerClarification(ctx *gin.Context) {
	request, ok := parseContestContentRequest[answerClarificationRequest](ctx)
	if !ok {
		return
	}
	actorID, err := contentRequestActor(ctx.Request.Context())
	if err == nil {
		err = parseContentIDs(
			ctx,
			&request.ContestID,
			&request.ClarificationID,
		)
	}
	if err == nil {
		request.CommandID, err = contentCommandID(ctx)
	}
	if err == nil {
		request.ExpectedVersion, err = contentExpectedVersion(ctx)
	}
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	if !validateContestContentRequest(ctx, *request) {
		return
	}
	result, err := handler.content.AnswerClarification(
		ctx.Request.Context(),
		request.ContestID,
		actorID,
		toAnswerClarificationRequest(request),
	)
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	respondContestContent(ctx, http.StatusOK, result.Version, toClarificationResponse(result))
}

func (handler *contentHandler) CloseClarification(ctx *gin.Context) {
	handler.clarificationCommand(ctx, true)
}

func (handler *contentHandler) WithdrawClarification(ctx *gin.Context) {
	handler.clarificationCommand(ctx, false)
}

func (handler *contentHandler) clarificationCommand(
	ctx *gin.Context,
	closeClarification bool,
) {
	request, ok := parseContestContentRequest[clarificationCommandRequest](ctx)
	if !ok {
		return
	}
	actorID, err := contentRequestActor(ctx.Request.Context())
	if err == nil {
		err = parseContentIDs(
			ctx,
			&request.ContestID,
			&request.ClarificationID,
		)
	}
	if err == nil {
		request.CommandID, err = contentCommandID(ctx)
	}
	if err == nil {
		request.ExpectedVersion, err = contentExpectedVersion(ctx)
	}
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	if !validateContestContentRequest(ctx, *request) {
		return
	}

	var result *contract.ClarificationResponse
	if closeClarification {
		result, err = handler.content.CloseClarification(
			ctx.Request.Context(),
			request.ContestID,
			actorID,
			toClarificationCommandRequest(request),
		)
	} else {
		result, err = handler.content.WithdrawClarification(
			ctx.Request.Context(),
			request.ContestID,
			actorID,
			toClarificationCommandRequest(request),
		)
	}
	if err != nil {
		failContestContent(ctx, err)
		return
	}
	respondContestContent(ctx, http.StatusOK, result.Version, toClarificationResponse(result))
}

func parseContestContentRequest[T any](
	ctx *gin.Context,
) (*T, bool) {
	var value T
	if err := ctx.ShouldBindJSON(&value); err != nil {
		var maximumBytesError *http.MaxBytesError
		if errors.As(err, &maximumBytesError) {
			failContestContent(
				ctx,
				apperr.New(
					apperr.CodeBodyTooLarge,
					"request body too large",
					err,
				),
			)
			return nil, false
		}
		if !errors.Is(err, io.EOF) {
			failContestContent(
				ctx,
				apperr.New(
					apperr.CodeParamInvalid,
					"invalid request body",
					err,
				),
			)
			return nil, false
		}
	}
	return &value, true
}

func validateContestContentRequest(
	ctx *gin.Context,
	value any,
) bool {
	violations := validation.Validate(value)
	if len(violations) > 0 {
		failContestContent(
			ctx,
			apperr.New(
				apperr.CodeValidationFailed,
				violations[0].Message,
				nil,
			).WithDetails(map[string]any{"errors": violations}),
		)
		return false
	}
	return true
}

func contentRequestActor(ctx context.Context) (uuid.UUID, error) {
	id, ok := authctx.UserID(ctx)
	if !ok || id == uuid.Nil {
		return uuid.Nil, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	return id, nil
}

func parseContentContestID(
	ctx *gin.Context,
	destination *uuid.UUID,
) error {
	if destination == nil {
		return ports.ErrInvalidContestContentCommand
	}
	parsed, err := parseCanonicalContentUUID(ctx.Param("id"))
	if err != nil {
		return err
	}
	*destination = parsed
	return nil
}

func parseContentIDs(
	ctx *gin.Context,
	contestID *uuid.UUID,
	contentID *uuid.UUID,
) error {
	if err := parseContentContestID(ctx, contestID); err != nil {
		return err
	}
	if contentID == nil {
		return ports.ErrInvalidContestContentCommand
	}
	parsed, err := parseCanonicalContentUUID(ctx.Param("content_id"))
	if err != nil {
		return err
	}
	*contentID = parsed
	return nil
}

func parseCanonicalContentUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil ||
		parsed == uuid.Nil ||
		parsed.String() != value {
		return uuid.Nil, ports.ErrInvalidContestContentCommand
	}
	return parsed, nil
}

func contentCommandID(ctx *gin.Context) (uuid.UUID, error) {
	values := ctx.Request.Header.Values("Idempotency-Key")
	if len(values) != 1 {
		return uuid.Nil, ports.ErrInvalidContestContentCommand
	}
	return parseCanonicalContentUUID(values[0])
}

func contentExpectedVersion(ctx *gin.Context) (int64, error) {
	values := ctx.Request.Header.Values("If-Match")
	if len(values) != 1 {
		return 0, ports.ErrInvalidContestContentCommand
	}
	version, err := parseContestVersionETag(values[0])
	if err != nil {
		return 0, ports.ErrInvalidContestContentCommand
	}
	return version, nil
}

func contestContentNoStore(ctx *gin.Context) {
	ctx.Header("Cache-Control", "private, no-store")
	ctx.Header("Pragma", "no-cache")
	ctx.Header("Expires", "0")
}

func respondContestContent(
	ctx *gin.Context,
	status int,
	version int64,
	result any,
) {
	contestContentNoStore(ctx)
	ctx.Header("ETag", contestVersionETag(version))
	response.RespondStatus(ctx, status, result)
}

func failContestContent(ctx *gin.Context, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		_ = ctx.Error(err)
		ctx.Abort()
		return
	}
	mapped := mapContestContentError(err)
	_ = ctx.Error(mapped)
	response.ErrorResponse(ctx, apperr.CodeInternalServer, mapped)
	ctx.Abort()
}

func mapContestContentError(err error) error {
	if err == nil {
		return nil
	}
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) {
		return err
	}
	switch {
	case errors.Is(err, ports.ErrInvalidContestContentCommand),
		errors.Is(err, entity.ErrContestContentInvalid):
		return apperr.New(
			apperr.CodeBadRequest,
			"invalid contest content request",
			err,
		)
	case errors.Is(err, entity.ErrContestContentNotFound),
		errors.Is(err, entity.ErrContestNotFound):
		return apperr.New(
			apperr.CodeNotFound,
			"contest content not found",
			err,
		)
	case errors.Is(err, entity.ErrContestContentForbidden),
		errors.Is(err, entity.ErrContestContentRegistration):
		return apperr.New(
			apperr.CodeForbidden,
			"contest content access denied",
			err,
		)
	case errors.Is(err, entity.ErrContestContentVersionConflict),
		errors.Is(err, entity.ErrContestContentStateConflict),
		errors.Is(err, entity.ErrContestContentSchedulePending),
		errors.Is(err, entity.ErrContestContentContestNotRunning),
		errors.Is(err, entity.ErrContestContentNotReady):
		return apperr.New(
			apperr.CodeConflict,
			"contest content state changed",
			err,
		)
	default:
		return apperr.New(
			apperr.CodeInternalServer,
			"contest content request failed",
			err,
		)
	}
}
