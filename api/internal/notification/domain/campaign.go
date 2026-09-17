package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
)

type AudienceSelector struct {
	Kind    string
	RoleID  *uuid.UUID
	UserIDs []uuid.UUID
}

func (selector AudienceSelector) Validate() error {
	if !contract.ValidAudienceKind(selector.Kind) {
		return contract.ErrInvalidCampaign
	}
	switch selector.Kind {
	case contract.AudienceExplicitUsers:
		if selector.RoleID != nil || len(selector.UserIDs) == 0 ||
			len(selector.UserIDs) > contract.MaximumExplicitRecipients {
			return contract.ErrInvalidCampaign
		}
	case contract.AudienceRole:
		if selector.RoleID == nil || *selector.RoleID == uuid.Nil || len(selector.UserIDs) != 0 {
			return contract.ErrInvalidCampaign
		}
	case contract.AudienceAllActive:
		if selector.RoleID != nil || len(selector.UserIDs) != 0 {
			return contract.ErrInvalidCampaign
		}
	default:
		return contract.ErrInvalidCampaign
	}
	if !canonicalUUIDs(selector.UserIDs) {
		return contract.ErrInvalidCampaign
	}
	return nil
}

func CanonicalAudience(kind string, roleID *uuid.UUID, userIDs []uuid.UUID) (AudienceSelector, error) {
	selector := AudienceSelector{
		Kind:    kind,
		RoleID:  cloneCampaignUUID(roleID),
		UserIDs: append([]uuid.UUID(nil), userIDs...),
	}
	slices.SortFunc(selector.UserIDs, func(left, right uuid.UUID) int {
		return bytes.Compare(left[:], right[:])
	})
	selector.UserIDs = slices.Compact(selector.UserIDs)
	if err := selector.Validate(); err != nil {
		return AudienceSelector{}, err
	}
	return selector, nil
}

type Campaign struct {
	ID                                uuid.UUID
	Status                            string
	CurrentDraftRevisionID            uuid.UUID
	ScheduledRevisionID               *uuid.UUID
	ScheduledRenderSourceID           *uuid.UUID
	ScheduledRenderSourceKind         *string
	ScheduledNormalizedSourceChecksum []byte
	ScheduledRenderArtifactID         *uuid.UUID
	OperationID                       *uuid.UUID
	Version                           int64
	ScheduledFor                      *time.Time
	LastFailureCode                   *string
	CreatedBy                         uuid.UUID
	ScheduledBy                       *uuid.UUID
	CancelledBy                       *uuid.UUID
	ScheduleReason                    *string
	CancelReason                      *string
	CreatedAt                         time.Time
	UpdatedAt                         time.Time
	ScheduledAt                       *time.Time
	StartedAt                         *time.Time
	CompletedAt                       *time.Time
	CancelledAt                       *time.Time
}

func (campaign Campaign) Validate() error {
	if campaign.ID == uuid.Nil || campaign.CurrentDraftRevisionID == uuid.Nil ||
		campaign.CreatedBy == uuid.Nil || campaign.Version < 1 ||
		!contract.ValidCampaignStatus(campaign.Status) ||
		campaign.CreatedAt.IsZero() || campaign.UpdatedAt.IsZero() {
		return contract.ErrInvalidCampaign
	}
	cancellationEvidence := 0
	if campaign.CancelledBy != nil {
		cancellationEvidence++
	}
	if campaign.CancelReason != nil {
		cancellationEvidence++
	}
	if campaign.CancelledAt != nil {
		cancellationEvidence++
	}
	if (cancellationEvidence != 0 && cancellationEvidence != 3) ||
		(cancellationEvidence == 3 &&
			(campaign.Status != contract.CampaignStatusCancelled ||
				*campaign.CancelledBy == uuid.Nil ||
				!contract.ValidCampaignReason(*campaign.CancelReason) ||
				campaign.CancelledAt.IsZero())) {
		return contract.ErrInvalidCampaign
	}
	if campaign.Status == contract.CampaignStatusDraft {
		if campaign.ScheduledRevisionID != nil || campaign.OperationID != nil ||
			campaign.ScheduledAt != nil {
			return contract.ErrInvalidCampaign
		}
		return nil
	}
	if campaign.ScheduledRevisionID == nil || *campaign.ScheduledRevisionID == uuid.Nil ||
		campaign.ScheduledRenderSourceID == nil || *campaign.ScheduledRenderSourceID == uuid.Nil ||
		campaign.ScheduledRenderSourceKind == nil ||
		*campaign.ScheduledRenderSourceKind != "notification_campaign_body" ||
		len(campaign.ScheduledNormalizedSourceChecksum) != sha256.Size ||
		campaign.ScheduledRenderArtifactID == nil || *campaign.ScheduledRenderArtifactID == uuid.Nil ||
		campaign.OperationID == nil || *campaign.OperationID == uuid.Nil ||
		campaign.ScheduledBy == nil || *campaign.ScheduledBy == uuid.Nil ||
		campaign.ScheduleReason == nil || !contract.ValidCampaignReason(*campaign.ScheduleReason) ||
		campaign.ScheduledAt == nil || campaign.ScheduledAt.IsZero() {
		return contract.ErrInvalidCampaign
	}
	return nil
}

type CampaignRevision struct {
	ID                       uuid.UUID
	CampaignID               uuid.UUID
	RevisionNumber           int64
	Title                    string
	BodyMarkdown             string
	BodyRenderSourceID       uuid.UUID
	BodyRenderSourceKind     string
	BodyRenderArtifactID     uuid.UUID
	SanitizedHTML            string
	ActionPath               *string
	Audience                 AudienceSelector
	NormalizedSourceChecksum [sha256.Size]byte
	AuthorID                 uuid.UUID
	CreatedAt                time.Time
}

func (revision CampaignRevision) Validate() error {
	if revision.ID == uuid.Nil || revision.CampaignID == uuid.Nil ||
		revision.RevisionNumber < 1 || revision.BodyRenderSourceID == uuid.Nil ||
		revision.BodyRenderArtifactID == uuid.Nil ||
		revision.BodyRenderSourceKind != "notification_campaign_body" ||
		revision.NormalizedSourceChecksum == ([sha256.Size]byte{}) ||
		revision.AuthorID == uuid.Nil || revision.CreatedAt.IsZero() ||
		!contract.ValidCampaignTitle(revision.Title) ||
		!contract.ValidCampaignBody(revision.BodyMarkdown) ||
		revision.SanitizedHTML == "" || revision.Audience.Validate() != nil {
		return contract.ErrInvalidCampaign
	}
	if revision.ActionPath != nil && contract.ValidateActionPath(*revision.ActionPath) != nil {
		return contract.ErrInvalidCampaign
	}
	return nil
}

type CampaignView struct {
	Campaign Campaign
	Revision CampaignRevision
	Counts   RecipientCounts
}

type RecipientCounts struct {
	Total   int64
	Pending int64
	Created int64
	Skipped int64
	Failed  int64
}

func (counts RecipientCounts) Validate() error {
	if counts.Total < 0 || counts.Pending < 0 || counts.Created < 0 ||
		counts.Skipped < 0 || counts.Failed < 0 ||
		counts.Total != counts.Pending+counts.Created+counts.Skipped+counts.Failed {
		return contract.ErrInvalidCampaign
	}
	return nil
}

type AudienceMember struct {
	UserID   uuid.UUID
	Username string
}

func (member AudienceMember) Validate() error {
	if member.UserID == uuid.Nil || member.Username == "" ||
		len(member.Username) > 64 || strings.TrimSpace(member.Username) != member.Username {
		return contract.ErrInvalidCampaign
	}
	return nil
}

type AudienceSnapshot struct {
	Members     []AudienceMember
	Fingerprint [sha256.Size]byte
	EstimatedAt time.Time
}

func NewAudienceSnapshot(members []AudienceMember, estimatedAt time.Time) (AudienceSnapshot, error) {
	if estimatedAt.IsZero() {
		return AudienceSnapshot{}, contract.ErrInvalidCampaign
	}
	cloned := append([]AudienceMember(nil), members...)
	slices.SortFunc(cloned, func(left, right AudienceMember) int {
		return bytes.Compare(left.UserID[:], right.UserID[:])
	})
	for index, member := range cloned {
		if member.Validate() != nil || (index > 0 && member.UserID == cloned[index-1].UserID) {
			return AudienceSnapshot{}, contract.ErrInvalidCampaign
		}
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte("judgify.notification-campaign-audience.v1"))
	var length [8]byte
	for _, member := range cloned {
		binary.BigEndian.PutUint64(length[:], uint64(len(member.UserID)))
		_, _ = digest.Write(length[:])
		_, _ = digest.Write(member.UserID[:])
	}
	var fingerprint [sha256.Size]byte
	copy(fingerprint[:], digest.Sum(nil))
	return AudienceSnapshot{
		Members: cloned, Fingerprint: fingerprint, EstimatedAt: estimatedAt.UTC(),
	}, nil
}

type CampaignConfirmation struct {
	ID                       uuid.UUID
	NonceHash                [sha256.Size]byte
	ActorID                  uuid.UUID
	SessionID                uuid.UUID
	CampaignID               uuid.UUID
	CampaignRevisionID       uuid.UUID
	RenderSourceID           uuid.UUID
	RenderSourceKind         string
	NormalizedSourceChecksum [sha256.Size]byte
	RenderArtifactID         uuid.UUID
	AudienceFingerprint      [sha256.Size]byte
	ExpectedCampaignVersion  int64
	ScheduledFor             *time.Time
	DryRunEstimatedAt        time.Time
	DryRunRecipientCount     int64
	IssuedAt                 time.Time
	ExpiresAt                time.Time
	ConsumedAt               *time.Time
	ConsumedByIdempotencyKey *uuid.UUID
	ScheduleRequestHash      []byte
	OperationID              *uuid.UUID
}

func (confirmation CampaignConfirmation) Validate() error {
	if confirmation.ID == uuid.Nil ||
		confirmation.NonceHash == ([sha256.Size]byte{}) ||
		confirmation.ActorID == uuid.Nil || confirmation.SessionID == uuid.Nil ||
		confirmation.CampaignID == uuid.Nil || confirmation.CampaignRevisionID == uuid.Nil ||
		confirmation.RenderSourceID == uuid.Nil ||
		confirmation.RenderSourceKind != "notification_campaign_body" ||
		confirmation.NormalizedSourceChecksum == ([sha256.Size]byte{}) ||
		confirmation.RenderArtifactID == uuid.Nil ||
		confirmation.AudienceFingerprint == ([sha256.Size]byte{}) ||
		confirmation.ExpectedCampaignVersion < 1 ||
		confirmation.DryRunRecipientCount < 0 ||
		confirmation.DryRunEstimatedAt.IsZero() || confirmation.IssuedAt.IsZero() ||
		!confirmation.ExpiresAt.After(confirmation.IssuedAt) ||
		confirmation.DryRunEstimatedAt.After(confirmation.IssuedAt) ||
		(confirmation.ScheduledFor != nil &&
			confirmation.ScheduledFor.IsZero()) {
		return contract.ErrInvalidCampaign
	}
	consumed := confirmation.ConsumedAt != nil
	if consumed != (confirmation.ConsumedByIdempotencyKey != nil) ||
		consumed != (confirmation.OperationID != nil) ||
		consumed != (len(confirmation.ScheduleRequestHash) == sha256.Size) {
		return contract.ErrInvalidCampaign
	}
	return nil
}

type CampaignRecipient struct {
	ID                 uuid.UUID
	CampaignID         uuid.UUID
	UserID             uuid.UUID
	CampaignRevisionID uuid.UUID
	SourceEventID      uuid.UUID
	Status             string
	NotificationID     *uuid.UUID
	SkipCode           *string
	FailureCode        *string
	AttemptCount       uint32
	NextAttemptAt      *time.Time
	CreatedAt          time.Time
	LastAttemptAt      *time.Time
	ProcessedAt        *time.Time
}

func (recipient CampaignRecipient) Validate() error {
	if recipient.ID == uuid.Nil || recipient.CampaignID == uuid.Nil ||
		recipient.UserID == uuid.Nil || recipient.CampaignRevisionID == uuid.Nil ||
		recipient.SourceEventID == uuid.Nil ||
		!contract.ValidRecipientStatus(recipient.Status) ||
		recipient.CreatedAt.IsZero() {
		return contract.ErrInvalidCampaign
	}
	return nil
}

func canonicalUUIDs(values []uuid.UUID) bool {
	for index, value := range values {
		if value == uuid.Nil || (index > 0 && bytes.Compare(values[index-1][:], value[:]) >= 0) {
			return false
		}
	}
	return true
}

func cloneCampaignUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
