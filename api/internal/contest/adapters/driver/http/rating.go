package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	handlerCommon "github.com/huynhanx03/go-common/pkg/common/http/handler"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/authctx"
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	"github.com/huynhanx03/judgify/internal/ports"
)

// RatingHandler defines the rating HTTP handler interface.
type RatingHandler interface {
	GetContestRatingChanges(ctx context.Context, req *contestIDRequest) ([]*ratingChangeResponse, error)
	StartRerating(*gin.Context)
}

type ratingHandler struct {
	handlerCommon.BaseHandler
	ratingService ports.RatingService
	rerating      ports.RatingReratingStarter
}

// NewRatingHandler creates a new RatingHandler instance.
func NewRatingHandler(
	ratingService ports.RatingService,
	rerating ...ports.RatingReratingStarter,
) RatingHandler {
	value := &ratingHandler{ratingService: ratingService}
	if len(rerating) == 1 {
		value.rerating = rerating[0]
	}
	return value
}

func (h *ratingHandler) GetContestRatingChanges(ctx context.Context, req *contestIDRequest) ([]*ratingChangeResponse, error) {
	id, err := toContestID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.ratingService.GetContestRatingChanges(ctx, id)
	if err != nil {
		return nil, err
	}
	return toRatingChanges(result), nil
}

func (h *ratingHandler) StartRerating(c *gin.Context) {
	contestLifecycleNoStore(c)
	if h == nil || h.rerating == nil ||
		c == nil || c.Request == nil {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeInternalServer,
				"historical rating rebuild is unavailable",
				nil,
			),
		)
		return
	}
	actorUserID, userOK := authctx.UserID(c.Request.Context())
	actorSessionID, sessionOK := authctx.SessionID(c.Request.Context())
	if !userOK || !sessionOK {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeUnauthorized,
				"authentication required",
				nil,
			),
		)
		return
	}
	contestID, err := parseCanonicalRatingUUID(c.Param("id"))
	if err != nil {
		failContestLifecycle(c, err)
		return
	}
	values := c.Request.Header.Values(
		commonrequest.IdempotencyKeyHeader,
	)
	if len(values) != 1 {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeBadRequest,
				"exactly one Idempotency-Key header is required",
				nil,
			),
		)
		return
	}
	idempotencyKey, err := parseCanonicalRatingUUID(values[0])
	if err != nil {
		failContestLifecycle(c, err)
		return
	}
	request, err := commonrequest.ParseRequest[startRatingReratingRequest](c)
	if err != nil {
		failContestLifecycle(c, err)
		return
	}
	result, err := h.rerating.Start(
		c.Request.Context(),
		actorUserID,
		actorSessionID,
		idempotencyKey,
		contestID,
		request.Reason,
	)
	if err != nil {
		failContestLifecycle(c, mapRatingReratingError(err))
		return
	}
	if result == nil || result.Operation.ID == uuid.Nil ||
		result.Operation.Kind !=
			contract.RatingReratingOperationKind ||
		result.TriggerContestID != contestID ||
		result.FromSequence < 1 ||
		result.ThroughSequence < result.FromSequence {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeInternalServer,
				"historical rating rebuild returned an invalid result",
				nil,
			),
		)
		return
	}
	c.Header(
		"Location",
		"/admin/operations/"+result.Operation.ID.String(),
	)
	c.Header(
		contestIdempotencyReplayHeader,
		strconv.FormatBool(result.IdempotentReplay),
	)
	response.RespondStatus(c, http.StatusAccepted, toRatingReratingStartResponse(result))
}

func parseCanonicalRatingUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil ||
		parsed.String() != value {
		return uuid.Nil, apperr.New(
			apperr.CodeBadRequest,
			"identity must be a canonical UUID",
			err,
		)
	}
	return parsed, nil
}

func mapRatingReratingError(err error) error {
	switch {
	case errors.Is(err, operation.ErrConflict),
		errors.Is(err, ports.ErrRatingReratingConflict),
		errors.Is(err, ports.ErrRatingSequenceBlocked):
		return apperr.New(
			apperr.CodeConflict,
			"historical rating rebuild conflicts with current state",
			err,
		)
	case errors.Is(err, ports.ErrRatingNotAvailable):
		return apperr.New(
			apperr.CodeConflict,
			"contest does not have an active rating sequence",
			err,
		)
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return apperr.New(
			apperr.CodeInternalServer,
			"historical rating rebuild could not be scheduled",
			err,
		)
	}
}
