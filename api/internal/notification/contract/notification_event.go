package contract

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// InboxNotificationPayload is the serialized outbox/realtime projection.
// It is deliberately separate from the application response model.
type InboxNotificationPayload struct {
	ID                 uuid.UUID       `json:"id"`
	Type               string          `json:"type"`
	Category           string          `json:"category"`
	ContentKind        string          `json:"content_kind"`
	TitleKey           *string         `json:"title_key,omitempty"`
	BodyKey            *string         `json:"body_key,omitempty"`
	Params             json.RawMessage `json:"params,omitempty"`
	CampaignRevisionID *uuid.UUID      `json:"campaign_revision_id,omitempty"`
	CampaignTitle      *string         `json:"campaign_title,omitempty"`
	ActionPath         *string         `json:"action_path,omitempty"`
	Version            int64           `json:"version"`
	CreatedAt          time.Time       `json:"created_at"`
	ReadAt             *time.Time      `json:"read_at,omitempty"`
	ArchivedAt         *time.Time      `json:"archived_at,omitempty"`
	ExpiresAt          *time.Time      `json:"expires_at,omitempty"`
}

type InboxEventPayload struct {
	UserID       uuid.UUID                 `json:"user_id"`
	Notification *InboxNotificationPayload `json:"notification,omitempty"`
	UnreadCount  int64                     `json:"unread_count"`
	InboxVersion int64                     `json:"inbox_version"`
}
