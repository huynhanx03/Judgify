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
	"github.com/huynhanx03/go-common/pkg/correlation"

	"github.com/huynhanx03/judgify/internal/authctx"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type RegistrationHandler interface {
	RegisterAccepted(*gin.Context)
	WithdrawAccepted(*gin.Context)
}

type registrationHandler struct {
	commands ports.RegistrationCommandService
}

func NewRegistrationHandler(
	commands ports.RegistrationCommandService,
) RegistrationHandler {
	return &registrationHandler{commands: commands}
}

func (handler *registrationHandler) RegisterAccepted(c *gin.Context) {
	handler.apply(c, ports.RegistrationRegister)
}

func (handler *registrationHandler) WithdrawAccepted(c *gin.Context) {
	handler.apply(c, ports.RegistrationWithdraw)
}

func (handler *registrationHandler) apply(
	c *gin.Context,
	mutation ports.RegistrationMutation,
) {
	contestLifecycleNoStore(c)
	if handler == nil || handler.commands == nil ||
		c == nil || c.Request == nil {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeInternalServer,
				"contest registration is unavailable",
				ports.ErrContestCommandDenied,
			),
		)
		return
	}
	_, authenticated := authctx.Principal(c.Request.Context())
	userID, userOK := authctx.UserID(c.Request.Context())
	sessionID, sessionOK := authctx.SessionID(c.Request.Context())
	if !authenticated || !userOK || !sessionOK {
		failContestLifecycle(
			c,
			apperr.New(apperr.CodeUnauthorized, "unauthorized", nil),
		)
		return
	}
	cid := correlation.FromContext(c.Request.Context())
	if correlation.Validate(cid) != nil {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeBadRequest,
				"invalid request correlation",
				ports.ErrInvalidContestCommand,
			),
		)
		return
	}
	contestID, expectedContestVersion, commandID, err :=
		parseRegistrationCommandTransport(c)
	if err != nil {
		failContestLifecycle(c, err)
		return
	}
	request, err := commonrequest.ParseRequest[registrationCommandRequest](c)
	if err != nil {
		failContestLifecycle(c, err)
		return
	}
	expectedRegistrationVersion :=
		request.ExpectedRegistrationVersion
	result, err := handler.commands.Apply(
		c.Request.Context(),
		ports.RegistrationCommand{
			ContestID:                   contestID,
			UserID:                      userID,
			ExpectedContestVersion:      expectedContestVersion,
			ExpectedRegistrationVersion: &expectedRegistrationVersion,
			IdempotencyKey:              commandID,
			ActorUserID:                 userID,
			ActorSessionID:              &sessionID,
		},
		mutation,
	)
	if err != nil {
		failContestLifecycle(c, mapContestRegistrationError(err))
		return
	}
	if result.CommandID != commandID ||
		result.ContestVersion != expectedContestVersion ||
		result.Registration.ID == uuid.Nil ||
		result.Registration.ContestID != contestID ||
		result.Registration.UserID != userID ||
		result.Registration.Version < 1 ||
		result.OccurredAt.IsZero() {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeInternalServer,
				"contest registration returned an invalid result",
				ports.ErrContestCommandDenied,
			),
		)
		return
	}
	expectedStatus := entity.RegistrationRegistered
	if mutation == ports.RegistrationWithdraw {
		expectedStatus = entity.RegistrationWithdrawn
	}
	if result.Registration.Status != expectedStatus {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeInternalServer,
				"contest registration returned an invalid state",
				ports.ErrContestCommandDenied,
			),
		)
		return
	}
	c.Header("Location", "/contests/"+contestID.String())
	c.Header("ETag", contestVersionETag(result.ContestVersion))
	c.Header(
		contestIdempotencyReplayHeader,
		strconv.FormatBool(result.ExactReplay),
	)
	response.RespondStatus(
		c,
		http.StatusOK,
		registrationCommandResponse{
			ContestID:           contestID.String(),
			UserID:              userID.String(),
			RegistrationID:      result.Registration.ID.String(),
			Status:              string(result.Registration.Status),
			RegistrationVersion: result.Registration.Version,
			ContestVersion:      result.ContestVersion,
			OccurredAt:          result.OccurredAt,
			CommandID:           commandID.String(),
			IdempotentReplay:    result.ExactReplay,
			CID:                 cid,
		},
	)
}

func parseRegistrationCommandTransport(
	c *gin.Context,
) (uuid.UUID, int64, uuid.UUID, error) {
	contestID, err := uuid.Parse(c.Param("id"))
	if err != nil || contestID == uuid.Nil ||
		contestID.String() != c.Param("id") {
		return uuid.Nil, 0, uuid.Nil, apperr.New(
			apperr.CodeBadRequest,
			"invalid contest identity",
			ports.ErrInvalidContestCommand,
		)
	}
	idempotencyValues := c.Request.Header.Values("Idempotency-Key")
	if len(idempotencyValues) != 1 {
		return uuid.Nil, 0, uuid.Nil, apperr.New(
			apperr.CodeBadRequest,
			"exactly one Idempotency-Key header is required",
			ports.ErrInvalidContestCommand,
		)
	}
	commandID, err := uuid.Parse(idempotencyValues[0])
	if err != nil || commandID == uuid.Nil ||
		commandID.String() != idempotencyValues[0] {
		return uuid.Nil, 0, uuid.Nil, apperr.New(
			apperr.CodeBadRequest,
			"Idempotency-Key must be a canonical UUID",
			ports.ErrInvalidContestCommand,
		)
	}
	matchValues := c.Request.Header.Values("If-Match")
	if len(matchValues) != 1 {
		return uuid.Nil, 0, uuid.Nil, apperr.New(
			apperr.CodeBadRequest,
			"exactly one strong contest If-Match header is required",
			ports.ErrInvalidContestCommand,
		)
	}
	expectedVersion, err := parseContestVersionETag(matchValues[0])
	if err != nil {
		return uuid.Nil, 0, uuid.Nil, apperr.New(
			apperr.CodeBadRequest,
			"invalid contest If-Match header",
			err,
		)
	}
	return contestID, expectedVersion, commandID, nil
}

func mapContestRegistrationError(err error) error {
	switch {
	case errors.Is(err, ports.ErrInvalidContestCommand),
		errors.Is(err, entity.ErrContestReason):
		return apperr.New(
			apperr.CodeBadRequest,
			"invalid contest registration command",
			err,
		)
	case errors.Is(err, entity.ErrContestNotFound):
		return apperr.New(
			apperr.CodeNotFound,
			"contest not found",
			err,
		)
	case errors.Is(err, entity.ErrContestRegistrationBanned):
		return apperr.New(
			apperr.CodeForbidden,
			"contest registration is unavailable",
			err,
		)
	case errors.Is(err, entity.ErrContestVersionConflict),
		errors.Is(err, entity.ErrContestCommandConflict),
		errors.Is(err, entity.ErrContestRegistrationTransition),
		errors.Is(err, entity.ErrContestRegistrationMode),
		errors.Is(err, entity.ErrContestRegistrationFull),
		errors.Is(err, entity.ErrContestSubmissionExists):
		return apperr.New(
			apperr.CodeConflict,
			"contest registration conflicts with current state",
			err,
		)
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return apperr.New(
			apperr.CodeInternalServer,
			"contest registration command failed",
			err,
		)
	}
}
