package http

import "time"

type registrationCommandRequest struct {
	ExpectedRegistrationVersion int64 `json:"expected_registration_version" validate:"min=0"`
}
type registrationCommandResponse struct {
	ContestID           string    `json:"contest_id"`
	UserID              string    `json:"user_id"`
	RegistrationID      string    `json:"registration_id"`
	Status              string    `json:"status"`
	RegistrationVersion int64     `json:"registration_version"`
	ContestVersion      int64     `json:"contest_version"`
	OccurredAt          time.Time `json:"occurred_at"`
	CommandID           string    `json:"command_id"`
	IdempotentReplay    bool      `json:"idempotent_replay"`
	CID                 string    `json:"cid"`
}
