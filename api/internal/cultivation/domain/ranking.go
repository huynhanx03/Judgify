package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type RankingKind string
type RankingVisibility string

const (
	RankingByRating                    RankingKind       = "rating"
	RankingByExperience                RankingKind       = "experience"
	RankingVisibilityPublic            RankingVisibility = "public"
	RankingVisibilityAnonymous         RankingVisibility = "anonymous"
	RankingVisibilityHidden            RankingVisibility = "hidden"
	RankingReconciliationOperationKind                   = "cultivation.ranking.reconciliation.v1"
)

var (
	ErrInvalidRankingQuery                  = errors.New("cultivation: invalid ranking query")
	ErrRankingRevisionChanged               = errors.New("cultivation: ranking projection revision changed")
	ErrRankingPrivacyConflict               = errors.New("cultivation: ranking privacy version conflict")
	ErrRankingProjectionNotFound            = errors.New("cultivation: ranking projection not found")
	ErrInvalidRankingReconciliation         = errors.New("cultivation: invalid ranking reconciliation")
	ErrRankingReconciliationActive          = errors.New("cultivation: ranking reconciliation already active")
	ErrRankingReconciliationPreviewConflict = errors.New("cultivation: ranking reconciliation preview changed")
)

type RankingReconciliationPreviewCommand struct {
	ActorUserID    uuid.UUID
	ActorSessionID uuid.UUID
}

type RankingReconciliationCommand struct {
	ID               uuid.UUID
	ActorUserID      uuid.UUID
	ActorSessionID   uuid.UUID
	Reason           string
	SourceCutoff     time.Time
	ExpectedRevision int64
	SourceCount      int64
	RepairCount      int64
	PreviewChecksum  [sha256.Size]byte
}

type RankingReconciliationPlan struct {
	SourceCutoff       time.Time
	ThroughUserID      uuid.UUID
	Total              int64
	RepairCount        int64
	ProjectionRevision int64
}

type RankingReconciliationBatchQuery struct {
	SourceCutoff  time.Time
	ThroughUserID uuid.UUID
	AfterUserID   uuid.UUID
	ExpectedTotal int64
	Limit         int
}

type RankingReconciliationBatch struct {
	Processed          int64
	Repaired           int64
	LastUserID         uuid.UUID
	Done               bool
	ProjectionRevision int64
}

type RankingReconciliationState struct {
	ProjectionRevision int64
	LastReconciledAt   *time.Time
}

func RankingReconciliationPreviewChecksum(
	command RankingReconciliationPreviewCommand,
	plan RankingReconciliationPlan,
) [sha256.Size]byte {
	document := struct {
		Schema             string    `json:"schema"`
		ActorUserID        uuid.UUID `json:"actor_user_id"`
		ActorSessionID     uuid.UUID `json:"actor_session_id"`
		SourceCutoff       time.Time `json:"source_cutoff"`
		ThroughUserID      uuid.UUID `json:"through_user_id"`
		SourceCount        int64     `json:"source_count"`
		RepairCount        int64     `json:"repair_count"`
		ProjectionRevision int64     `json:"projection_revision"`
	}{
		Schema:             "judgify.cultivation.ranking-reconciliation-preview.v1",
		ActorUserID:        command.ActorUserID,
		ActorSessionID:     command.ActorSessionID,
		SourceCutoff:       plan.SourceCutoff.UTC(),
		ThroughUserID:      plan.ThroughUserID,
		SourceCount:        plan.Total,
		RepairCount:        plan.RepairCount,
		ProjectionRevision: plan.ProjectionRevision,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(encoded)
}

func RankingReconciliationPreviewMatches(
	actual [sha256.Size]byte,
	command RankingReconciliationPreviewCommand,
	plan RankingReconciliationPlan,
) bool {
	expected := RankingReconciliationPreviewChecksum(command, plan)
	return actual != ([sha256.Size]byte{}) &&
		subtle.ConstantTimeCompare(actual[:], expected[:]) == 1
}

type RankingCursor struct {
	Version            int
	Kind               RankingKind
	ProjectionRevision int64
	Score              int64
	UpdatedAt          time.Time
	PublicID           uuid.UUID
}

type RankingPrivacy struct {
	UserID             uuid.UUID
	Visibility         RankingVisibility
	Version            int64
	ProjectionRevision int64
}

type RankingPrivacyCommand struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Visibility      RankingVisibility
	ExpectedVersion int64
	Reason          string
}

type RankingPrivacyResult struct {
	Privacy            RankingPrivacy
	PreviousVisibility RankingVisibility
	Replayed           bool
	UpdatedAt          time.Time
}

type RankingProjectionEntry struct {
	Position    int64
	PublicID    uuid.UUID
	DisplayName *string
	DisplayMode string
	Rating      int
	TotalExp    int64
	UpdatedAt   time.Time
}

type RankingProjectionPage struct {
	Entries            []RankingProjectionEntry
	ProjectionRevision int64
	HasMore            bool
}

func (kind RankingKind) Valid() bool {
	return kind == RankingByRating || kind == RankingByExperience
}

func (visibility RankingVisibility) Valid() bool {
	return visibility == RankingVisibilityPublic ||
		visibility == RankingVisibilityAnonymous ||
		visibility == RankingVisibilityHidden
}

func NormalizeRankingPrivacyCommand(command RankingPrivacyCommand) (RankingPrivacyCommand, error) {
	command.Reason = strings.TrimSpace(command.Reason)
	if command.ID == uuid.Nil || command.UserID == uuid.Nil ||
		!command.Visibility.Valid() || command.ExpectedVersion < 1 ||
		!validRankingPrivacyReason(command.Reason) {
		return RankingPrivacyCommand{}, ErrInvalidRankingQuery
	}
	return command, nil
}

func RankingPrivacyRequestChecksum(command RankingPrivacyCommand) [sha256.Size]byte {
	document := struct {
		Schema          string            `json:"schema"`
		ID              uuid.UUID         `json:"id"`
		UserID          uuid.UUID         `json:"user_id"`
		Visibility      RankingVisibility `json:"visibility"`
		ExpectedVersion int64             `json:"expected_version"`
		Reason          string            `json:"reason"`
	}{
		Schema: "judgify.cultivation.ranking-privacy.v1",
		ID:     command.ID, UserID: command.UserID, Visibility: command.Visibility,
		ExpectedVersion: command.ExpectedVersion, Reason: command.Reason,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(encoded)
}

func validRankingPrivacyReason(value string) bool {
	if !utf8.ValidString(value) || len(value) < 3 || len(value) > 512 {
		return false
	}
	for _, current := range value {
		if unicode.IsControl(current) && current != '\n' && current != '\t' {
			return false
		}
	}
	return true
}

func (entry RankingProjectionEntry) Score(kind RankingKind) int64 {
	if kind == RankingByRating {
		return int64(entry.Rating)
	}
	return entry.TotalExp
}
