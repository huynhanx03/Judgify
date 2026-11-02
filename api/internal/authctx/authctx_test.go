package authctx

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/constraints"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
)

func TestCanonicalPrincipalContext(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()
	principal := authentication.Principal{
		Authenticated:      true,
		Subject:            userID.String(),
		UserID:             userID.String(),
		SessionID:          sessionID.String(),
		TokenID:            uuid.NewString(),
		AuthenticatedAt:    time.Now().Add(-time.Minute),
		SecurityVersion:    1,
		AuthenticatedUntil: time.Now().Add(time.Minute),
	}
	ctx := authentication.WithPrincipal(context.Background(), principal)
	if got, ok := UserID(ctx); !ok || got != userID {
		t.Fatalf("UserID() = %s, %t", got, ok)
	}
	if got, ok := SessionID(ctx); !ok || got != sessionID {
		t.Fatalf("SessionID() = %s, %t", got, ok)
	}
}

func TestAuthContextRejectsLegacyOrAmbiguousIdentity(t *testing.T) {
	legacy := context.WithValue(context.Background(), constraints.ContextKeyUserID, uuid.NewString())
	if _, ok := UserID(legacy); ok {
		t.Fatal("legacy transport context became authentication authority")
	}

	userID := uuid.NewString()
	principal := authentication.Principal{Authenticated: true, Subject: userID, UserID: uuid.NewString()}
	if _, ok := Principal(authentication.WithPrincipal(context.Background(), principal)); ok {
		t.Fatal("subject/user mismatch accepted")
	}

	parsed := uuid.New()
	nonCanonical := "{" + parsed.String() + "}"
	principal = authentication.Principal{Authenticated: true, Subject: nonCanonical, UserID: nonCanonical}
	if _, ok := UserID(authentication.WithPrincipal(context.Background(), principal)); ok {
		t.Fatal("non-canonical UUID accepted")
	}
}

func TestAuthContextRequiresCompleteLiveSessionPrincipal(t *testing.T) {
	now := time.Now().UTC()
	valid := authentication.Principal{
		Authenticated:      true,
		Subject:            uuid.NewString(),
		SessionID:          uuid.NewString(),
		TokenID:            uuid.NewString(),
		AuthenticatedAt:    now.Add(-time.Minute),
		SecurityVersion:    1,
		AuthenticatedUntil: now.Add(time.Minute),
	}
	valid.UserID = valid.Subject
	tests := []struct {
		name   string
		mutate func(*authentication.Principal)
	}{
		{name: "missing session", mutate: func(principal *authentication.Principal) { principal.SessionID = "" }},
		{name: "missing token ID", mutate: func(principal *authentication.Principal) { principal.TokenID = "" }},
		{name: "missing auth time", mutate: func(principal *authentication.Principal) { principal.AuthenticatedAt = time.Time{} }},
		{name: "missing security version", mutate: func(principal *authentication.Principal) { principal.SecurityVersion = 0 }},
		{name: "expired boundary", mutate: func(principal *authentication.Principal) { principal.AuthenticatedUntil = now.Add(-time.Minute) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			principal := valid
			test.mutate(&principal)
			if _, ok := Principal(authentication.WithPrincipal(context.Background(), principal)); ok {
				t.Fatal("incomplete session principal accepted")
			}
		})
	}
}
