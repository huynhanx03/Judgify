package authz

import (
	"context"
	"errors"
	"testing"
	"time"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
)

type verifierFunc func(context.Context, string, authentication.TokenPurpose) (authentication.Principal, error)

func (fn verifierFunc) Verify(ctx context.Context, raw string, purpose authentication.TokenPurpose) (authentication.Principal, error) {
	return fn(ctx, raw, purpose)
}

type authorizerFunc func(context.Context, authorization.Request) (authorization.Decision, error)

func (fn authorizerFunc) Authorize(ctx context.Context, request authorization.Request) (authorization.Decision, error) {
	return fn(ctx, request)
}

func TestCheckMapsMissingCredentialToUnauthorized(t *testing.T) {
	server := NewCheckServer([]RoutePolicy{{Method: "GET", Pattern: "/private", Class: Protected, Resource: "problem", Action: "read"}}, verifierFunc(nil), authorizerFunc(nil), "access")
	response, err := server.Check(context.Background(), checkRequest("GET", "/private", ""))
	if err != nil || response.GetDeniedResponse().GetStatus().GetCode() != 401 {
		t.Fatalf("response=%#v err=%v, want 401", response, err)
	}
}

func TestCheckMapsAuthorizationOutcomes(t *testing.T) {
	principal := authentication.Principal{Authenticated: true, Subject: "user", UserID: "user", SessionID: "session", TokenID: "token", AuthenticatedAt: time.Now().UTC(), AuthenticatedUntil: time.Now().UTC().Add(time.Hour)}
	policies := []RoutePolicy{{Method: "GET", Pattern: "/private", Class: Protected, Resource: "problem", Action: "read"}}
	for name, authorize := range map[string]struct {
		decision authorization.Decision
		err      error
		want     int
	}{
		"denied": {decision: authorization.Decision{Allowed: false}, want: 403},
		"failed": {err: errors.New("policy unavailable"), want: 503},
	} {
		t.Run(name, func(t *testing.T) {
			server := NewCheckServer(policies, verifierFunc(func(context.Context, string, authentication.TokenPurpose) (authentication.Principal, error) {
				return principal, nil
			}), authorizerFunc(func(context.Context, authorization.Request) (authorization.Decision, error) {
				return authorize.decision, authorize.err
			}), "access")
			response, err := server.Check(context.Background(), checkRequest("GET", "/private", "Bearer valid"))
			if err != nil || int(response.GetDeniedResponse().GetStatus().GetCode()) != authorize.want {
				t.Fatalf("response=%#v err=%v, want %d", response, err, authorize.want)
			}
		})
	}
}

func TestCheckAcceptsConfiguredAccessCookie(t *testing.T) {
	principal := authentication.Principal{Authenticated: true, Subject: "user", AuthenticatedUntil: time.Now().UTC().Add(time.Hour)}
	server := NewCheckServer(
		[]RoutePolicy{{Method: "GET", Pattern: "/private", Class: Authenticated}},
		verifierFunc(func(_ context.Context, credential string, _ authentication.TokenPurpose) (authentication.Principal, error) {
			if credential != "cookie-token" {
				t.Fatalf("credential=%q, want cookie-token", credential)
			}
			return principal, nil
		}),
		authorizerFunc(nil),
		"judgify_access",
	)
	request := checkRequest("GET", "/private", "")
	request.Attributes.Request.Http.Headers["cookie"] = "other=value; judgify_access=cookie-token"
	response, err := server.Check(context.Background(), request)
	if err != nil || response.GetOkResponse() == nil {
		t.Fatalf("response=%#v err=%v, want allowed", response, err)
	}
}

func checkRequest(method, path, credential string) *authv3.CheckRequest {
	headers := map[string]string{"x-request-id": "019f6abb-8dd5-7581-8449-2b9ad77873e5", "x-correlation-id": "019f6abb-8dd5-7581-8449-2b9ad77873e5"}
	if credential != "" {
		headers["authorization"] = credential
	}
	return &authv3.CheckRequest{Attributes: &authv3.AttributeContext{Request: &authv3.AttributeContext_Request{Http: &authv3.AttributeContext_HttpRequest{Method: method, Path: path, Headers: headers}}}}
}
