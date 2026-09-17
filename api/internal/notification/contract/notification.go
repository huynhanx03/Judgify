package contract

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type NotificationResponse struct {
	ID                 uuid.UUID
	Type               string
	Category           string
	ContentKind        string
	TitleKey           *string
	BodyKey            *string
	Params             json.RawMessage
	CampaignRevisionID *uuid.UUID
	CampaignTitle      *string
	ActionPath         *string
	Version            int64
	CreatedAt          time.Time
	ReadAt             *time.Time
	ArchivedAt         *time.Time
	ExpiresAt          *time.Time
}

type CampaignContentResponse struct {
	NotificationID uuid.UUID
	RevisionID     uuid.UUID
	Title          string
	SanitizedHTML  string
	ActionPath     *string
}

type ListRequest struct {
	Cursor          string
	Limit           int
	IncludeArchived bool
}

type ListResponse struct {
	Items        []NotificationResponse
	NextCursor   *string
	UnreadCount  int64
	InboxVersion int64
}

type UnreadCountResponse struct {
	Count   int64
	Version int64
}

type MutationResponse struct {
	Notification *NotificationResponse
	UnreadCount  int64
	InboxVersion int64
	Changed      bool
}

type PreferenceResponse struct {
	Category string
	InApp    bool
	Required bool
	Version  int64
}

type UpdatePreferenceRequest struct {
	Category        string
	InApp           bool
	ExpectedVersion int64
}
