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

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestPublishAcceptedBuildsStrictAuditedLifecycleCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contestID := uuid.New()
	commandID := uuid.New()
	now := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)
	lifecycle := &contestLifecycleServiceStub{
		result: ports.TransitionResult{
			ContestID:  contestID,
			From:       entity.ContestDraft,
			To:         entity.ContestUpcoming,
			Version:    8,
			OccurredAt: now,
			CommandID:  commandID,
		},
	}
	router := contestPublishRouter(lifecycle)
	request := httptest.NewRequest(
		http.MethodPost,
		"/contests/"+contestID.String()+"/publish",
		bytes.NewBufferString(`{"reason":"reviewed for publication"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", commandID.String())
	request.Header.Set("If-Match", `"contest-v7"`)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if lifecycle.calls != 1 ||
		lifecycle.command.ContestID != contestID ||
		lifecycle.command.Target != entity.ContestUpcoming ||
		lifecycle.command.ExpectedVersion != 7 ||
		lifecycle.command.IdempotencyKey != commandID ||
		lifecycle.command.ActorUserID == uuid.Nil ||
		lifecycle.command.ActorSessionID == nil ||
		*lifecycle.command.ActorSessionID == uuid.Nil ||
		lifecycle.command.Reason != "reviewed for publication" {
		t.Fatalf("lifecycle command = %+v", lifecycle.command)
	}
	if response.Header().Get("ETag") != `"contest-v8"` ||
		response.Header().Get(contestIdempotencyReplayHeader) != "false" ||
		response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %+v", response.Header())
	}
	var envelope struct {
		Data contestLifecycleResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.ContestID != contestID.String() ||
		envelope.Data.Version != 8 ||
		envelope.Data.CID != "contest-publish-cid" {
		t.Fatalf("lifecycle response = %+v", envelope.Data)
	}
}

func TestPublishAcceptedRejectsWeakOrDuplicateConcurrencyHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name    string
		headers func(http.Header)
	}{
		{
			name: "weak match",
			headers: func(header http.Header) {
				header.Set("Idempotency-Key", uuid.NewString())
				header.Set("If-Match", `W/"contest-v1"`)
			},
		},
		{
			name: "duplicate command",
			headers: func(header http.Header) {
				header.Add("Idempotency-Key", uuid.NewString())
				header.Add("Idempotency-Key", uuid.NewString())
				header.Set("If-Match", `"contest-v1"`)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			lifecycle := &contestLifecycleServiceStub{}
			router := contestPublishRouter(lifecycle)
			request := httptest.NewRequest(
				http.MethodPost,
				"/contests/"+uuid.NewString()+"/publish",
				bytes.NewBufferString(`{"reason":"reviewed"}`),
			)
			request.Header.Set("Content-Type", "application/json")
			test.headers(request.Header)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"status = %d, body = %s",
					response.Code,
					response.Body.String(),
				)
			}
			if lifecycle.calls != 0 {
				t.Fatalf("lifecycle calls = %d, want 0", lifecycle.calls)
			}
		})
	}
}

func TestOperatorLifecycleEndpointsBindExactTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name   string
		path   string
		target entity.ContestState
		from   entity.ContestState
	}{
		{name: "end", path: "end", target: entity.ContestEnded, from: entity.ContestRunning},
		{name: "cancel", path: "cancel", target: entity.ContestCancelled, from: entity.ContestUpcoming},
	} {
		t.Run(test.name, func(t *testing.T) {
			contestID := uuid.New()
			commandID := uuid.New()
			lifecycle := &contestLifecycleServiceStub{result: ports.TransitionResult{
				ContestID:  contestID,
				From:       test.from,
				To:         test.target,
				Version:    5,
				OccurredAt: time.Now().UTC(),
				CommandID:  commandID,
			}}
			router := contestOperatorLifecycleRouter(lifecycle)
			request := httptest.NewRequest(
				http.MethodPost,
				"/admin/contests/"+contestID.String()+"/"+test.path,
				bytes.NewBufferString(`{"reason":"operator reviewed transition"}`),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", commandID.String())
			request.Header.Set("If-Match", `"contest-v4"`)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusOK || lifecycle.calls != 1 ||
				lifecycle.command.Target != test.target ||
				lifecycle.command.ExpectedVersion != 4 {
				t.Fatalf(
					"status = %d, command = %+v, body = %s",
					response.Code,
					lifecycle.command,
					response.Body.String(),
				)
			}
		})
	}
}

func contestPublishRouter(
	lifecycle *contestLifecycleServiceStub,
) *gin.Engine {
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
		ctx = correlation.WithContext(ctx, "contest-publish-cid")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	handler := NewContestHandler(nil, lifecycle)
	router.POST(
		"/contests/:id/publish",
		handler.PublishAccepted,
	)
	return router
}

func contestOperatorLifecycleRouter(
	lifecycle *contestLifecycleServiceStub,
) *gin.Engine {
	router := contestPublishRouter(lifecycle)
	handler := NewContestHandler(nil, lifecycle)
	router.POST("/admin/contests/:id/end", handler.EndAccepted)
	router.POST("/admin/contests/:id/cancel", handler.CancelAccepted)
	return router
}

type contestLifecycleServiceStub struct {
	command ports.TransitionCommand
	result  ports.TransitionResult
	err     error
	calls   int
}

func (stub *contestLifecycleServiceStub) Transition(
	_ context.Context,
	command ports.TransitionCommand,
) (ports.TransitionResult, error) {
	stub.calls++
	stub.command = command
	return stub.result, stub.err
}
