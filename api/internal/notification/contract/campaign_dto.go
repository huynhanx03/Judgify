package contract

import (
	"time"

	"github.com/google/uuid"
)

type CampaignAudienceRequest struct {
	Kind    string      `json:"kind" validate:"required,oneof=explicit_users role all_active"`
	RoleID  *uuid.UUID  `json:"role_id,omitempty"`
	UserIDs []uuid.UUID `json:"user_ids,omitempty" validate:"max=5000,dive,required"`
}

type CreateCampaignRequest struct {
	Title        string                  `json:"title" validate:"required,max=160"`
	BodyMarkdown string                  `json:"body_markdown" validate:"required,max=65536"`
	ActionPath   *string                 `json:"action_path,omitempty" validate:"omitempty,max=512"`
	Audience     CampaignAudienceRequest `json:"audience" validate:"required"`
	Reason       string                  `json:"reason" validate:"required,max=1024"`
}

type ReviseCampaignRequest struct {
	ID              uuid.UUID               `uri:"id,parser=encoding.TextUnmarshaler" json:"-" validate:"required"`
	ExpectedVersion int64                   `json:"expected_version" validate:"required,min=1"`
	Title           string                  `json:"title" validate:"required,max=160"`
	BodyMarkdown    string                  `json:"body_markdown" validate:"required,max=65536"`
	ActionPath      *string                 `json:"action_path,omitempty" validate:"omitempty,max=512"`
	Audience        CampaignAudienceRequest `json:"audience" validate:"required"`
	Reason          string                  `json:"reason" validate:"required,max=1024"`
}

type CampaignIDRequest struct {
	ID uuid.UUID `uri:"id,parser=encoding.TextUnmarshaler" validate:"required"`
}

type CampaignListRequest struct {
	Status string `form:"status" json:"-" validate:"omitempty,oneof=draft scheduled sending completed failed cancelled"`
	Cursor string `form:"cursor" json:"-" validate:"omitempty,max=128"`
	Limit  int    `form:"limit" json:"-" validate:"omitempty,min=1,max=100"`
}

type DryRunCampaignRequest struct {
	ID              uuid.UUID  `uri:"id,parser=encoding.TextUnmarshaler" json:"-" validate:"required"`
	ExpectedVersion int64      `json:"expected_version" validate:"required,min=1"`
	ScheduledFor    *time.Time `json:"scheduled_for,omitempty"`
}

type ScheduleCampaignRequest struct {
	ID                uuid.UUID `uri:"id,parser=encoding.TextUnmarshaler" json:"-" validate:"required"`
	ConfirmationToken string    `json:"confirmation_token" validate:"required,max=128"`
	Reason            string    `json:"reason" validate:"required,max=1024"`
}

type CancelCampaignRequest struct {
	ID                       uuid.UUID `uri:"id,parser=encoding.TextUnmarshaler" json:"-" validate:"required"`
	ExpectedCampaignVersion  int64     `json:"expected_campaign_version" validate:"required,min=1"`
	ExpectedOperationVersion int64     `json:"expected_operation_version" validate:"required,min=1"`
	Reason                   string    `json:"reason" validate:"required,max=1024"`
}

type CampaignAudienceResponse struct {
	Kind          string      `json:"kind"`
	RoleID        *uuid.UUID  `json:"role_id,omitempty"`
	ExplicitUsers []uuid.UUID `json:"explicit_users,omitempty"`
}

type CampaignRevisionResponse struct {
	ID             uuid.UUID                `json:"id"`
	RevisionNumber int64                    `json:"revision_number"`
	Title          string                   `json:"title"`
	BodyMarkdown   string                   `json:"body_markdown"`
	SanitizedHTML  string                   `json:"sanitized_html"`
	ActionPath     *string                  `json:"action_path,omitempty"`
	Audience       CampaignAudienceResponse `json:"audience"`
	AuthorID       uuid.UUID                `json:"author_id"`
	CreatedAt      time.Time                `json:"created_at"`
}

type CampaignRecipientCountsResponse struct {
	Total   int64 `json:"total"`
	Pending int64 `json:"pending"`
	Created int64 `json:"created"`
	Skipped int64 `json:"skipped"`
	Failed  int64 `json:"failed"`
}

type CampaignOperationResponse struct {
	ID              uuid.UUID `json:"id"`
	Status          string    `json:"status"`
	Processed       int64     `json:"processed"`
	Succeeded       int64     `json:"succeeded"`
	Failed          int64     `json:"failed"`
	Total           int64     `json:"total"`
	Version         int64     `json:"version"`
	RequestedAction string    `json:"requested_action"`
	LastErrorCode   string    `json:"last_error_code,omitempty"`
}

type CampaignResponse struct {
	ID              uuid.UUID                       `json:"id"`
	Status          string                          `json:"status"`
	Version         int64                           `json:"version"`
	Revision        CampaignRevisionResponse        `json:"revision"`
	Recipients      CampaignRecipientCountsResponse `json:"recipients"`
	Operation       *CampaignOperationResponse      `json:"operation,omitempty"`
	ScheduledFor    *time.Time                      `json:"scheduled_for,omitempty"`
	LastFailureCode *string                         `json:"last_failure_code,omitempty"`
	CreatedBy       uuid.UUID                       `json:"created_by"`
	ScheduledBy     *uuid.UUID                      `json:"scheduled_by,omitempty"`
	CancelledBy     *uuid.UUID                      `json:"cancelled_by,omitempty"`
	CancelReason    *string                         `json:"cancel_reason,omitempty"`
	CreatedAt       time.Time                       `json:"created_at"`
	UpdatedAt       time.Time                       `json:"updated_at"`
	ScheduledAt     *time.Time                      `json:"scheduled_at,omitempty"`
	StartedAt       *time.Time                      `json:"started_at,omitempty"`
	CompletedAt     *time.Time                      `json:"completed_at,omitempty"`
	CancelledAt     *time.Time                      `json:"cancelled_at,omitempty"`
}

type CampaignListResponse struct {
	Items      []CampaignResponse `json:"items"`
	NextCursor *string            `json:"next_cursor,omitempty"`
}

type CampaignAudienceMemberResponse struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
}

type CampaignDryRunResponse struct {
	CampaignID            uuid.UUID                        `json:"campaign_id"`
	RevisionID            uuid.UUID                        `json:"revision_id"`
	CampaignVersion       int64                            `json:"campaign_version"`
	RecipientCount        int64                            `json:"recipient_count"`
	Sample                []CampaignAudienceMemberResponse `json:"sample"`
	EstimatedAt           time.Time                        `json:"estimated_at"`
	ConfirmationToken     string                           `json:"confirmation_token"`
	ConfirmationExpiresAt time.Time                        `json:"confirmation_expires_at"`
	ScheduledFor          *time.Time                       `json:"scheduled_for,omitempty"`
}

type CampaignScheduleResponse struct {
	Campaign  CampaignResponse          `json:"campaign"`
	Operation CampaignOperationResponse `json:"operation"`
	Replayed  bool                      `json:"replayed"`
}

type CampaignCancelResponse struct {
	Campaign  CampaignResponse          `json:"campaign"`
	Operation CampaignOperationResponse `json:"operation"`
	Replayed  bool                      `json:"replayed"`
}

type CampaignRecipientResponse struct {
	UserID         uuid.UUID  `json:"user_id"`
	Status         string     `json:"status"`
	NotificationID *uuid.UUID `json:"notification_id,omitempty"`
	SkipCode       *string    `json:"skip_code,omitempty"`
	FailureCode    *string    `json:"failure_code,omitempty"`
	AttemptCount   uint32     `json:"attempt_count"`
	LastAttemptAt  *time.Time `json:"last_attempt_at,omitempty"`
	ProcessedAt    *time.Time `json:"processed_at,omitempty"`
}

type CampaignRecipientListRequest struct {
	ID     uuid.UUID `uri:"id,parser=encoding.TextUnmarshaler" json:"-" validate:"required"`
	Status string    `form:"status" json:"-" validate:"omitempty,oneof=pending created skipped failed"`
	Cursor string    `form:"cursor" json:"-" validate:"omitempty,max=128"`
	Limit  int       `form:"limit" json:"-" validate:"omitempty,min=1,max=100"`
}

type CampaignRecipientListResponse struct {
	Items      []CampaignRecipientResponse `json:"items"`
	NextCursor *string                     `json:"next_cursor,omitempty"`
}
