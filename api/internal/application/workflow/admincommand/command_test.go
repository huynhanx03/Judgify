package admincommand

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeCommandMetaRequiresServerIdentityVersionReasonAndCommandEvidence(t *testing.T) {
	t.Parallel()
	input := CommandMeta{
		ActorID: uuid.New(), SessionID: uuid.New(), ExpectedVersion: 4,
		IdempotencyKey: uuid.New(), Reason: "  Incident INC-42 verified  ",
		ConfirmationToken: "v1.signed.confirmation",
	}
	normalized, err := NormalizeCommandMeta(input, CommandMetaPolicy{
		RequireIdempotency:  true,
		RequireConfirmation: true,
	})
	if err != nil {
		t.Fatalf("NormalizeCommandMeta() error = %v", err)
	}
	if normalized.Reason != "Incident INC-42 verified" ||
		normalized.ActorID != input.ActorID ||
		normalized.SessionID != input.SessionID ||
		normalized.ExpectedVersion != input.ExpectedVersion ||
		normalized.IdempotencyKey != input.IdempotencyKey ||
		normalized.ConfirmationToken != input.ConfirmationToken {
		t.Fatalf("NormalizeCommandMeta() = %#v", normalized)
	}

	invalid := []CommandMeta{
		{},
		{ActorID: input.ActorID, SessionID: input.SessionID, ExpectedVersion: 4},
		{ActorID: input.ActorID, SessionID: input.SessionID, ExpectedVersion: 4, IdempotencyKey: uuid.New(), Reason: "ok", ConfirmationToken: input.ConfirmationToken},
		{ActorID: input.ActorID, SessionID: input.SessionID, ExpectedVersion: 4, IdempotencyKey: uuid.New(), Reason: "bad\x00reason", ConfirmationToken: input.ConfirmationToken},
	}
	for index, value := range invalid {
		if _, err := NormalizeCommandMeta(value, CommandMetaPolicy{
			RequireIdempotency:  true,
			RequireConfirmation: true,
		}); !errors.Is(err, ErrInvalidCommandMeta) {
			t.Fatalf("case %d error = %v", index, err)
		}
	}
}
