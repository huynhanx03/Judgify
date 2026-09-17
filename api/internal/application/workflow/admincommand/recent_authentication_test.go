package admincommand

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
)

func TestRequireRecentAuthenticationBindsLiveActorAndSession(t *testing.T) {
	now := time.Now().UTC()
	actorID := uuid.New()
	sessionID := uuid.New()
	principal := authentication.Principal{
		Authenticated:      true,
		Subject:            actorID.String(),
		UserID:             actorID.String(),
		SessionID:          sessionID.String(),
		TokenID:            uuid.NewString(),
		AuthenticatedAt:    now.Add(-time.Minute),
		AuthenticatedUntil: now.Add(time.Hour),
		SecurityVersion:    1,
	}
	ctx := authentication.WithPrincipal(context.Background(), principal)
	if err := RequireRecentAuthentication(
		ctx, actorID, sessionID, now, 10*time.Minute,
	); err != nil {
		t.Fatalf("RequireRecentAuthentication() error = %v", err)
	}
	if err := RequireRecentAuthentication(
		ctx, actorID, uuid.New(), now, 10*time.Minute,
	); !errors.Is(err, ErrRecentAuthenticationRequired) {
		t.Fatalf("session mismatch error = %v", err)
	}
	if err := RequireRecentAuthentication(
		ctx, actorID, sessionID, now.Add(20*time.Minute), 10*time.Minute,
	); !errors.Is(err, ErrRecentAuthenticationRequired) {
		t.Fatalf("stale authentication error = %v", err)
	}
}
