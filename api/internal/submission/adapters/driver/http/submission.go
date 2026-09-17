package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"
	"github.com/huynhanx03/go-common/pkg/common/http/response"
	"github.com/huynhanx03/go-common/pkg/correlation"
	commondto "github.com/huynhanx03/go-common/pkg/dto"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/submission/contract"
	service "github.com/huynhanx03/judgify/internal/submission/usecase"
	"github.com/huynhanx03/judgify/pkg/judging"
)

const idempotencyReplayHeader = "Idempotency-Replayed"

// SubmissionHandler defines the submission HTTP handler interface.
type SubmissionHandler interface {
	SubmitAccepted(*gin.Context)
	Get(ctx context.Context, req *submissionIDRequest) (*submissionResponse, error)
	FindByProblem(ctx context.Context, req *problemSubmissionsRequest) ([]*submissionSummaryResponse, error)
	FindMySubmissions(*gin.Context)
	FindAdmin(ctx context.Context, req *contract.AdminSubmissionFindRequest) (*commondto.Paginated[*adminSubmissionSummaryResponse], error)
	GetAdmin(ctx context.Context, req *submissionIDRequest) (*adminSubmissionDetailResponse, error)
	Rejudge(*gin.Context)
	Cancel(*gin.Context)
}

type submissionHandler struct {
	handler.BaseHandler
	submissionService        ports.SubmissionService
	submissionCommandService ports.SubmissionCommandService
	operations               ports.SubmissionOperations
}

// NewSubmissionHandlerWithOperations extends the established read/intake
// constructor with the separate administrative operation service. Keeping it
// distinct prevents a missing operations dependency from weakening ordinary
// contestant submission routes.
func NewSubmissionHandlerWithOperations(
	submissionService ports.SubmissionService,
	commandService ports.SubmissionCommandService,
	operations ports.SubmissionOperations,
) SubmissionHandler {
	handler := NewSubmissionHandler(submissionService, commandService).(*submissionHandler)
	handler.operations = operations
	return handler
}

// NewSubmissionHandler creates a new SubmissionHandler instance.
func NewSubmissionHandler(
	submissionService ports.SubmissionService,
	commandServices ...ports.SubmissionCommandService,
) SubmissionHandler {
	handler := &submissionHandler{submissionService: submissionService}
	if len(commandServices) == 1 {
		handler.submissionCommandService = commandServices[0]
	}
	return handler
}

// SubmitAccepted owns the strict asynchronous command boundary. Generic
// handler.Wrap is intentionally not used because this route requires one
// opaque header and an explicit HTTP 202 response.
func (h *submissionHandler) SubmitAccepted(c *gin.Context) {
	submissionCommandNoStore(c)
	if h == nil || h.submissionCommandService == nil ||
		c == nil || c.Request == nil {
		failSubmissionCommand(
			c,
			apperr.New(
				apperr.CodeInternalServer,
				"submission intake is unavailable",
				serviceUnavailableError(),
			),
		)
		return
	}
	userID, authenticated := authctx.UserID(c.Request.Context())
	if !authenticated {
		failSubmissionCommand(
			c,
			apperr.New(apperr.CodeUnauthorized, "unauthorized", nil),
		)
		return
	}
	cid := correlation.FromContext(c.Request.Context())
	if correlation.Validate(cid) != nil {
		failSubmissionCommand(
			c,
			apperr.New(
				apperr.CodeBadRequest,
				"invalid request correlation",
				intake.ErrInvalidCommand,
			),
		)
		return
	}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || intake.ValidateIdempotencyKey(keys[0]) != nil {
		failSubmissionCommand(
			c,
			apperr.New(
				apperr.CodeBadRequest,
				"exactly one valid Idempotency-Key header is required",
				intake.ErrInvalidCommand,
			),
		)
		return
	}
	request, err := commonrequest.ParseRequest[createSubmissionRequest](c)
	if err != nil {
		failSubmissionCommand(c, err)
		return
	}
	command, err := toCreateSubmissionCommand(request)
	if err != nil {
		failSubmissionCommand(c, apperr.New(apperr.CodeBadRequest, "invalid submission request", err))
		return
	}
	if err := contract.ValidateCreateSubmissionCommand(command); err != nil {
		var violation *judging.ContentViolation
		if errors.As(err, &violation) {
			failSubmissionCommand(
				c,
				apperr.New(
					apperr.CodeBadRequest,
					violation.Error(),
					err,
				).WithDetails(violation),
			)
			return
		}
		failSubmissionCommand(
			c,
			apperr.New(
				apperr.CodeBadRequest,
				"invalid submission request",
				err,
			),
		)
		return
	}
	result, err := h.submissionCommandService.Submit(
		c.Request.Context(),
		userID,
		keys[0],
		command,
	)
	if err != nil {
		failSubmissionCommand(c, err)
		return
	}
	if result == nil || result.ID == uuid.Nil {
		failSubmissionCommand(
			c,
			apperr.New(
				apperr.CodeInternalServer,
				"submission intake returned an invalid result",
				intake.ErrPersistence,
			),
		)
		return
	}
	c.Header("Location", "/submissions/"+result.ID.String())
	c.Header(
		idempotencyReplayHeader,
		strconv.FormatBool(result.IdempotentReplay),
	)
	response.RespondStatus(c, http.StatusAccepted, toSubmissionAcceptedResponse(result))
}

func submissionCommandNoStore(c *gin.Context) {
	if c == nil {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

func failSubmissionCommand(c *gin.Context, err error) {
	if c == nil {
		return
	}
	_ = c.Error(err)
	var appError *apperr.AppError
	if errors.As(err, &appError) &&
		appError.Code == ports.CodeJudgeCapacityUnavailable {
		c.Header("Retry-After", "2")
		c.JSON(http.StatusServiceUnavailable, response.Body{
			Code:    appError.Code,
			Message: appError.Message,
			Data:    appError.Details,
			CID:     correlation.FromContext(c.Request.Context()),
		})
		c.Abort()
		return
	}
	if errors.As(err, &appError) &&
		appError.Code == apperr.CodeTooManyRequests {
		c.Header("Retry-After", "1")
	}
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
	c.Abort()
}

func serviceUnavailableError() error {
	return errors.New("submission handler dependency unavailable")
}

func (h *submissionHandler) Get(ctx context.Context, req *submissionIDRequest) (*submissionResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil)
	}
	id, err := toSubmissionID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.submissionService.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return toSubmissionResponse(result), nil
}

func (h *submissionHandler) FindByProblem(ctx context.Context, req *problemSubmissionsRequest) ([]*submissionSummaryResponse, error) {
	id, err := toProblemID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.submissionService.FindByProblemID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toSubmissionSummaryResponses(result), nil
}

func (h *submissionHandler) FindMySubmissions(c *gin.Context) {
	if h == nil || h.submissionService == nil || c == nil ||
		c.Request == nil {
		failSubmissionRead(
			c,
			apperr.New(
				apperr.CodeInternalServer,
				"submission history is unavailable",
				serviceUnavailableError(),
			),
		)
		return
	}
	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		failSubmissionRead(
			c,
			apperr.New(apperr.CodeUnauthorized, "unauthorized", nil),
		)
		return
	}
	request, err := parseMySubmissionsRequest(c)
	if err != nil {
		failSubmissionRead(c, err)
		return
	}
	result, err := h.submissionService.FindMyByProblem(
		c.Request.Context(),
		userID,
		request,
	)
	if err != nil {
		failSubmissionRead(c, err)
		return
	}
	response.Respond(c, toMySubmissionsPageResponse(result))
}

func parseMySubmissionsRequest(
	c *gin.Context,
) (*contract.MySubmissionsRequest, error) {
	if c == nil || c.Request == nil || len(c.Request.URL.RawQuery) > 1024 {
		return nil, invalidMySubmissionsHTTPQuery(nil)
	}
	rawProblemID := c.Param("id")
	problemID, err := uuid.Parse(rawProblemID)
	if err != nil || problemID == uuid.Nil || problemID.String() != rawProblemID {
		return nil, invalidMySubmissionsHTTPQuery(err)
	}
	query, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return nil, invalidMySubmissionsHTTPQuery(err)
	}
	for key := range query {
		if key != "cursor" && key != "limit" && key != "contest_id" {
			return nil, invalidMySubmissionsHTTPQuery(nil)
		}
	}
	cursorValues, cursorPresent := query["cursor"]
	if cursorPresent &&
		(len(cursorValues) != 1 || cursorValues[0] == "" ||
			strings.TrimSpace(cursorValues[0]) != cursorValues[0]) {
		return nil, invalidMySubmissionsHTTPQuery(nil)
	}
	limitValues, limitPresent := query["limit"]
	if limitPresent &&
		(len(limitValues) != 1 || limitValues[0] == "" ||
			strings.TrimSpace(limitValues[0]) != limitValues[0]) {
		return nil, invalidMySubmissionsHTTPQuery(nil)
	}
	contestValues, contestPresent := query["contest_id"]
	if contestPresent &&
		(len(contestValues) != 1 || contestValues[0] == "" ||
			strings.TrimSpace(contestValues[0]) != contestValues[0]) {
		return nil, invalidMySubmissionsHTTPQuery(nil)
	}
	request := &contract.MySubmissionsRequest{ProblemID: problemID}
	if cursorPresent {
		request.Cursor = cursorValues[0]
	}
	if contestPresent {
		contestID, parseErr := uuid.Parse(contestValues[0])
		if parseErr != nil || contestID == uuid.Nil ||
			contestID.String() != contestValues[0] {
			return nil, invalidMySubmissionsHTTPQuery(parseErr)
		}
		request.ContestID = &contestID
	}
	if !limitPresent {
		return request, nil
	}
	limit, err := strconv.Atoi(limitValues[0])
	if err != nil || strconv.Itoa(limit) != limitValues[0] || limit < 1 ||
		limit > ports.MaximumMySubmissionPageLimit {
		return nil, invalidMySubmissionsHTTPQuery(err)
	}
	request.Limit = limit
	return request, nil
}

func invalidMySubmissionsHTTPQuery(cause error) error {
	return apperr.New(
		apperr.CodeBadRequest,
		"invalid submission history query",
		cause,
	)
}

func failSubmissionRead(c *gin.Context, err error) {
	if c == nil {
		return
	}
	_ = c.Error(err)
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
	c.Abort()
}

func (h *submissionHandler) FindAdmin(
	ctx context.Context,
	req *contract.AdminSubmissionFindRequest,
) (*commondto.Paginated[*adminSubmissionSummaryResponse], error) {
	result, err := h.submissionService.FindAdmin(ctx, req)
	if err != nil {
		return nil, err
	}
	return toAdminSubmissionPageResponse(result), nil
}

func (h *submissionHandler) GetAdmin(
	ctx context.Context,
	req *submissionIDRequest,
) (*adminSubmissionDetailResponse, error) {
	id, err := toSubmissionID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.submissionService.GetAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	return toAdminSubmissionDetailResponse(result), nil
}

// Rejudge accepts a strict, one-header administrative command. The handler
// does not expose a generic mutation surface for immutable judge contracts.
func (h *submissionHandler) Rejudge(c *gin.Context) {
	submissionCommandNoStore(c)
	if h == nil || h.operations == nil || c == nil || c.Request == nil {
		failJudgeOperation(c, ports.ErrOperationUnavailable)
		return
	}
	actor, ok := judgeOperationActor(c.Request.Context())
	if !ok {
		failJudgeOperation(c, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil))
		return
	}
	key, ok := strictOperationIdempotencyKey(c)
	if !ok {
		failJudgeOperation(c, apperr.New(apperr.CodeBadRequest, "exactly one UUID Idempotency-Key header is required", service.ErrInvalidJudgeOperation))
		return
	}
	request, err := commonrequest.ParseRequest[rejudgeSubmissionRequest](c)
	if err != nil {
		failJudgeOperation(c, err)
		return
	}
	submissionID, err := uuid.Parse(c.Param("id"))
	if err != nil || submissionID == uuid.Nil || submissionID.String() != c.Param("id") {
		failJudgeOperation(c, apperr.New(apperr.CodeBadRequest, "invalid submission id", service.ErrInvalidJudgeOperation))
		return
	}
	result, err := h.operations.Rejudge(c.Request.Context(), actor, toRejudgeCommand(request, submissionID, key))
	if err != nil {
		failJudgeOperation(c, err)
		return
	}
	c.Header(idempotencyReplayHeader, strconv.FormatBool(result.IdempotentReplay))
	response.SuccessResponse(c, apperr.CodeSuccess, toJudgeOperationResponse(service.ToJudgeOperationResponse(result)))
}

// Cancel marks the observed generation as cancelled. A running worker sees
// the terminal aggregate phase on its next fenced heartbeat and exits without
// publishing an active contestant result.
func (h *submissionHandler) Cancel(c *gin.Context) {
	submissionCommandNoStore(c)
	if h == nil || h.operations == nil || c == nil || c.Request == nil {
		failJudgeOperation(c, ports.ErrOperationUnavailable)
		return
	}
	actor, ok := judgeOperationActor(c.Request.Context())
	if !ok {
		failJudgeOperation(c, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil))
		return
	}
	request, err := commonrequest.ParseRequest[cancelSubmissionRequest](c)
	if err != nil {
		failJudgeOperation(c, err)
		return
	}
	submissionID, err := uuid.Parse(c.Param("id"))
	if err != nil || submissionID == uuid.Nil || submissionID.String() != c.Param("id") {
		failJudgeOperation(c, apperr.New(apperr.CodeBadRequest, "invalid submission id", service.ErrInvalidJudgeOperation))
		return
	}
	result, err := h.operations.Cancel(c.Request.Context(), actor, toCancelCommand(request, submissionID))
	if err != nil {
		failJudgeOperation(c, err)
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, toJudgeOperationResponse(service.ToJudgeOperationResponse(result)))
}

func strictOperationIdempotencyKey(c *gin.Context) (uuid.UUID, bool) {
	if c == nil || c.Request == nil {
		return uuid.Nil, false
	}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) != 1 {
		return uuid.Nil, false
	}
	key, err := uuid.Parse(keys[0])
	return key, err == nil && key != uuid.Nil && key.String() == keys[0]
}

func judgeOperationActor(ctx context.Context) (ports.OperationActor, bool) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return ports.OperationActor{}, false
	}
	actor := ports.OperationActor{UserID: userID}
	if sessionID, present := authctx.SessionID(ctx); present {
		actor.SessionID = &sessionID
	}
	return actor, true
}

func failJudgeOperation(c *gin.Context, err error) {
	if c == nil {
		return
	}
	switch {
	case errors.Is(err, ports.ErrOperationConflict):
		err = apperr.New(apperr.CodeConflict, "submission state changed; refresh and retry", err)
	case errors.Is(err, service.ErrInvalidJudgeOperation):
		err = apperr.New(apperr.CodeBadRequest, "invalid judge operation", err)
	case errors.Is(err, ports.ErrOperationUnavailable):
		err = apperr.New(apperr.CodeInternalServer, "judge operations are unavailable", err)
	}
	_ = c.Error(err)
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
	c.Abort()
}
