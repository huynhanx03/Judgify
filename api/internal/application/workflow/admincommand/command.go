package admincommand

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	minimumCommandReasonBytes = 3
	maximumCommandReasonBytes = 1024
)

var ErrInvalidCommandMeta = errors.New("admin command: invalid metadata")

// CommandMeta is assembled at the transport/application boundary from the
// authenticated principal plus explicit command evidence. Actor and session
// identities are never accepted from a request body.
type CommandMeta struct {
	ActorID           uuid.UUID
	SessionID         uuid.UUID
	ExpectedVersion   int64
	IdempotencyKey    uuid.UUID
	Reason            string
	ConfirmationToken string
}

type CommandMetaPolicy struct {
	RequireIdempotency  bool
	RequireConfirmation bool
}

func NormalizeCommandMeta(
	meta CommandMeta,
	policy CommandMetaPolicy,
) (CommandMeta, error) {
	meta.Reason = strings.TrimSpace(meta.Reason)
	if meta.ActorID == uuid.Nil || meta.SessionID == uuid.Nil ||
		meta.ExpectedVersion < 1 || !validCommandReason(meta.Reason) ||
		(policy.RequireIdempotency && meta.IdempotencyKey == uuid.Nil) ||
		(policy.RequireConfirmation &&
			(meta.ConfirmationToken == "" ||
				len(meta.ConfirmationToken) > maximumConfirmationTokenBytes)) {
		return CommandMeta{}, ErrInvalidCommandMeta
	}
	if !policy.RequireConfirmation {
		meta.ConfirmationToken = ""
	}
	if !policy.RequireIdempotency {
		meta.IdempotencyKey = uuid.Nil
	}
	return meta, nil
}

func validCommandReason(value string) bool {
	if !utf8.ValidString(value) || len(value) < minimumCommandReasonBytes ||
		len(value) > maximumCommandReasonBytes {
		return false
	}
	for _, current := range value {
		if unicode.IsControl(current) && current != '\n' && current != '\t' {
			return false
		}
	}
	return true
}
