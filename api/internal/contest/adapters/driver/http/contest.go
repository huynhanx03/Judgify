package http

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"
	"github.com/huynhanx03/go-common/pkg/common/http/response"
	"github.com/huynhanx03/go-common/pkg/correlation"

	"github.com/huynhanx03/judgify/internal/authctx"
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"

	d "github.com/huynhanx03/go-common/pkg/dto"
)

// ContestHandler defines the contest HTTP handler interface.
type ContestHandler interface {
	FindPublic(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*contestResponse], error)
	GetPublic(ctx context.Context, req *getContestRequest) (*contestResponse, error)
	GetProblem(ctx context.Context, req *getContestProblemRequest) (*contestProblemDetailResponse, error)
	FindAdmin(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*contestResponse], error)
	GetAdmin(ctx context.Context, req *getContestRequest) (*contestResponse, error)
	Create(ctx context.Context, req *createContestRequest) (*contestResponse, error)
	Update(ctx context.Context, req *updateContestRequest) (*contestResponse, error)
	PublishAccepted(*gin.Context)
	EndAccepted(*gin.Context)
	CancelAccepted(*gin.Context)
	Delete(ctx context.Context, req *deleteContestRequest) (*contestResponse, error)
}

type contestHandler struct {
	handler.BaseHandler
	contestService   ports.ContestService
	lifecycleService ports.LifecycleService
}

// NewContestHandler creates a new ContestHandler instance.
func NewContestHandler(
	contestService ports.ContestService,
	lifecycleServices ...ports.LifecycleService,
) ContestHandler {
	value := &contestHandler{contestService: contestService}
	if len(lifecycleServices) == 1 {
		value.lifecycleService = lifecycleServices[0]
	}
	return value
}

func (h *contestHandler) FindPublic(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*contestResponse], error) {
	result, err := h.contestService.FindPublic(ctx, req)
	return toContestPage(result), err
}

func (h *contestHandler) GetPublic(ctx context.Context, req *getContestRequest) (*contestResponse, error) {
	userID := extractUserID(ctx)
	result, err := h.contestService.GetPublic(ctx, req.ID, userID)
	return toContestResponse(result), err
}

func (h *contestHandler) GetProblem(
	ctx context.Context,
	req *getContestProblemRequest,
) (*contestProblemDetailResponse, error) {
	result, err := h.contestService.GetProblem(
		ctx,
		req.ContestID,
		req.ContestProblemID,
	)
	return toContestProblemDetailResponse(result), err
}

func (h *contestHandler) FindAdmin(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*contestResponse], error) {
	result, err := h.contestService.FindAdmin(ctx, req)
	return toContestPage(result), err
}

func (h *contestHandler) GetAdmin(ctx context.Context, req *getContestRequest) (*contestResponse, error) {
	result, err := h.contestService.GetAdmin(ctx, req.ID)
	return toContestResponse(result), err
}

func (h *contestHandler) Create(ctx context.Context, req *createContestRequest) (*contestResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil)
	}
	result, err := h.contestService.Create(ctx, userID, toCreateContestRequest(req))
	return toContestResponse(result), err
}

func (h *contestHandler) Update(ctx context.Context, req *updateContestRequest) (*contestResponse, error) {
	command := toUpdateContestRequest(req)
	result, err := h.contestService.Update(ctx, command.ID, command)
	return toContestResponse(result), err
}

var contestVersionETagPattern = regexp.MustCompile(
	`^"contest-v([1-9][0-9]{0,18})"$`,
)

const contestIdempotencyReplayHeader = "Idempotency-Replayed"

// PublishAccepted is the canonical lifecycle command boundary. The retired
// Publish method remains source-compatible but is no longer registered live.
func (h *contestHandler) PublishAccepted(c *gin.Context) {
	h.transitionAccepted(c, entity.ContestUpcoming)
}

func (h *contestHandler) EndAccepted(c *gin.Context) {
	h.transitionAccepted(c, entity.ContestEnded)
}

func (h *contestHandler) CancelAccepted(c *gin.Context) {
	h.transitionAccepted(c, entity.ContestCancelled)
}

func (h *contestHandler) transitionAccepted(
	c *gin.Context,
	target entity.ContestState,
) {
	contestLifecycleNoStore(c)
	if h == nil || h.lifecycleService == nil ||
		c == nil || c.Request == nil ||
		(target != entity.ContestUpcoming &&
			target != entity.ContestEnded &&
			target != entity.ContestCancelled) {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeInternalServer,
				"contest lifecycle is unavailable",
				ports.ErrContestCommandDenied,
			),
		)
		return
	}
	_, authenticated := authctx.Principal(c.Request.Context())
	actorID, actorOK := authctx.UserID(c.Request.Context())
	sessionID, sessionOK := authctx.SessionID(c.Request.Context())
	if !authenticated || !actorOK || !sessionOK {
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
	contestID, err := uuid.Parse(c.Param("id"))
	if err != nil || contestID == uuid.Nil ||
		contestID.String() != c.Param("id") {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeBadRequest,
				"invalid contest identity",
				ports.ErrInvalidContestCommand,
			),
		)
		return
	}
	idempotencyValues := c.Request.Header.Values("Idempotency-Key")
	if len(idempotencyValues) != 1 {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeBadRequest,
				"exactly one Idempotency-Key header is required",
				ports.ErrInvalidContestCommand,
			),
		)
		return
	}
	commandID, err := uuid.Parse(idempotencyValues[0])
	if err != nil || commandID == uuid.Nil ||
		commandID.String() != idempotencyValues[0] {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeBadRequest,
				"Idempotency-Key must be a canonical UUID",
				ports.ErrInvalidContestCommand,
			),
		)
		return
	}
	matchValues := c.Request.Header.Values("If-Match")
	if len(matchValues) != 1 {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeBadRequest,
				"exactly one strong contest If-Match header is required",
				ports.ErrInvalidContestCommand,
			),
		)
		return
	}
	expectedVersion, err := parseContestVersionETag(matchValues[0])
	if err != nil {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeBadRequest,
				"invalid contest If-Match header",
				err,
			),
		)
		return
	}
	request, err := commonrequest.ParseRequest[publishContestCommandRequest](c)
	if err != nil {
		failContestLifecycle(c, err)
		return
	}
	result, err := h.lifecycleService.Transition(
		c.Request.Context(),
		ports.TransitionCommand{
			ContestID:       contestID,
			Target:          target,
			ExpectedVersion: expectedVersion,
			IdempotencyKey:  commandID,
			ActorUserID:     actorID,
			ActorSessionID:  &sessionID,
			Reason:          request.Reason,
		},
	)
	if err != nil {
		failContestLifecycle(c, mapContestLifecycleError(err))
		return
	}
	if result.ContestID != contestID ||
		result.CommandID != commandID ||
		result.To != target ||
		result.Version != expectedVersion+1 ||
		result.OccurredAt.IsZero() {
		failContestLifecycle(
			c,
			apperr.New(
				apperr.CodeInternalServer,
				"contest lifecycle returned an invalid result",
				ports.ErrContestCommandDenied,
			),
		)
		return
	}
	c.Header("Location", "/contests/"+contestID.String())
	c.Header(
		"ETag",
		contestVersionETag(result.Version),
	)
	c.Header(
		contestIdempotencyReplayHeader,
		strconv.FormatBool(result.ExactReplay),
	)
	response.RespondStatus(
		c,
		http.StatusOK,
		toContestLifecycleResponse(contract.ContestLifecycleResponse{
			ContestID:        result.ContestID,
			From:             string(result.From),
			To:               string(result.To),
			Version:          result.Version,
			OccurredAt:       result.OccurredAt,
			CommandID:        result.CommandID,
			IdempotentReplay: result.ExactReplay,
			CID:              cid,
		}),
	)
}

func parseContestVersionETag(value string) (int64, error) {
	matches := contestVersionETagPattern.FindStringSubmatch(value)
	if len(matches) != 2 {
		return 0, ports.ErrInvalidContestCommand
	}
	version, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil || version < 1 {
		return 0, ports.ErrInvalidContestCommand
	}
	return version, nil
}

func contestVersionETag(version int64) string {
	return `"contest-v` + strconv.FormatInt(version, 10) + `"`
}

func contestLifecycleNoStore(c *gin.Context) {
	if c == nil {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

func failContestLifecycle(c *gin.Context, err error) {
	if c == nil {
		return
	}
	_ = c.Error(err)
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
	c.Abort()
}

func mapContestLifecycleError(err error) error {
	switch {
	case errors.Is(err, ports.ErrInvalidContestCommand),
		errors.Is(err, entity.ErrContestReason):
		return apperr.New(
			apperr.CodeBadRequest,
			"invalid contest lifecycle command",
			err,
		)
	case errors.Is(err, entity.ErrContestNotFound):
		return apperr.New(
			apperr.CodeNotFound,
			"contest not found",
			err,
		)
	case errors.Is(err, entity.ErrContestVersionConflict),
		errors.Is(err, entity.ErrContestCommandConflict),
		errors.Is(err, entity.ErrContestTransition):
		return apperr.New(
			apperr.CodeConflict,
			"contest lifecycle command conflicts with current state",
			err,
		)
	case errors.Is(err, entity.ErrContestPolicy),
		errors.Is(err, entity.ErrContestProblemSnapshot):
		return apperr.New(
			apperr.CodeValidationFailed,
			"contest cannot be published until its policy and problems are valid",
			err,
		)
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return apperr.New(
			apperr.CodeInternalServer,
			"contest lifecycle command failed",
			err,
		)
	}
}

func (h *contestHandler) Delete(ctx context.Context, req *deleteContestRequest) (*contestResponse, error) {
	command := toDeleteContestRequest(req)
	if err := h.contestService.Delete(ctx, command.ID, command.Reason); err != nil {
		return nil, err
	}
	return nil, nil
}

// extractUserID returns the authenticated user's ID, or uuid.Nil if absent.
func extractUserID(ctx context.Context) uuid.UUID {
	id, _ := authctx.UserID(ctx)
	return id
}

// ensure handler.Wrap is available (used by handler group)
var _ = handler.Wrap[any, any]
