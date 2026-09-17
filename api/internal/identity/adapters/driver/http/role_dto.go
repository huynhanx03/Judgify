package http

import "time"

type createRoleRequest struct {
	CommandID       string `json:"command_id" validate:"required"`
	ExpectedVersion int64  `json:"expected_version" validate:"required,eq=1"`
	Reason          string `json:"reason" validate:"required,min=3,max=1024"`
	Key             string `json:"key" validate:"required,min=2,max=50,lowercase"`
	Name            string `json:"name" validate:"required,min=2,max=50"`
	Description     string `json:"description" validate:"omitempty,max=255"`
}

type roleCommandRequest struct {
	ID                string  `uri:"id" validate:"required"`
	CommandID         string  `json:"command_id" validate:"required"`
	ExpectedVersion   int64   `json:"expected_version" validate:"required,gt=0"`
	Reason            string  `json:"reason" validate:"required,min=3,max=1024"`
	Name              *string `json:"name" validate:"omitempty,min=2,max=50"`
	Description       *string `json:"description" validate:"omitempty,max=255"`
	ConfirmationToken string  `json:"confirmation_token,omitempty" validate:"omitempty,max=2048"`
}

type getRoleRequest struct {
	ID string `uri:"id" validate:"required"`
}

type roleResponse struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsSystem    bool   `json:"is_system"`
	IsProtected bool   `json:"is_protected"`
	Status      string `json:"status"`
	Version     int64  `json:"version"`
}

type roleMetadataPreviewResponse struct {
	Preview struct {
		Action           string       `json:"action"`
		RoleID           string       `json:"role_id"`
		ExpectedVersion  int64        `json:"expected_version"`
		ResultingVersion int64        `json:"resulting_version"`
		Before           roleResponse `json:"before"`
		After            roleResponse `json:"after"`
	} `json:"preview"`
	ConfirmationToken string    `json:"confirmation_token"`
	ExpiresAt         time.Time `json:"expires_at"`
}

type roleMutationReceiptResponse struct {
	CommandID        string       `json:"command_id"`
	EventID          string       `json:"event_id"`
	Action           string       `json:"action"`
	Role             roleResponse `json:"role"`
	CommittedAt      time.Time    `json:"committed_at"`
	IdempotentReplay bool         `json:"idempotent_replay"`
}
