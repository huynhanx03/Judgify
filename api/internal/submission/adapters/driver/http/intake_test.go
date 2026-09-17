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

	contract "github.com/huynhanx03/judgify/internal/submission/contract"
)

func TestSubmitAcceptedRequiresOneIdempotencyKeyAndReturnsCommittedSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	submissionID := uuid.New()
	commandService := &submissionCommandServiceStub{
		response: &contract.SubmissionAcceptedResponse{
			ID:                    submissionID,
			AggregateVersion:      1,
			LatestGeneration:      1,
			LatestGenerationPhase: "queued",
			SubmittedAt: time.Date(
				2026,
				time.July,
				28,
				10,
				0,
				0,
				0,
				time.UTC,
			),
			CID:              "http-test-cid",
			IdempotentReplay: true,
		},
	}
	router := submissionIntakeRouter(t, commandService)
	body := []byte(`{
		"problem_id":"` + uuid.NewString() + `",
		"runtime_key":"go",
		"language":"cpp",
		"source_code":"package main\nfunc main() {}"
	}`)
	request := httptest.NewRequest(
		http.MethodPost,
		"/submissions",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "http-test-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") !=
		"/submissions/"+submissionID.String() ||
		response.Header().Get(idempotencyReplayHeader) != "true" ||
		response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("response headers = %+v", response.Header())
	}
	if commandService.calls != 1 ||
		commandService.idempotencyKey != "http-test-key" ||
		commandService.request == nil ||
		commandService.request.RuntimeKey != "go" {
		t.Fatalf("command service call = %+v", commandService)
	}
	var envelope struct {
		Data submissionAcceptedResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.ID != submissionID.String() ||
		!envelope.Data.IdempotentReplay {
		t.Fatalf("response data = %+v", envelope.Data)
	}
}

func TestSubmitAcceptedRejectsDuplicateIdempotencyHeaderBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	commandService := &submissionCommandServiceStub{}
	router := submissionIntakeRouter(t, commandService)
	request := httptest.NewRequest(
		http.MethodPost,
		"/submissions",
		bytes.NewBufferString(`{
			"problem_id":"`+uuid.NewString()+`",
			"runtime_key":"go",
			"source_code":"package main"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("Idempotency-Key", "first")
	request.Header.Add("Idempotency-Key", "second")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if commandService.calls != 0 {
		t.Fatalf("command service calls = %d, want 0", commandService.calls)
	}
}

func submissionIntakeRouter(
	t *testing.T,
	commandService *submissionCommandServiceStub,
) *gin.Engine {
	t.Helper()
	handler := NewSubmissionHandler(nil, commandService)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		now := time.Now().UTC()
		userID := uuid.New()
		principal := authentication.Principal{
			Authenticated:      true,
			Subject:            userID.String(),
			UserID:             userID.String(),
			SessionID:          uuid.NewString(),
			TokenID:            uuid.NewString(),
			AuthenticatedAt:    now.Add(-time.Minute),
			AuthenticatedUntil: now.Add(time.Hour),
			SecurityVersion:    1,
		}
		ctx := authentication.WithPrincipal(
			c.Request.Context(),
			principal,
		)
		ctx = correlation.WithContext(ctx, "http-test-cid")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/submissions", handler.SubmitAccepted)
	return router
}

type submissionCommandServiceStub struct {
	response       *contract.SubmissionAcceptedResponse
	err            error
	request        *contract.CreateSubmissionCommand
	idempotencyKey string
	userID         uuid.UUID
	calls          int
}

func (stub *submissionCommandServiceStub) Submit(
	_ context.Context,
	userID uuid.UUID,
	idempotencyKey string,
	command *contract.CreateSubmissionCommand,
) (*contract.SubmissionAcceptedResponse, error) {
	stub.calls++
	stub.userID = userID
	stub.idempotencyKey = idempotencyKey
	stub.request = command
	return stub.response, stub.err
}
