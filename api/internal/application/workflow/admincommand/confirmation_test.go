package admincommand

import (
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestConfirmationProtectorBindsExactAdministrativeRequest(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 2, 8, 0, 0, 0, time.UTC)
	protector, err := NewConfirmationProtector(
		[]byte("0123456789abcdef0123456789abcdef"),
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("NewConfirmationProtector() error = %v", err)
	}
	protector.now = func() time.Time { return now }
	expected := validConfirmationExpectation()

	token, issued, err := protector.IssueWithClaims(expected)
	if err != nil {
		t.Fatalf("IssueWithClaims() error = %v", err)
	}
	if issued.ConfirmationExpectation != expected ||
		!issued.IssuedAt.Equal(now) ||
		!issued.ExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("IssueWithClaims() claims = %#v", issued)
	}
	claims, err := protector.Verify(token, expected)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.ActorID != expected.ActorID ||
		claims.SessionID != expected.SessionID ||
		claims.IdempotencyKey != expected.IdempotencyKey ||
		claims.Action != expected.Action ||
		claims.Resource != expected.Resource ||
		claims.TargetID != expected.TargetID ||
		claims.ExpectedVersion != expected.ExpectedVersion ||
		claims.RequestHash != expected.RequestHash ||
		!claims.IssuedAt.Equal(now) ||
		!claims.ExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("Verify() claims = %#v, want exact request binding", claims)
	}

	mutations := map[string]func(*ConfirmationExpectation){
		"actor":       func(value *ConfirmationExpectation) { value.ActorID = uuid.New() },
		"session":     func(value *ConfirmationExpectation) { value.SessionID = uuid.New() },
		"idempotency": func(value *ConfirmationExpectation) { value.IdempotencyKey = uuid.New() },
		"action":      func(value *ConfirmationExpectation) { value.Action = "material.archive" },
		"resource":    func(value *ConfirmationExpectation) { value.Resource = "problem" },
		"target":      func(value *ConfirmationExpectation) { value.TargetID = uuid.NewString() },
		"version":     func(value *ConfirmationExpectation) { value.ExpectedVersion++ },
		"request": func(value *ConfirmationExpectation) {
			value.RequestHash = sha256.Sum256([]byte("changed request"))
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := expected
			mutate(&changed)
			if _, err := protector.Verify(token, changed); !errors.Is(
				err,
				ErrConfirmationMismatch,
			) {
				t.Fatalf("Verify(changed %s) error = %v", name, err)
			}
		})
	}
}

func TestConfirmationProtectorRejectsTamperExpiryAndMalformedInputs(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 2, 8, 0, 0, 0, time.UTC)
	protector, err := NewConfirmationProtector(
		[]byte("0123456789abcdef0123456789abcdef"),
		time.Minute,
	)
	if err != nil {
		t.Fatalf("NewConfirmationProtector() error = %v", err)
	}
	protector.now = func() time.Time { return now }
	expected := validConfirmationExpectation()
	token, err := protector.Issue(expected)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	tampered := token[:len(token)-1] + "A"
	if tampered == token {
		tampered = token[:len(token)-1] + "B"
	}
	if _, err := protector.Verify(tampered, expected); !errors.Is(
		err,
		ErrInvalidConfirmation,
	) {
		t.Fatalf("Verify(tampered) error = %v", err)
	}

	protector.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := protector.Verify(token, expected); !errors.Is(
		err,
		ErrConfirmationExpired,
	) {
		t.Fatalf("Verify(expired) error = %v", err)
	}

	invalid := expected
	invalid.Action = " Material Publish "
	if _, err := protector.Issue(invalid); !errors.Is(
		err,
		ErrInvalidConfirmation,
	) {
		t.Fatalf("Issue(invalid) error = %v", err)
	}
	if _, err := protector.Verify("", expected); !errors.Is(
		err,
		ErrInvalidConfirmation,
	) {
		t.Fatalf("Verify(empty) error = %v", err)
	}
}

func TestConfirmationRequestHashIsDomainSeparatedAndBounded(t *testing.T) {
	t.Parallel()
	first, err := ConfirmationRequestHash(
		"material.publish.v1",
		[]byte(`{"material_id":"f950cf1f-a851-4bb3-a076-c45855014960"}`),
	)
	if err != nil {
		t.Fatalf("ConfirmationRequestHash(first) error = %v", err)
	}
	second, err := ConfirmationRequestHash(
		"material.archive.v1",
		[]byte(`{"material_id":"f950cf1f-a851-4bb3-a076-c45855014960"}`),
	)
	if err != nil {
		t.Fatalf("ConfirmationRequestHash(second) error = %v", err)
	}
	if first == second || first == ([sha256.Size]byte{}) {
		t.Fatal("ConfirmationRequestHash() is not domain separated")
	}
	if _, err := ConfirmationRequestHash(
		"material.publish.v1",
		make([]byte, maximumConfirmationRequestBytes+1),
	); !errors.Is(err, ErrInvalidConfirmation) {
		t.Fatalf("ConfirmationRequestHash(oversize) error = %v", err)
	}
}

func validConfirmationExpectation() ConfirmationExpectation {
	return ConfirmationExpectation{
		ActorID:         uuid.New(),
		SessionID:       uuid.New(),
		IdempotencyKey:  uuid.New(),
		Action:          "material.publish",
		Resource:        "material",
		TargetID:        uuid.NewString(),
		ExpectedVersion: 7,
		RequestHash:     sha256.Sum256([]byte("exact request")),
	}
}
