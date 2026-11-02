// Package authctx exposes the canonical authenticated principal stored in a
// request context. Transport-specific compatibility values deliberately do
// not participate in authorization decisions.
package authctx

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/security/authentication"
)

// Principal returns a structurally valid authenticated principal. Identity's
// session authenticator additionally proves its live database state before
// middleware stores it in the context.
func Principal(ctx context.Context) (authentication.Principal, bool) {
	principal, ok := authentication.FromContext(ctx)
	now := time.Now().UTC()
	if !ok || principal.Validate(now) != nil || !principal.Authenticated ||
		principal.Subject == "" || principal.UserID == "" ||
		principal.Subject != principal.UserID || principal.SessionID == "" ||
		principal.TokenID == "" || principal.AuthenticatedAt.IsZero() ||
		principal.SecurityVersion == 0 {
		return authentication.Anonymous(), false
	}
	if !isCanonicalUUID(principal.Subject) || !isCanonicalUUID(principal.SessionID) ||
		!isCanonicalUUID(principal.TokenID) {
		return authentication.Anonymous(), false
	}
	return principal, true
}

func isCanonicalUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}

// UserID returns the canonical UUID that is both the token subject and user
// identifier. Alternate UUID spellings are rejected so audit and policy
// subjects cannot acquire multiple textual identities.
func UserID(ctx context.Context) (id uuid.UUID, ok bool) {
	principal, ok := Principal(ctx)
	if !ok {
		return uuid.Nil, false
	}
	parsed, err := uuid.Parse(principal.Subject)
	if err != nil || parsed == uuid.Nil || parsed.String() != principal.Subject {
		return uuid.Nil, false
	}
	return parsed, true
}

// SessionID returns the canonical live-session UUID carried by the principal.
func SessionID(ctx context.Context) (id uuid.UUID, ok bool) {
	principal, ok := Principal(ctx)
	if !ok || principal.SessionID == "" {
		return uuid.Nil, false
	}
	parsed, err := uuid.Parse(principal.SessionID)
	if err != nil || parsed == uuid.Nil || parsed.String() != principal.SessionID {
		return uuid.Nil, false
	}
	return parsed, true
}
