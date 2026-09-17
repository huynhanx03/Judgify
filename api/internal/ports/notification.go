// Package ports defines Judgify's stable internal capability catalog.
package ports

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	entity "github.com/huynhanx03/judgify/internal/notification/domain"
	"github.com/huynhanx03/judgify/internal/platform/operation"
)

// Notification capabilities are added here during the Notification migration.

var (
	ErrCampaignNotFound                 = errors.New("notification campaign: not found")
	ErrCampaignConflict                 = errors.New("notification campaign: version conflict")
	ErrCampaignNotDraft                 = errors.New("notification campaign: draft required")
	ErrCampaignNotCancellable           = errors.New("notification campaign: cancellable delivery required")
	ErrCampaignConfirmationInvalid      = errors.New("notification campaign: invalid confirmation")
	ErrCampaignConfirmationExpired      = errors.New("notification campaign: confirmation expired")
	ErrCampaignAudienceChanged          = errors.New("notification campaign: audience changed")
	ErrCampaignAudienceTooLarge         = errors.New("notification campaign: audience too large")
	ErrCampaignReauthenticationRequired = errors.New("notification campaign: recent reauthentication required")
	ErrCampaignPermissionDenied         = errors.New("notification campaign: permission denied")
)

type CampaignCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type CampaignListQuery struct {
	Status string
	Cursor *CampaignCursor
	Limit  int
}

type RecipientCursor struct {
	UserID uuid.UUID
}

type RecipientListQuery struct {
	CampaignID uuid.UUID
	Status     string
	Cursor     *RecipientCursor
	Limit      int
}

type CampaignPage struct {
	Items []entity.CampaignView
	Next  *CampaignCursor
}

type RecipientPage struct {
	Items []entity.CampaignRecipient
	Next  *RecipientCursor
}

type ScheduleFreeze struct {
	CampaignID      uuid.UUID
	Revision        entity.CampaignRevision
	ExpectedVersion int64
	OperationID     uuid.UUID
	ScheduledFor    *time.Time
	ActorID         uuid.UUID
	Reason          string
	At              time.Time
}

type CampaignCancellation struct {
	CampaignID      uuid.UUID
	OperationID     uuid.UUID
	ExpectedVersion int64
	ActorID         uuid.UUID
	Reason          string
	At              time.Time
}

type ConfirmationConsumption struct {
	NonceHash      [sha256.Size]byte
	IdempotencyKey uuid.UUID
	RequestHash    [sha256.Size]byte
	OperationID    uuid.UUID
	ConsumedAt     time.Time
}

type RecipientResult struct {
	UserID         uuid.UUID
	Status         string
	NotificationID *uuid.UUID
	SkipCode       string
	FailureCode    string
	AttemptedAt    time.Time
}

type CampaignRepository interface {
	Create(context.Context, entity.Campaign, entity.CampaignRevision) (entity.CampaignView, error)
	Lock(context.Context, uuid.UUID) (entity.CampaignView, error)
	Get(context.Context, uuid.UUID) (entity.CampaignView, error)
	List(context.Context, CampaignListQuery) (CampaignPage, error)
	AddRevision(context.Context, entity.CampaignRevision, int64, time.Time) (entity.CampaignView, error)

	StoreConfirmation(context.Context, entity.CampaignConfirmation) error
	LockConfirmation(context.Context, [sha256.Size]byte) (entity.CampaignConfirmation, error)
	FreezeSchedule(context.Context, ScheduleFreeze, []entity.AudienceMember) (entity.CampaignView, error)
	ConsumeConfirmation(context.Context, ConfirmationConsumption) error
	MarkCancelled(context.Context, CampaignCancellation) (entity.CampaignView, bool, error)

	LockDeliveryBatch(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		int,
		time.Time,
	) (entity.CampaignView, []entity.CampaignRecipient, error)
	ApplyRecipientResult(context.Context, uuid.UUID, RecipientResult) error
	MarkSending(context.Context, uuid.UUID, uuid.UUID, time.Time) (entity.CampaignView, bool, error)
	MarkTerminal(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		string,
		string,
		time.Time,
	) (entity.CampaignView, error)
	ListRecipients(context.Context, RecipientListQuery) (RecipientPage, error)
}

type CampaignService interface {
	Create(context.Context, uuid.UUID, *contract.CreateCampaignRequest) (*contract.CampaignResponse, error)
	Revise(context.Context, uuid.UUID, *contract.ReviseCampaignRequest) (*contract.CampaignResponse, error)
	Get(context.Context, uuid.UUID) (*contract.CampaignResponse, error)
	List(context.Context, *contract.CampaignListRequest) (*contract.CampaignListResponse, error)
	DryRun(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		*contract.DryRunCampaignRequest,
	) (*contract.CampaignDryRunResponse, error)
	ListRecipients(
		context.Context,
		*contract.CampaignRecipientListRequest,
	) (*contract.CampaignRecipientListResponse, error)
}

type CampaignScheduler interface {
	Schedule(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		uuid.UUID,
		*contract.ScheduleCampaignRequest,
	) (*contract.CampaignScheduleResponse, error)
}

type CampaignCanceller interface {
	Cancel(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		uuid.UUID,
		*contract.CancelCampaignRequest,
	) (*contract.CampaignCancelResponse, error)
}

// CampaignCapabilityChecker keeps audience-sensitive authorization in the
// application boundary. Route guards enforce compose/read, while role-wide
// and all-active broadcasts are re-authorized against the exact frozen
// audience immediately before dry-run and schedule.
type CampaignCapabilityChecker interface {
	RequireAudienceCapability(context.Context, string) error
}

type OperationReader interface {
	Get(context.Context, uuid.UUID) (operation.Operation, error)
}

var (
	ErrInvalidRequest            = errors.New("notification: invalid request")
	ErrInvalidCursor             = errors.New("notification: invalid cursor")
	ErrNotFound                  = errors.New("notification: not found")
	ErrPreferenceVersionConflict = errors.New("notification: preference version conflict")
)

type CreateCommand struct {
	UserID             uuid.UUID
	SourceEventID      uuid.UUID
	Type               string
	Category           string
	TitleKey           *string
	BodyKey            *string
	Params             json.RawMessage
	CampaignRevisionID *uuid.UUID
	ActionPath         *string
	OccurredAt         time.Time
	ExpiresAt          *time.Time
}

type ListQuery struct {
	UserID          uuid.UUID
	Cursor          string
	Limit           int
	IncludeArchived bool
	Now             time.Time
}

type MutationResult struct {
	Notification *entity.Notification
	Inbox        entity.InboxState
	Changed      bool
}

type Repository interface {
	Create(context.Context, CreateCommand) (*entity.Notification, entity.InboxState, bool, error)
	List(context.Context, ListQuery) ([]entity.Notification, *string, entity.InboxState, error)
	CampaignContent(context.Context, uuid.UUID, uuid.UUID, time.Time) (entity.CampaignContent, error)
	Unread(context.Context, uuid.UUID, time.Time) (entity.InboxState, error)
	MarkRead(context.Context, uuid.UUID, uuid.UUID, time.Time) (MutationResult, error)
	MarkAllRead(context.Context, uuid.UUID, time.Time) (MutationResult, error)
	Archive(context.Context, uuid.UUID, uuid.UUID, time.Time) (MutationResult, error)
	Preferences(context.Context, uuid.UUID) ([]entity.Preference, error)
	UpdatePreference(context.Context, uuid.UUID, string, bool, int64, time.Time) (entity.Preference, error)
}

type Service interface {
	CreateFromEvent(context.Context, CreateCommand) (*contract.NotificationResponse, bool, error)
	List(context.Context, uuid.UUID, *contract.ListRequest) (*contract.ListResponse, error)
	CampaignContent(context.Context, uuid.UUID, uuid.UUID) (*contract.CampaignContentResponse, error)
	Unread(context.Context, uuid.UUID) (*contract.UnreadCountResponse, error)
	MarkRead(context.Context, uuid.UUID, uuid.UUID) (*contract.MutationResponse, error)
	MarkAllRead(context.Context, uuid.UUID) (*contract.MutationResponse, error)
	Archive(context.Context, uuid.UUID, uuid.UUID) (*contract.MutationResponse, error)
	Preferences(context.Context, uuid.UUID) ([]contract.PreferenceResponse, error)
	UpdatePreference(context.Context, uuid.UUID, *contract.UpdatePreferenceRequest) (*contract.PreferenceResponse, error)
}
