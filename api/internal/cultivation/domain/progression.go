package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaximumPublicProgressionBalance = int64(9_007_199_254_740_991)

var (
	ErrInvalidProgressionCommand  = errors.New("cultivation: invalid progression command")
	ErrProgressionConflict        = errors.New("cultivation: progression version conflict")
	ErrProgressionNotFound        = errors.New("cultivation: progression entry not found")
	ErrProgressionAlreadyReversed = errors.New("cultivation: progression entry already reversed")
)

type ProgressionSnapshot struct {
	UserID    uuid.UUID
	Balance   int64
	Version   int64
	UpdatedAt time.Time
}

type AdjustmentPreview struct {
	UserID          uuid.UUID
	Delta           int64
	BalanceBefore   int64
	BalanceAfter    int64
	ExpectedVersion int64
	ReversedEntryID *uuid.UUID
	Checksum        [sha256.Size]byte
}

type AdjustmentCommand struct {
	ID                      uuid.UUID
	UserID                  uuid.UUID
	Delta                   int64
	ExpectedBalance         int64
	ExpectedVersion         int64
	IncidentID              string
	Reason                  string
	PreviewChecksum         [sha256.Size]byte
	ReversedEntryID         *uuid.UUID
	RewardRuleRevisionID    *uuid.UUID
	RewardProfileRevisionID *uuid.UUID
	SourceChecksum          [sha256.Size]byte
}

type AdjustmentResult struct {
	LedgerEntryID uuid.UUID
	UserID        uuid.UUID
	Delta         int64
	BalanceBefore int64
	BalanceAfter  int64
	Version       int64
	Replayed      bool
	CreatedAt     time.Time
}

type ProgressionLedgerEntry struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Kind              string
	SourceType        string
	SourceAggregateID uuid.UUID
	SourceVersion     int64
	SourceEventID     uuid.UUID
	ReversedEntryID   *uuid.UUID
	// Reward revision identifiers and the source checksum are immutable
	// provenance for a solve-derived entry.  They are intentionally optional:
	// manual and legacy entries do not evaluate the live reward timeline.
	RewardRuleRevisionID    *uuid.UUID
	RewardProfileRevisionID *uuid.UUID
	SourceChecksum          [sha256.Size]byte
	Delta                   int64
	BalanceBefore           int64
	BalanceAfter            int64
	ProgressionVersion      int64
	IncidentID              string
	Reason                  string
	CorrelationID           string
	OccurredAt              time.Time
}

func NormalizeAdjustmentCommand(command AdjustmentCommand, maximumAbsoluteDelta int64) (AdjustmentCommand, error) {
	command.IncidentID = strings.TrimSpace(command.IncidentID)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.ID == uuid.Nil || command.UserID == uuid.Nil || command.Delta == 0 ||
		maximumAbsoluteDelta <= 0 || command.Delta < -maximumAbsoluteDelta ||
		command.Delta > maximumAbsoluteDelta || command.ExpectedBalance < 0 ||
		command.ExpectedBalance > MaximumPublicProgressionBalance ||
		command.ExpectedVersion < 0 || command.PreviewChecksum == ([sha256.Size]byte{}) ||
		!validProgressionText(command.IncidentID, 3, 128) ||
		!validProgressionText(command.Reason, 3, 1024) ||
		(command.ReversedEntryID != nil && *command.ReversedEntryID == uuid.Nil) {
		return AdjustmentCommand{}, ErrInvalidProgressionCommand
	}
	return command, nil
}

func NewAdjustmentPreview(
	snapshot ProgressionSnapshot,
	delta int64,
	reversedEntryID *uuid.UUID,
	maximumAbsoluteDelta int64,
) (AdjustmentPreview, error) {
	if snapshot.UserID == uuid.Nil || snapshot.Balance < 0 ||
		snapshot.Balance > MaximumPublicProgressionBalance || snapshot.Version < 0 ||
		delta == 0 || maximumAbsoluteDelta <= 0 || delta < -maximumAbsoluteDelta ||
		delta > maximumAbsoluteDelta ||
		(delta > 0 && snapshot.Balance > MaximumPublicProgressionBalance-delta) ||
		(delta < 0 && snapshot.Balance < -delta) ||
		(reversedEntryID != nil && *reversedEntryID == uuid.Nil) {
		return AdjustmentPreview{}, ErrInvalidProgressionCommand
	}
	preview := AdjustmentPreview{
		UserID:          snapshot.UserID,
		Delta:           delta,
		BalanceBefore:   snapshot.Balance,
		BalanceAfter:    snapshot.Balance + delta,
		ExpectedVersion: snapshot.Version,
		ReversedEntryID: reversedEntryID,
	}
	preview.Checksum = AdjustmentPreviewChecksum(preview)
	return preview, nil
}

func AdjustmentPreviewChecksum(preview AdjustmentPreview) [sha256.Size]byte {
	document := struct {
		Schema          string     `json:"schema"`
		UserID          uuid.UUID  `json:"user_id"`
		Delta           int64      `json:"delta"`
		BalanceBefore   int64      `json:"balance_before"`
		BalanceAfter    int64      `json:"balance_after"`
		ExpectedVersion int64      `json:"expected_version"`
		ReversedEntryID *uuid.UUID `json:"reversed_entry_id,omitempty"`
	}{
		Schema: "judgify.cultivation.adjustment-preview.v1",
		UserID: preview.UserID, Delta: preview.Delta,
		BalanceBefore: preview.BalanceBefore, BalanceAfter: preview.BalanceAfter,
		ExpectedVersion: preview.ExpectedVersion,
		ReversedEntryID: preview.ReversedEntryID,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(encoded)
}

func AdjustmentSourceChecksum(command AdjustmentCommand) [sha256.Size]byte {
	document := struct {
		Schema          string     `json:"schema"`
		ID              uuid.UUID  `json:"id"`
		UserID          uuid.UUID  `json:"user_id"`
		Delta           int64      `json:"delta"`
		ExpectedBalance int64      `json:"expected_balance"`
		ExpectedVersion int64      `json:"expected_version"`
		IncidentID      string     `json:"incident_id"`
		Reason          string     `json:"reason"`
		PreviewChecksum string     `json:"preview_checksum"`
		ReversedEntryID *uuid.UUID `json:"reversed_entry_id,omitempty"`
	}{
		Schema: "judgify.cultivation.manual-adjustment.v1",
		ID:     command.ID, UserID: command.UserID, Delta: command.Delta,
		ExpectedBalance: command.ExpectedBalance,
		ExpectedVersion: command.ExpectedVersion,
		IncidentID:      command.IncidentID, Reason: command.Reason,
		PreviewChecksum: checksumHex(command.PreviewChecksum),
		ReversedEntryID: command.ReversedEntryID,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(encoded)
}

func validProgressionText(value string, minimum, maximum int) bool {
	if !utf8.ValidString(value) || len(value) < minimum || len(value) > maximum {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}

func checksumHex(value [sha256.Size]byte) string {
	const digits = "0123456789abcdef"
	encoded := make([]byte, sha256.Size*2)
	for index, item := range value {
		encoded[index*2] = digits[item>>4]
		encoded[index*2+1] = digits[item&15]
	}
	return string(encoded)
}
