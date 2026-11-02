package domain

import (
	"time"

	"github.com/google/uuid"
)

// ContestRegistration represents a user registration for a contest.
type ContestRegistration struct {
	ID                      uuid.UUID
	ContestID               uuid.UUID
	UserID                  uuid.UUID
	Status                  RegistrationStatus
	Source                  RegistrationSource
	Version                 int64
	OfficialSubmissionCount int64
	RegisteredAt            *time.Time
	WithdrawnAt             *time.Time
	BannedAt                *time.Time
	Reason                  *string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func (registration ContestRegistration) CanTransitionTo(next RegistrationStatus) error {
	if !next.Valid() || !registration.Status.Valid() {
		return ErrContestRegistrationTransition
	}
	if registration.Status == RegistrationBanned {
		return ErrContestRegistrationBanned
	}
	switch {
	case registration.Status == RegistrationRegistered && next == RegistrationWithdrawn:
		return nil
	case registration.Status == RegistrationWithdrawn && next == RegistrationRegistered:
		return nil
	case next == RegistrationBanned:
		return nil
	default:
		return ErrContestRegistrationTransition
	}
}
