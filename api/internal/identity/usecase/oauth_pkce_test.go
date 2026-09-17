package usecase

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/oauth"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

func TestOAuthDurableProtectorBindsPKCEStateAndCompletionProfile(t *testing.T) {
	t.Parallel()

	protector, err := NewOAuthDurableProtector(OAuthDurableProtectorConfig{
		Secret: bytes.Repeat([]byte{0x42}, 32),
		KeyID:  "oauth-test-v1",
		Random: bytes.NewReader(bytes.Repeat([]byte{0x24}, 512)),
	})
	if err != nil {
		t.Fatalf("NewOAuthDurableProtector() error = %v", err)
	}
	now := time.Date(2026, 8, 2, 8, 0, 0, 0, time.UTC)
	preSessionID := uuid.New()
	flow, err := protector.ProtectFlow(OAuthFlowProtectionRequest{
		ID: uuid.New(), Provider: "google", Purpose: entity.OAuthPurposeLogin,
		Binding:     entity.OAuthBinding{PreSessionID: preSessionID},
		RedirectURI: "https://app.example.com/oauth/google/callback",
		IssuedAt:    now, ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("ProtectFlow() error = %v", err)
	}
	stateHash, err := protector.HashState(flow.State)
	if err != nil || !bytes.Equal(stateHash, flow.Flow.StateHash) {
		t.Fatalf("HashState() mismatch: hash=%x error=%v", stateHash, err)
	}
	verifier, err := protector.OpenPKCE(&flow.Flow, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("OpenPKCE() error = %v", err)
	}
	challenge := sha256.Sum256([]byte(verifier))
	if got := base64.RawURLEncoding.EncodeToString(challenge[:]); got != flow.PKCEChallenge {
		t.Fatalf("PKCE challenge = %q, want %q", got, flow.PKCEChallenge)
	}

	profile := oauth.Profile{
		Subject: "provider-subject-1", Email: "owner@example.com",
		EmailVerified: true, DisplayName: "Owner", Login: "owner",
	}
	completion, err := protector.ProtectCompletion(OAuthCompletionProtectionRequest{
		ID: uuid.New(), Flow: &flow.Flow, Profile: profile,
		Binding:  entity.OAuthBinding{PreSessionID: preSessionID},
		IssuedAt: now.Add(time.Minute), ExpiresAt: now.Add(9 * time.Minute),
	})
	if err != nil {
		t.Fatalf("ProtectCompletion() error = %v", err)
	}
	ticketHash, err := protector.HashTicket(completion.Ticket)
	if err != nil || !bytes.Equal(ticketHash, completion.Record.TicketHash) {
		t.Fatalf("HashTicket() mismatch: hash=%x error=%v", ticketHash, err)
	}
	opened, err := protector.OpenProfile(&completion.Record)
	if err != nil || opened.Subject != profile.Subject || opened.Email != profile.Email ||
		opened.DisplayName != profile.DisplayName || opened.Login != profile.Login {
		t.Fatalf("OpenProfile() = %#v, error=%v", opened, err)
	}

	tampered := completion.Record
	tampered.Provider = "github"
	if _, err := protector.OpenProfile(&tampered); err == nil {
		t.Fatal("OpenProfile() accepted tampered associated data")
	}
}

func TestOAuthDurableProtectorRejectsExpiredOrTamperedFlow(t *testing.T) {
	t.Parallel()

	protector, err := NewOAuthDurableProtector(OAuthDurableProtectorConfig{
		Secret: bytes.Repeat([]byte{0x33}, 32),
		KeyID:  "oauth-test-v1",
		Random: bytes.NewReader(bytes.Repeat([]byte{0x55}, 256)),
	})
	if err != nil {
		t.Fatalf("NewOAuthDurableProtector() error = %v", err)
	}
	now := time.Date(2026, 8, 2, 8, 0, 0, 0, time.UTC)
	flow, err := protector.ProtectFlow(OAuthFlowProtectionRequest{
		ID: uuid.New(), Provider: "github", Purpose: entity.OAuthPurposeLogin,
		Binding:     entity.OAuthBinding{PreSessionID: uuid.New()},
		RedirectURI: "https://app.example.com/oauth/github/callback",
		IssuedAt:    now, ExpiresAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("ProtectFlow() error = %v", err)
	}
	if _, err := protector.OpenPKCE(&flow.Flow, flow.Flow.ExpiresAt); err == nil {
		t.Fatal("OpenPKCE() accepted an expired flow")
	}
	tampered := flow.Flow
	tampered.RedirectURI = "https://app.example.com/oauth/other/callback"
	if _, err := protector.OpenPKCE(&tampered, now.Add(time.Second)); err == nil {
		t.Fatal("OpenPKCE() accepted tampered associated data")
	}
}

func TestFederatedIdentityProjectionNeverPromotesProviderEmailToRecoveryAuthority(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	authenticatedAt := time.Now().UTC()
	identity := federatedIdentityFromProfile(userID, "google", oauth.Profile{
		Subject:       "subject-1",
		Email:         "alice@example.com",
		EmailVerified: true,
		DisplayName:   "Alice",
		Login:         "alice",
		AvatarURL:     "https://images.example.com/alice.png",
	}, authenticatedAt)
	if identity.UserID != userID || identity.Provider != "google" ||
		identity.ProviderSubject != "subject-1" || identity.DisplayName == nil ||
		*identity.DisplayName != "Alice" || identity.AvatarURL == nil ||
		*identity.AvatarURL != "https://images.example.com/alice.png" ||
		identity.LastAuthenticatedAt == nil || !identity.LastAuthenticatedAt.Equal(authenticatedAt) {
		t.Fatalf("federatedIdentityFromProfile() = %#v", identity)
	}
}
