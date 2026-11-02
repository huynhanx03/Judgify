package http

import (
	"encoding/json"
	"time"
)

type notificationIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

type notificationListRequest struct {
	Cursor          string `form:"cursor" validate:"omitempty,max=128"`
	Limit           int    `form:"limit" validate:"omitempty,min=1,max=100"`
	IncludeArchived bool   `form:"include_archived"`
}

type updateNotificationPreferenceRequest struct {
	Category        string `uri:"category" validate:"required,max=64"`
	InApp           bool   `json:"in_app"`
	ExpectedVersion int64  `json:"expected_version" validate:"min=0"`
}

type unreadCountResponse struct {
	Count   int64 `json:"count"`
	Version int64 `json:"version"`
}
type preferenceResponse struct {
	Category string `json:"category"`
	InApp    bool   `json:"in_app"`
	Required bool   `json:"required"`
	Version  int64  `json:"version"`
}
type campaignContentResponse struct {
	NotificationID string  `json:"notification_id"`
	RevisionID     string  `json:"revision_id"`
	Title          string  `json:"title"`
	SanitizedHTML  string  `json:"sanitized_html"`
	ActionPath     *string `json:"action_path,omitempty"`
}
type notificationResponse struct {
	ID                 string          `json:"id"`
	Type               string          `json:"type"`
	Category           string          `json:"category"`
	ContentKind        string          `json:"content_kind"`
	TitleKey           *string         `json:"title_key,omitempty"`
	BodyKey            *string         `json:"body_key,omitempty"`
	Params             json.RawMessage `json:"params,omitempty"`
	CampaignRevisionID *string         `json:"campaign_revision_id,omitempty"`
	CampaignTitle      *string         `json:"campaign_title,omitempty"`
	ActionPath         *string         `json:"action_path,omitempty"`
	Version            int64           `json:"version"`
	CreatedAt          time.Time       `json:"created_at"`
	ReadAt             *time.Time      `json:"read_at,omitempty"`
	ArchivedAt         *time.Time      `json:"archived_at,omitempty"`
	ExpiresAt          *time.Time      `json:"expires_at,omitempty"`
}
type notificationListResponse struct {
	Items        []*notificationResponse `json:"items"`
	NextCursor   *string                 `json:"next_cursor,omitempty"`
	UnreadCount  int64                   `json:"unread_count"`
	InboxVersion int64                   `json:"inbox_version"`
}
type notificationMutationResponse struct {
	Notification *notificationResponse `json:"notification,omitempty"`
	UnreadCount  int64                 `json:"unread_count"`
	InboxVersion int64                 `json:"inbox_version"`
	Changed      bool                  `json:"changed"`
}
