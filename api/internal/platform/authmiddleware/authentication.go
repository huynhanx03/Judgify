// Package authmiddleware binds go-common's transport-neutral security checker
// to Judgify's Gin request lifecycle.
package authmiddleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/response"
	"github.com/huynhanx03/go-common/pkg/constraints"
	"github.com/huynhanx03/go-common/pkg/security"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
)

const defaultMaxCredentialBytes = 16 << 10

// CredentialSource identifies where a verified credential was received.
type CredentialSource uint8

const (
	CredentialBearer CredentialSource = iota + 1
	CredentialCookie
)

type credentialContextKey struct{}

type authenticationConfig struct {
	sources    map[CredentialSource]struct{}
	cookieName string
	maxBytes   int
	purpose    authentication.TokenPurpose
	err        error
}

type AuthenticationOption func(*authenticationConfig)

func WithExpectedTokenPurpose(purpose authentication.TokenPurpose) AuthenticationOption {
	return func(config *authenticationConfig) {
		if err := purpose.Validate(); err != nil {
			config.err = security.ErrInvalidRequest
			return
		}
		config.purpose = purpose
	}
}

func WithCredentialSources(sources ...CredentialSource) AuthenticationOption {
	return func(config *authenticationConfig) {
		selected := make(map[CredentialSource]struct{}, len(sources))
		for _, source := range sources {
			if source != CredentialBearer && source != CredentialCookie {
				config.err = security.ErrInvalidRequest
				return
			}
			if _, exists := selected[source]; exists {
				config.err = security.ErrInvalidRequest
				return
			}
			selected[source] = struct{}{}
		}
		if len(selected) == 0 {
			config.err = security.ErrInvalidRequest
			return
		}
		config.sources = selected
	}
}

func WithCredentialCookie(name string) AuthenticationOption {
	return func(config *authenticationConfig) {
		if !validCookieName(name) {
			config.err = security.ErrInvalidRequest
			return
		}
		config.cookieName = name
	}
}

func WithMaxCredentialBytes(maximum int) AuthenticationOption {
	return func(config *authenticationConfig) {
		if maximum <= 0 || maximum > 64<<10 {
			config.err = security.ErrInvalidRequest
			return
		}
		config.maxBytes = maximum
	}
}

// CredentialSourceFromContext returns transport metadata without exposing the
// credential itself.
func CredentialSourceFromContext(ctx context.Context) (CredentialSource, bool) {
	if ctx == nil {
		return 0, false
	}
	source, ok := ctx.Value(credentialContextKey{}).(CredentialSource)
	return source, ok
}

func AuthenticationWithVerifier(verifier authentication.Verifier, options ...AuthenticationOption) gin.HandlerFunc {
	return authenticate(verifier, false, options...)
}

func OptionalAuthenticationWithVerifier(verifier authentication.Verifier, options ...AuthenticationOption) gin.HandlerFunc {
	return authenticate(verifier, true, options...)
}

func authenticate(verifier authentication.Verifier, optional bool, options ...AuthenticationOption) gin.HandlerFunc {
	config := authenticationConfig{
		sources:  map[CredentialSource]struct{}{CredentialBearer: {}},
		maxBytes: defaultMaxCredentialBytes,
		purpose:  authentication.PurposeAccess,
	}
	for _, option := range options {
		if option == nil {
			config.err = authentication.ErrInvalidConfiguration
			continue
		}
		option(&config)
	}
	if _, enabled := config.sources[CredentialCookie]; enabled && config.cookieName == "" {
		config.err = authentication.ErrInvalidConfiguration
	}
	checker := security.NewChecker(security.Config{Verifier: verifier})

	return func(c *gin.Context) {
		if config.err != nil {
			internalError(c)
			return
		}
		raw, source, present, err := extract(c.Request, config)
		if err != nil {
			unauthorized(c)
			return
		}
		if !present {
			if !optional {
				unauthorized(c)
				return
			}
			ctx := authentication.WithPrincipal(c.Request.Context(), authentication.Anonymous())
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}
		principal, err := checker.Authenticate(c.Request.Context(), raw, config.purpose)
		if err != nil {
			if c.Request.Context().Err() != nil {
				c.Abort()
				return
			}
			unauthorized(c)
			return
		}
		ctx := authentication.WithPrincipal(c.Request.Context(), principal)
		ctx = context.WithValue(ctx, credentialContextKey{}, source)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

type credential struct {
	raw    string
	source CredentialSource
}

func extract(request *http.Request, config authenticationConfig) (string, CredentialSource, bool, error) {
	var found []credential
	authorizationValues := request.Header.Values(constraints.HeaderAuthorization)
	if len(authorizationValues) > 1 {
		return "", 0, false, authentication.ErrInvalidToken
	}
	if len(authorizationValues) == 1 {
		if _, enabled := config.sources[CredentialBearer]; !enabled {
			return "", 0, false, authentication.ErrInvalidToken
		}
		parts := strings.Fields(authorizationValues[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], constraints.TokenTypeBearer) || !validCredential(parts[1], config.maxBytes) {
			return "", 0, false, authentication.ErrInvalidToken
		}
		found = append(found, credential{raw: parts[1], source: CredentialBearer})
	}
	if config.cookieName != "" {
		cookies := request.CookiesNamed(config.cookieName)
		if len(cookies) > 1 {
			return "", 0, false, authentication.ErrInvalidToken
		}
		if len(cookies) == 1 {
			if _, enabled := config.sources[CredentialCookie]; !enabled || !validCredential(cookies[0].Value, config.maxBytes) {
				return "", 0, false, authentication.ErrInvalidToken
			}
			found = append(found, credential{raw: cookies[0].Value, source: CredentialCookie})
		}
	}
	if len(found) > 1 {
		return "", 0, false, authentication.ErrInvalidToken
	}
	if len(found) == 0 {
		return "", 0, false, nil
	}
	return found[0].raw, found[0].source, true, nil
}

func validCredential(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validCookieName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			strings.ContainsRune("!#$%&'*+-.^_|~", character) {
			continue
		}
		return false
	}
	return true
}

func unauthorized(c *gin.Context) {
	response.ErrorResponse(c, apperr.CodeUnauthorized, apperr.New(apperr.CodeUnauthorized, "unauthorized", nil))
	c.Abort()
}

func internalError(c *gin.Context) {
	response.ErrorResponse(c, apperr.CodeInternalServer, apperr.New(apperr.CodeInternalServer, "internal server error", nil))
	c.Abort()
}

// AuthorizationChecker exposes route-level resource/action checks to Gin.
type AuthorizationChecker struct {
	checker *security.Checker
}

type PermissionRequirement struct {
	Resource string
	Action   string
}

func NewAuthorizationChecker(authorizer authorization.Authorizer) *AuthorizationChecker {
	return &AuthorizationChecker{checker: security.NewChecker(security.Config{Authorizer: authorizer})}
}

func (checker *AuthorizationChecker) RequirePermission(resource, action string) gin.HandlerFunc {
	return checker.RequireAnyPermission(PermissionRequirement{Resource: resource, Action: action})
}

func (checker *AuthorizationChecker) RequireAnyPermission(requirements ...PermissionRequirement) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if checker == nil || checker.checker == nil || len(requirements) == 0 {
			internalError(c)
			return
		}
		principal, err := checker.checker.Principal(ctx)
		if err != nil {
			if errors.Is(err, security.ErrUnauthenticated) {
				unauthorized(c)
			} else if ctx.Err() != nil {
				c.Abort()
			} else {
				internalError(c)
			}
			return
		}
		permissions := make([]security.Permission, len(requirements))
		for index, requirement := range requirements {
			permissions[index] = security.Permission{Resource: requirement.Resource, Action: requirement.Action}
		}
		_, err = checker.checker.Authorize(ctx, principal, permissions...)
		if err == nil {
			c.Next()
			return
		}
		if errors.Is(err, security.ErrForbidden) {
			response.ErrorResponse(c, apperr.CodeForbidden, apperr.New(apperr.CodeForbidden, "forbidden", nil))
			c.Abort()
			return
		}
		if ctx.Err() != nil {
			c.Abort()
			return
		}
		internalError(c)
	}
}
