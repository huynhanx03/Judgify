// Package authz adapts the product's verifier and Casbin authorizer to
// Envoy's gRPC ext_authz protocol. The adapter is deliberately fail-closed:
// an unclassified route, malformed gateway identity, or unavailable policy
// engine never reaches an upstream service.
package authz

import (
	"context"
	"net/url"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
)

// CheckServer is the sole product authorization boundary called by Envoy.
type CheckServer struct {
	authv3.UnimplementedAuthorizationServer
	policies         []RoutePolicy
	verifier         authentication.Verifier
	authorizer       authorization.Authorizer
	accessCookieName string
	now              func() time.Time
}

func NewCheckServer(policies []RoutePolicy, verifier authentication.Verifier, authorizer authorization.Authorizer, accessCookieName string) *CheckServer {
	copyPolicies := append([]RoutePolicy(nil), policies...)
	return &CheckServer{policies: copyPolicies, verifier: verifier, authorizer: authorizer, accessCookieName: accessCookieName, now: time.Now}
}

func (server *CheckServer) Check(ctx context.Context, request *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	httpRequest := request.GetAttributes().GetRequest().GetHttp()
	if httpRequest == nil {
		return deny(typev3.StatusCode_Forbidden), nil
	}
	path, ok := canonicalPath(httpRequest.GetPath())
	if !ok || !validGatewayIdentity(httpRequest.GetHeaders()) {
		return deny(typev3.StatusCode_Forbidden), nil
	}
	policy, ok := Resolve(server.policies, httpRequest.GetMethod(), path)
	if !ok || !validPolicy(policy) {
		return deny(typev3.StatusCode_Forbidden), nil
	}
	if policy.Class == Public {
		return allow(), nil
	}
	credential, ok := credentialFromHeaders(httpRequest.GetHeaders(), server.accessCookieName)
	if !ok || server.verifier == nil {
		return deny(typev3.StatusCode_Unauthorized), nil
	}
	principal, err := server.verifier.Verify(ctx, credential, authentication.PurposeAccess)
	if err != nil || principal.Validate(server.now().UTC()) != nil {
		return deny(typev3.StatusCode_Unauthorized), nil
	}
	if policy.Class == Authenticated {
		return allow(), nil
	}
	if server.authorizer == nil {
		return deny(typev3.StatusCode_ServiceUnavailable), nil
	}
	decision, err := server.authorizer.Authorize(authentication.WithPrincipal(ctx, principal), authorization.Request{
		Principal: principal,
		Resource:  policy.Resource,
		Action:    policy.Action,
	})
	if err != nil {
		return deny(typev3.StatusCode_ServiceUnavailable), nil
	}
	if !decision.Allowed {
		return deny(typev3.StatusCode_Forbidden), nil
	}
	return allow(), nil
}

func validPolicy(policy RoutePolicy) bool {
	if policy.Class == Public || policy.Class == Authenticated {
		return true
	}
	return policy.Class == Protected && policy.Resource != "" && policy.Action != ""
}

func canonicalPath(raw string) (string, bool) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Path == "" || parsed.RawPath != "" {
		return "", false
	}
	return parsed.Path, safePath(parsed.Path)
}

func validGatewayIdentity(headers map[string]string) bool {
	requestID, requestOK := header(headers, "x-request-id")
	correlationID, correlationOK := header(headers, "x-correlation-id")
	if !requestOK || !correlationOK || requestID != correlationID {
		return false
	}
	parsed, err := uuid.Parse(requestID)
	return err == nil && parsed.String() == requestID
}

func credentialFromHeaders(headers map[string]string, accessCookieName string) (string, bool) {
	value, ok := header(headers, "authorization")
	if ok {
		if !strings.HasPrefix(value, "Bearer ") {
			return "", false
		}
		return boundedCredential(strings.TrimPrefix(value, "Bearer "))
	}
	if accessCookieName == "" {
		return "", false
	}
	cookies, ok := header(headers, "cookie")
	if !ok {
		return "", false
	}
	for _, cookie := range strings.Split(cookies, ";") {
		name, value, found := strings.Cut(strings.TrimSpace(cookie), "=")
		if found && name == accessCookieName {
			return boundedCredential(value)
		}
	}
	return "", false
}

func boundedCredential(credential string) (string, bool) {
	return credential, credential != "" && len(credential) <= 8192 && !strings.ContainsAny(credential, " \t\r\n")
}

func header(headers map[string]string, wanted string) (string, bool) {
	for key, value := range headers {
		if strings.EqualFold(key, wanted) {
			return value, value != ""
		}
	}
	return "", false
}

func allow() *authv3.CheckResponse {
	return &authv3.CheckResponse{Status: &statuspb.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}
}

func deny(code typev3.StatusCode) *authv3.CheckResponse {
	return &authv3.CheckResponse{Status: &statuspb.Status{Code: int32(code)}, HttpResponse: &authv3.CheckResponse_DeniedResponse{DeniedResponse: &authv3.DeniedHttpResponse{
		Status:  &typev3.HttpStatus{Code: code},
		Headers: []*corev3.HeaderValueOption{{Header: &corev3.HeaderValue{Key: "cache-control", Value: "no-store"}}},
	}}}
}
