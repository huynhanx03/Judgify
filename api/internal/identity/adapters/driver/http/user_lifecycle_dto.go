package http

import "time"

type userLifecycleCommandRequest struct {
	ID                string `uri:"id" validate:"required"`
	CommandID         string `json:"command_id" validate:"required"`
	ExpectedVersion   int64  `json:"expected_version" validate:"required,gt=0"`
	Reason            string `json:"reason" validate:"required,min=3,max=1024"`
	ConfirmationToken string `json:"confirmation_token,omitempty" validate:"omitempty,max=2048"`
}

type userLifecycleProjectionResponse struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Status      string     `json:"status"`
	Version     int64      `json:"version"`
	SuspendedAt *time.Time `json:"suspended_at,omitempty"`
}

type userLifecyclePreviewResponse struct {
	Preview struct {
		Action           string                          `json:"action"`
		UserID           string                          `json:"user_id"`
		ExpectedVersion  int64                           `json:"expected_version"`
		ResultingVersion int64                           `json:"resulting_version"`
		Before           userLifecycleProjectionResponse `json:"before"`
		After            userLifecycleProjectionResponse `json:"after"`
	} `json:"preview"`
	ConfirmationToken string    `json:"confirmation_token"`
	ExpiresAt         time.Time `json:"expires_at"`
}

type userLifecycleReceiptResponse struct {
	CommandID        string                          `json:"command_id"`
	EventID          string                          `json:"event_id"`
	Action           string                          `json:"action"`
	User             userLifecycleProjectionResponse `json:"user"`
	CommittedAt      time.Time                       `json:"committed_at"`
	IdempotentReplay bool                            `json:"idempotent_replay"`
}
