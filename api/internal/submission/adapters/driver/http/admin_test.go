package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

func TestRejudgeHTTPRequiresStrictIdempotencyAndReturnsSafeAcknowledgement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	submissionID := uuid.New()
	jobID := uuid.New()
	operations := &httpOperationsStub{rejudgeResult: ports.OperationResult{
		SubmissionID: submissionID, Generation: 2, AggregateVersion: 9,
		Phase: entity.PhaseQueued, JobID: &jobID,
	}}
	handler := NewSubmissionHandlerWithOperations(nil, nil, operations)
	router := newOperationRouter(t, handler)
	request := httptest.NewRequest(
		http.MethodPost,
		"/admin/submissions/"+submissionID.String()+"/rejudge",
		bytes.NewBufferString(`{"expected_version":8,"reason":"checker correction"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", uuid.NewString())
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, "false", response.Header().Get(idempotencyReplayHeader))
	require.Equal(t, 1, operations.rejudgeCalls)
	var envelope struct {
		Data struct {
			SubmissionID uuid.UUID  `json:"submission_id"`
			Generation   int64      `json:"generation"`
			JobID        *uuid.UUID `json:"job_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.Equal(t, submissionID, envelope.Data.SubmissionID)
	require.Equal(t, int64(2), envelope.Data.Generation)
	require.Equal(t, &jobID, envelope.Data.JobID)
}

func TestRejudgeHTTPRejectsNonUUIDIdempotencyBeforeOperation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	operations := &httpOperationsStub{}
	handler := NewSubmissionHandlerWithOperations(nil, nil, operations)
	router := newOperationRouter(t, handler)
	request := httptest.NewRequest(
		http.MethodPost,
		"/admin/submissions/"+uuid.NewString()+"/rejudge",
		bytes.NewBufferString(`{"expected_version":1,"reason":"checker correction"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "not-a-uuid")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	require.Zero(t, operations.rejudgeCalls)
}

func newOperationRouter(t *testing.T, handler SubmissionHandler) *gin.Engine {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		now := time.Now().UTC()
		userID := uuid.New()
		principal := authentication.Principal{
			Authenticated: true, Subject: userID.String(), UserID: userID.String(),
			SessionID: uuid.NewString(), TokenID: uuid.NewString(),
			AuthenticatedAt: now.Add(-time.Minute), AuthenticatedUntil: now.Add(time.Hour), SecurityVersion: 1,
		}
		ctx := authentication.WithPrincipal(c.Request.Context(), principal)
		c.Request = c.Request.WithContext(correlation.WithContext(ctx, "operation-http-test-cid"))
		c.Next()
	})
	router.POST("/admin/submissions/:id/rejudge", handler.Rejudge)
	router.POST("/admin/submissions/:id/cancel", handler.Cancel)
	return router
}

type httpOperationsStub struct {
	rejudgeResult ports.OperationResult
	rejudgeErr    error
	cancelResult  ports.OperationResult
	cancelErr     error
	rejudgeCalls  int
	cancelCalls   int
}

func (stub *httpOperationsStub) Rejudge(context.Context, ports.OperationActor, ports.RejudgeCommand) (ports.OperationResult, error) {
	stub.rejudgeCalls++
	return stub.rejudgeResult, stub.rejudgeErr
}

func (stub *httpOperationsStub) Cancel(context.Context, ports.OperationActor, ports.CancelCommand) (ports.OperationResult, error) {
	stub.cancelCalls++
	return stub.cancelResult, stub.cancelErr
}

var _ ports.SubmissionOperations = (*httpOperationsStub)(nil)
