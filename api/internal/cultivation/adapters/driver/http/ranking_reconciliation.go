package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/authctx"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	service "github.com/huynhanx03/judgify/internal/cultivation/usecase"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	"github.com/huynhanx03/judgify/internal/ports"
)

type RankingReconciliationHandler interface {
	Status(context.Context) (*rankingReconciliationStatusResponse, error)
	Preview(*gin.Context)
	Start(*gin.Context)
}

type rankingReconciliationHandler struct {
	service ports.RankingReconciliationService
}

func NewRankingReconciliationHandler(
	service ports.RankingReconciliationService,
) RankingReconciliationHandler {
	return &rankingReconciliationHandler{service: service}
}

func (handler *rankingReconciliationHandler) Status(
	ctx context.Context,
) (*rankingReconciliationStatusResponse, error) {
	response, err := handler.service.Status(ctx)
	if err != nil {
		return nil, err
	}
	return toRankingReconciliationStatusResponse(response), nil
}

func (handler *rankingReconciliationHandler) Preview(c *gin.Context) {
	if handler == nil || handler.service == nil || c == nil || c.Request == nil {
		failRankingReconciliation(c, entity.ErrInvalidRankingReconciliation)
		return
	}
	actorUserID, userOK := authctx.UserID(c.Request.Context())
	actorSessionID, sessionOK := authctx.SessionID(c.Request.Context())
	if !userOK || !sessionOK {
		failRankingReconciliation(
			c,
			apperr.New(apperr.CodeUnauthorized, "authentication required", nil),
		)
		return
	}
	result, err := handler.service.Preview(
		c.Request.Context(),
		entity.RankingReconciliationPreviewCommand{
			ActorUserID:    actorUserID,
			ActorSessionID: actorSessionID,
		},
	)
	if err != nil {
		failRankingReconciliation(c, err)
		return
	}
	response.Respond(c, toRankingReconciliationPreviewResponse(result))
}

func (handler *rankingReconciliationHandler) Start(c *gin.Context) {
	if handler == nil || handler.service == nil || c == nil || c.Request == nil {
		failRankingReconciliation(c, entity.ErrInvalidRankingReconciliation)
		return
	}
	actorUserID, userOK := authctx.UserID(c.Request.Context())
	actorSessionID, sessionOK := authctx.SessionID(c.Request.Context())
	commandID, commandOK := c.Request.Context().Value(
		cultivationCommandIDContextKey{},
	).(uuid.UUID)
	if !userOK || !sessionOK {
		failRankingReconciliation(
			c,
			apperr.New(apperr.CodeUnauthorized, "authentication required", nil),
		)
		return
	}
	if !commandOK || commandID == uuid.Nil {
		failRankingReconciliation(
			c,
			apperr.New(apperr.CodeBadRequest, "valid canonical Idempotency-Key is required", nil),
		)
		return
	}
	request, err := commonrequest.ParseRequest[startRankingReconciliationRequest](c)
	if err != nil {
		failRankingReconciliation(c, err)
		return
	}
	previewChecksum, checksumOK := decodeCanonicalChecksum(request.PreviewChecksum)
	if !checksumOK {
		failRankingReconciliation(c, entity.ErrInvalidRankingReconciliation)
		return
	}
	result, err := handler.service.Start(
		c.Request.Context(),
		entity.RankingReconciliationCommand{
			ID:               commandID,
			ActorUserID:      actorUserID,
			ActorSessionID:   actorSessionID,
			Reason:           request.Reason,
			SourceCutoff:     request.SourceCutoff.UTC(),
			ExpectedRevision: request.ExpectedRevision,
			SourceCount:      request.SourceCount,
			RepairCount:      request.RepairCount,
			PreviewChecksum:  previewChecksum,
		},
	)
	if err != nil {
		failRankingReconciliation(c, err)
		return
	}
	if result == nil || result.Operation.ID == uuid.Nil ||
		result.Operation.Kind != service.RankingReconciliationOperationKind {
		failRankingReconciliation(c, entity.ErrInvalidRankingReconciliation)
		return
	}
	c.Header("Location", "/admin/operations/"+result.Operation.ID.String())
	c.Header("Idempotency-Replayed", strconv.FormatBool(result.IdempotentReplay))
	response.RespondStatus(c, http.StatusAccepted, toRankingReconciliationStartResponse(result))
}

func failRankingReconciliation(c *gin.Context, err error) {
	switch {
	case errors.Is(err, operation.ErrConflict),
		errors.Is(err, entity.ErrRankingReconciliationActive),
		errors.Is(err, entity.ErrRankingReconciliationPreviewConflict):
		err = apperr.New(apperr.CodeConflict, "ranking reconciliation conflicts with the existing command", err)
	case errors.Is(err, entity.ErrInvalidRankingReconciliation),
		errors.Is(err, operation.ErrInvalidRequest),
		errors.Is(err, operation.ErrInvalidOperation):
		err = apperr.New(apperr.CodeBadRequest, "invalid ranking reconciliation request", err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
	default:
		err = apperr.New(apperr.CodeInternalServer, "ranking reconciliation unavailable", err)
	}
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
}
