package contract

import (
	"time"

	"github.com/google/uuid"
)

type RegistrationCommandResponse struct {
	ContestID           uuid.UUID
	UserID              uuid.UUID
	RegistrationID      uuid.UUID
	Status              string
	RegistrationVersion int64
	ContestVersion      int64
	OccurredAt          time.Time
	CommandID           uuid.UUID
	IdempotentReplay    bool
	CID                 string
}
